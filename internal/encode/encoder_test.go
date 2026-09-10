package encode

import (
	"os/exec"
	"path/filepath"
	"testing"
)

func TestEncode_ChunkNaming(t *testing.T) {
	dir := t.TempDir()
	opts := EncodeOpts{InputDir: "/frames", InputFPS: 60, OutputDir: dir, ChunkIndex: 0, Format: "jpg"}
	want := filepath.Join(dir, "chunk_000.mp4")
	if got := OutputPathFor(opts); got != want {
		t.Errorf("OutputPathFor = %q, want %q", got, want)
	}
	args := EncodeCommand(opts)
	if last := args[len(args)-1]; last != want {
		t.Errorf("encode output arg = %q, want %q", last, want)
	}
}

func TestEncode_NoChunk(t *testing.T) {
	out := filepath.Join(t.TempDir(), "final.mp4")
	opts := EncodeOpts{InputDir: "/frames", InputFPS: 60, OutputPath: out, ChunkIndex: NoChunk, Format: "jpg"}
	if got := OutputPathFor(opts); got != out {
		t.Errorf("OutputPathFor = %q, want %q", got, out)
	}
	args := EncodeCommand(opts)
	if last := args[len(args)-1]; last != out {
		t.Errorf("encode output arg = %q, want %q", last, out)
	}
}

func TestQualityFor(t *testing.T) {
	cases := []struct {
		enc, preset string
		want        int
	}{
		{"hevc_videotoolbox", "quality", 50},
		{"hevc_videotoolbox", "balanced", 65},
		{"hevc_videotoolbox", "speed", 75},
		{"libx264", "quality", 18},
		{"libx264", "balanced", 20},
		{"libx264", "speed", 23},
	}
	for _, c := range cases {
		if got := QualityFor(c.enc, c.preset); got != c.want {
			t.Errorf("QualityFor(%q, %q) = %d, want %d", c.enc, c.preset, got, c.want)
		}
	}
}

func TestMuxAudio_NeedsFFmpeg(t *testing.T) {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		t.Skip("ffmpeg not installed")
	}
	// MuxAudio is exercised end-to-end by the smoke test; just check the
	// helper handles a missing input file cleanly.
	if err := MuxAudio(t.Context(), "/nonexistent.mp4", "/nonexistent.m4a"); err == nil {
		t.Error("MuxAudio on missing files returned nil, want error")
	}
}
