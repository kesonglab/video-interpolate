package encode

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type ConcatOpts struct {
	ChunkDir   string // dir holding chunk_000.mp4, chunk_001.mp4, ...
	OutputPath string
	ChunkCount int // expected number of chunks, validated before running
}

// ConcatChunks joins the chunks with ffmpeg's concat demuxer, stream copy only.
func ConcatChunks(ctx context.Context, opts ConcatOpts) error {
	if opts.ChunkDir == "" {
		return fmt.Errorf("concat: ChunkDir is required")
	}
	if opts.ChunkCount < 1 {
		return fmt.Errorf("concat: ChunkCount must be >= 1")
	}
	if err := os.MkdirAll(filepath.Dir(opts.OutputPath), 0o755); err != nil {
		return fmt.Errorf("concat: mkdir: %w", err)
	}

	var list strings.Builder
	for i := 0; i < opts.ChunkCount; i++ {
		name := fmt.Sprintf("chunk_%03d.mp4", i)
		fi, err := os.Stat(filepath.Join(opts.ChunkDir, name))
		if err != nil || fi.Size() == 0 {
			return fmt.Errorf("concat: missing chunk %s", name)
		}
		fmt.Fprintf(&list, "file '%s'\n", name)
	}
	listPath := filepath.Join(opts.ChunkDir, "list.txt")
	if err := os.WriteFile(listPath, []byte(list.String()), 0o644); err != nil {
		return fmt.Errorf("concat: write list: %w", err)
	}

	cmd := exec.CommandContext(ctx, "ffmpeg", "-y", "-v", "error",
		"-f", "concat", "-safe", "0", "-i", listPath,
		"-c", "copy", opts.OutputPath)
	// list entries are relative, resolve them against the chunk dir
	cmd.Dir = opts.ChunkDir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("ffmpeg concat: %v: %s", err, strings.TrimSpace(stderr.String()))
	}
	return nil
}
