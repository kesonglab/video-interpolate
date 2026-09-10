package encode

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type DecodeOpts struct {
	Input      string
	OutputDir  string
	HWDecoder  string // "videotoolbox" or ""
	Format     string // "jpg" or "png"
	JPGQuality int    // 1-31, lower is better
	StartFrame int    // chunk start, inclusive (0 if full)
	EndFrame   int    // chunk end, exclusive (0 if full)
	FPS        float64
	ProgressFn func(done int) // called ~every 500ms with frames written so far
}

// DecodeCommand returns the ffmpeg args DecodeToFrames runs.
func DecodeCommand(opts DecodeOpts) []string {
	if opts.Format == "" {
		opts.Format = "jpg"
	}
	args := []string{"-y", "-v", "error"}
	if opts.HWDecoder != "" {
		args = append(args, "-hwaccel", opts.HWDecoder)
	}
	args = append(args, "-i", opts.Input)
	if opts.StartFrame > 0 && opts.FPS > 0 {
		args = append(args, "-ss", strconv.FormatFloat(float64(opts.StartFrame)/opts.FPS, 'f', 3, 64))
	}
	if opts.EndFrame > opts.StartFrame {
		args = append(args, "-frames:v", strconv.Itoa(opts.EndFrame-opts.StartFrame))
	}
	if opts.Format == "jpg" {
		q := opts.JPGQuality
		if q < 1 {
			q = 2
		}
		args = append(args, "-qscale:v", strconv.Itoa(q))
	}
	args = append(args, filepath.Join(opts.OutputDir, "frame_%08d."+opts.Format))
	return args
}

// DecodeToFrames extracts frames from Input to OutputDir.
// Returns the count of frames written.
func DecodeToFrames(ctx context.Context, opts DecodeOpts) (int, error) {
	if opts.OutputDir == "" {
		return 0, errors.New("decode: OutputDir is required")
	}
	if opts.Format == "" {
		opts.Format = "jpg"
	}
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return 0, fmt.Errorf("decode: mkdir: %w", err)
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", DecodeCommand(opts)...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	stop := make(chan struct{})
	var wg sync.WaitGroup
	if opts.ProgressFn != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tick := time.NewTicker(500 * time.Millisecond)
			defer tick.Stop()
			for {
				select {
				case <-stop:
					return
				case <-tick.C:
					opts.ProgressFn(countFrames(opts.OutputDir, opts.Format))
				}
			}
		}()
	}

	err := cmd.Run()
	close(stop)
	wg.Wait()
	if err != nil {
		return 0, fmt.Errorf("ffmpeg decode: %v: %s", err, tail(stderr.String(), 200))
	}
	return countFrames(opts.OutputDir, opts.Format), nil
}

func countFrames(dir, format string) int {
	matches, _ := filepath.Glob(filepath.Join(dir, "frame_*."+format))
	return len(matches)
}

func tail(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return "..." + s[len(s)-n:]
	}
	return s
}
