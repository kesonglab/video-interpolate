package config

import (
	"path/filepath"
	"testing"
)

func TestDefault(t *testing.T) {
	d := Default()
	if d.OutputDir == "" {
		t.Error("OutputDir not set")
	}
	if d.Multiplier != 2 {
		t.Errorf("Multiplier = %d, want 2", d.Multiplier)
	}
	if d.TargetFPS != 0 {
		t.Errorf("TargetFPS = %v, want 0", d.TargetFPS)
	}
	if d.Encoder != "auto" {
		t.Errorf("Encoder = %q, want auto", d.Encoder)
	}
	if d.Quality != "balanced" {
		t.Errorf("Quality = %q, want balanced", d.Quality)
	}
	if d.Threads != "4:4:4" {
		t.Errorf("Threads = %q, want 4:4:4", d.Threads)
	}
	if d.Format != "jpg" {
		t.Errorf("Format = %q, want jpg", d.Format)
	}
	if d.JPGQuality != 2 {
		t.Errorf("JPGQuality = %d, want 2", d.JPGQuality)
	}
	if d.HWDecode != "auto" {
		t.Errorf("HWDecode = %q, want auto", d.HWDecode)
	}
	if d.Jobs != 1 {
		t.Errorf("Jobs = %d, want 1", d.Jobs)
	}
	if !d.SkipExisting {
		t.Error("SkipExisting = false, want true")
	}
	if d.Audio != "copy" {
		t.Errorf("Audio = %q, want copy", d.Audio)
	}
	if d.ChunkSize != 3000 {
		t.Errorf("ChunkSize = %d, want 3000", d.ChunkSize)
	}
}

func TestValidate(t *testing.T) {
	valid := Default()
	if err := valid.Validate(); err != nil {
		t.Fatalf("default config should validate: %v", err)
	}

	cases := []struct {
		name string
		bad  *Config
	}{
		{"multiplier", func() *Config { c := Default(); c.Multiplier = 3; return c }()},
		{"encoder", func() *Config { c := Default(); c.Encoder = "vp9"; return c }()},
		{"quality", func() *Config { c := Default(); c.Quality = "ultra"; return c }()},
		{"format", func() *Config { c := Default(); c.Format = "webp"; return c }()},
		{"audio", func() *Config { c := Default(); c.Audio = "merge"; return c }()},
		{"jpg_quality_low", func() *Config { c := Default(); c.JPGQuality = 0; return c }()},
		{"jpg_quality_high", func() *Config { c := Default(); c.JPGQuality = 32; return c }()},
		{"jobs", func() *Config { c := Default(); c.Jobs = 0; return c }()},
		{"chunk_size", func() *Config { c := Default(); c.ChunkSize = -1; return c }()},
		{"target_fps", func() *Config { c := Default(); c.TargetFPS = -1; return c }()},
	}
	for _, c := range cases {
		if err := c.bad.Validate(); err == nil {
			t.Errorf("%s: expected error, got nil", c.name)
		}
	}
}

func TestMerge(t *testing.T) {
	c := Default()
	c.Merge(map[string]any{
		"multiplier":  4,
		"encoder":     "libx264",
		"GPU":         -1,
		"jpg_quality": 5,
	})
	if c.Multiplier != 4 {
		t.Errorf("Multiplier = %d, want 4", c.Multiplier)
	}
	if c.Encoder != "libx264" {
		t.Errorf("Encoder = %q, want libx264", c.Encoder)
	}
	if c.GPU != -1 {
		t.Errorf("GPU = %d, want -1", c.GPU)
	}
	if c.JPGQuality != 5 {
		t.Errorf("JPGQuality = %d, want 5", c.JPGQuality)
	}
	// untouched fields stay at default
	if c.Format != "jpg" {
		t.Errorf("Format = %q, want jpg", c.Format)
	}
}

func TestLoadMissing(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "does-not-exist.toml"))
	if err != nil {
		t.Fatalf("Load(missing) = %v, want nil", err)
	}
	d := Default()
	if c.OutputDir != d.OutputDir || c.Multiplier != d.Multiplier {
		t.Errorf("Load(missing) should return defaults")
	}
}
