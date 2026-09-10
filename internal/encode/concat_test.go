package encode

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
)

func TestConcat(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	chunkDir := t.TempDir()
	for i := 0; i < 2; i++ {
		out := filepath.Join(chunkDir, fmt.Sprintf("chunk_%03d.mp4", i))
		cmd := exec.Command("ffmpeg", "-y", "-v", "error",
			"-f", "lavfi", "-i", "testsrc=duration=1:size=160x120:rate=30",
			"-pix_fmt", "yuv420p", out)
		if b, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("make chunk %d: %v: %s", i, err, b)
		}
	}

	out := filepath.Join(t.TempDir(), "final.mp4")
	if err := ConcatChunks(context.Background(), ConcatOpts{
		ChunkDir:   chunkDir,
		OutputPath: out,
		ChunkCount: 2,
	}); err != nil {
		t.Fatalf("ConcatChunks: %v", err)
	}

	// 2 x 1s chunks should probe to ~2s
	if d := probeDuration(t, out); d < 1.9 || d > 2.1 {
		t.Errorf("concat duration = %.3fs, want ~2s", d)
	}
}

func TestConcat_MissingChunk(t *testing.T) {
	err := ConcatChunks(context.Background(), ConcatOpts{
		ChunkDir:   t.TempDir(),
		OutputPath: filepath.Join(t.TempDir(), "final.mp4"),
		ChunkCount: 2,
	})
	if err == nil {
		t.Fatal("ConcatChunks with no chunks returned nil, want error")
	}
}

func probeDuration(t *testing.T, path string) float64 {
	t.Helper()
	out, err := exec.Command("ffprobe", "-v", "error", "-print_format", "json",
		"-show_entries", "format=duration", path).Output()
	if err != nil {
		t.Fatalf("ffprobe: %v", err)
	}
	var parsed struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if err := json.Unmarshal(out, &parsed); err != nil {
		t.Fatalf("parse ffprobe: %v", err)
	}
	d, _ := strconv.ParseFloat(parsed.Format.Duration, 64)
	return d
}
