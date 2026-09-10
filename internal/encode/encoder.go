package encode

import (
	"bufio"
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
)

// NoChunk marks a final (non-chunked) encode.
const NoChunk = -1

type EncodeOpts struct {
	InputDir   string
	InputFPS   float64         // 插帧后的帧率
	OutputPath string          // final output when ChunkIndex < 0
	OutputDir  string          // chunk output dir when ChunkIndex >= 0
	ChunkIndex int             // NoChunk = final output, >= 0 = chunk_%03d.mp4 under OutputDir
	Encoder    string          // "hevc_videotoolbox" / "h264_videotoolbox" / "libx264" / "libx265"
	Quality    int             // q:v for VT (0-100), crf for software
	PixelFmt   string          // "yuv420p" / "p010le"
	AudioPath  string          // optional
	AudioMode  string          // "copy" / "reencode" / "drop"
	Format     string          // intermediate frame format
	ProgressFn func(frame int) // called periodically
}

// OutputPathFor resolves where an encode writes: chunk_%03d.mp4 in OutputDir
// for chunks, otherwise OutputPath.
func OutputPathFor(opts EncodeOpts) string {
	if opts.ChunkIndex >= 0 {
		return filepath.Join(opts.OutputDir, fmt.Sprintf("chunk_%03d.mp4", opts.ChunkIndex))
	}
	return opts.OutputPath
}

// EncodeCommand returns the ffmpeg args EncodeFromFrames runs.
func EncodeCommand(opts EncodeOpts) []string {
	if opts.Format == "" {
		opts.Format = "jpg"
	}
	q := opts.Quality
	if q <= 0 {
		q = 23
	}
	args := []string{"-y", "-v", "error", "-progress", "pipe:1", "-nostats"}
	args = append(args, "-framerate", strconv.FormatFloat(opts.InputFPS, 'f', 3, 64))
	args = append(args, "-i", filepath.Join(opts.InputDir, "frame_%08d."+opts.Format))
	if opts.AudioMode != "drop" && opts.AudioPath != "" {
		args = append(args, "-i", opts.AudioPath)
	}
	if opts.AudioMode != "drop" && opts.AudioPath != "" {
		if opts.AudioMode == "reencode" {
			args = append(args, "-c:a", "aac", "-b:a", "192k")
		} else {
			args = append(args, "-c:a", "copy")
		}
	}
	args = append(args, "-r", strconv.FormatFloat(opts.InputFPS, 'f', 3, 64))
	switch opts.Encoder {
	case "hevc_videotoolbox":
		args = append(args, "-c:v", "hevc_videotoolbox", "-q:v", strconv.Itoa(q), "-tag:v", "hvc1", "-pix_fmt", "yuv420p")
	case "h264_videotoolbox":
		args = append(args, "-c:v", "h264_videotoolbox", "-q:v", strconv.Itoa(q), "-pix_fmt", "yuv420p")
	case "libx265":
		pf := opts.PixelFmt
		if pf == "" {
			pf = "yuv420p"
		}
		args = append(args, "-c:v", "libx265", "-crf", strconv.Itoa(q), "-preset", "medium", "-tag:v", "hvc1", "-pix_fmt", pf)
	default: // libx264
		args = append(args, "-c:v", "libx264", "-crf", strconv.Itoa(q), "-preset", "medium", "-pix_fmt", "yuv420p")
	}
	args = append(args, "-movflags", "+faststart", OutputPathFor(opts))
	return args
}

// EncodeFromFrames assembles frames from InputDir into OutputPath.
func EncodeFromFrames(ctx context.Context, opts EncodeOpts) error {
	if out := OutputPathFor(opts); out != "" {
		// stale file, ffmpeg -y overwrites anyway
		_ = os.Remove(out)
	}
	cmd := exec.CommandContext(ctx, "ffmpeg", EncodeCommand(opts)...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		return err
	}

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		scanProgress(stdout, opts.ProgressFn, stop)
	}()
	err = cmd.Wait()
	close(stop)
	wg.Wait()
	if err != nil {
		return fmt.Errorf("ffmpeg encode: %v: %s", err, tail(stderr.String(), 200))
	}
	return nil
}

// scanProgress reads ffmpeg's `-progress pipe:1` output and reports frame counts.
func scanProgress(r io.Reader, fn func(int), stop <-chan struct{}) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 64*1024), 64*1024)
	last := -1
	var lastEmit time.Time
	for sc.Scan() {
		line := sc.Text()
		if v, ok := parseFrameLine(line); ok {
			last = v
			if fn != nil && time.Since(lastEmit) >= 200*time.Millisecond {
				fn(v)
				lastEmit = time.Now()
			}
		}
		select {
		case <-stop:
			return
		default:
		}
	}
	if fn != nil && last >= 0 {
		fn(last)
	}
}

func parseFrameLine(line string) (int, bool) {
	if !strings.HasPrefix(line, "frame=") {
		return 0, false
	}
	v, err := strconv.Atoi(strings.TrimSpace(strings.TrimPrefix(line, "frame=")))
	if err != nil {
		return 0, false
	}
	return v, true
}
