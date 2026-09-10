package interpolate

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/kesonglab/video-interpolate/internal/system"
)

type RifeOpts struct {
	InputDir   string
	OutputDir  string
	GPU        int    // -1 = CPU
	Threads    string // "4:4:4"
	ModelPath  string // absolute path, "" = auto-detect
	Format     string // "jpg" or "png"
	Multiplier int    // 2 / 4 / 8
	Bin        string // explicit binary path, "" = auto-detect
	ProgressFn func(done, total int)
}

// RifeCommand returns the rife-ncnn-vulkan args.
// 2x needs no -e (default), 4x/8x use -e (Multiplier-1).
func RifeCommand(opts RifeOpts) []string {
	if opts.Format == "" {
		opts.Format = "jpg"
	}
	threads := opts.Threads
	if threads == "" {
		threads = "4:4:4"
	}
	args := []string{"rife-ncnn-vulkan",
		"-i", opts.InputDir,
		"-o", opts.OutputDir,
		"-g", strconv.Itoa(opts.GPU),
		"-j", threads,
		"-f", "frame_%08d." + opts.Format,
	}
	if opts.ModelPath != "" {
		args = append(args, "-m", opts.ModelPath)
	}
	if opts.Multiplier > 2 {
		args = append(args, "-e", strconv.Itoa(opts.Multiplier-1))
	}
	return args
}

// MultForFrames is how many frames the pipeline should expect after rife.
func MultForFrames(inputFrames, multiplier int) int {
	if multiplier < 2 {
		multiplier = 2
	}
	return inputFrames * multiplier
}

// Interpolate runs rife-ncnn-vulkan to produce interpolated frames.
func Interpolate(ctx context.Context, opts RifeOpts) error {
	if opts.OutputDir == "" {
		return fmt.Errorf("rife: OutputDir is required")
	}
	if opts.Format == "" {
		opts.Format = "jpg"
	}
	if opts.Threads == "" {
		opts.Threads = "4:4:4"
	}
	if err := os.MkdirAll(opts.OutputDir, 0o755); err != nil {
		return fmt.Errorf("rife: mkdir: %w", err)
	}

	bin := opts.Bin
	if bin == "" {
		var err error
		bin, err = system.FindRifeBinary()
		if err != nil {
			return fmt.Errorf("rife-ncnn-vulkan not found.\n" +
				"  install via brew: brew install rife-ncnn-vulkan\n" +
				"  or download: https://github.com/nihui/rife-ncnn-vulkan")
		}
	}
	if opts.ModelPath == "" {
		opts.ModelPath = DetectModel(filepath.Dir(bin))
	}

	args := RifeCommand(opts)
	args[0] = bin
	cmd := exec.CommandContext(ctx, bin, args[1:]...)
	cmd.Stdout = io.Discard
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	total := MultForFrames(countFrames(opts.InputDir, opts.Format), opts.Multiplier)
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
					opts.ProgressFn(countFrames(opts.OutputDir, opts.Format), total)
				}
			}
		}()
	}

	err := cmd.Run()
	close(stop)
	wg.Wait()
	if err != nil {
		return fmt.Errorf("rife failed: %v\ncommand: %s\n%s", err, strings.Join(args, " "), tail(stderr.String(), 500))
	}
	return nil
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
