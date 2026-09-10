package probe

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestProbeSample(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not available")
	}
	sample := filepath.Join("..", "..", "testdata", "sample.mp4")
	if err := os.MkdirAll(filepath.Dir(sample), 0o755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("ffmpeg", "-y", "-f", "lavfi",
		"-i", "testsrc=duration=1:size=320x240:rate=30",
		"-pix_fmt", "yuv420p", "-c:v", "libx264", sample)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Skipf("cannot generate sample video: %v: %s", err, out)
	}

	info, err := Probe(context.Background(), sample)
	if err != nil {
		t.Fatal(err)
	}
	if info.Width != 320 {
		t.Errorf("Width = %d, want 320", info.Width)
	}
	if info.Height != 240 {
		t.Errorf("Height = %d, want 240", info.Height)
	}
	if info.FPS != 30 {
		t.Errorf("FPS = %v, want 30", info.FPS)
	}
	if info.Codec != "h264" {
		t.Errorf("Codec = %q, want h264", info.Codec)
	}
	if info.FrameCount != 30 {
		t.Errorf("FrameCount = %d, want 30", info.FrameCount)
	}
	if info.Duration < 900*time.Millisecond || info.Duration > 1100*time.Millisecond {
		t.Errorf("Duration = %v, want ~1s", info.Duration)
	}
}
