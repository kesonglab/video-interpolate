package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/BurntSushi/toml"
)

type Config struct {
	OutputDir    string  `toml:"output_dir"`
	Multiplier   int     `toml:"multiplier"`
	TargetFPS    float64 `toml:"target_fps"` // 0 = auto (source × multiplier)
	Encoder      string  `toml:"encoder"`
	Quality      string  `toml:"quality"` // quality/balanced/speed
	GPU          int     `toml:"gpu"`
	Threads      string  `toml:"threads"`
	Format       string  `toml:"format"`
	JPGQuality   int     `toml:"jpg_quality"`
	HWDecode     string  `toml:"hw_decode"`
	RifeBin      string  `toml:"rife_bin"`
	RifeModel    string  `toml:"rife_model"`
	Jobs         int     `toml:"jobs"`
	SkipExisting bool    `toml:"skip_existing"`
	Audio        string  `toml:"audio"`
	ChunkSize    int     `toml:"chunk_size"`
	WatchDir     string  `toml:"watch_dir"`
}

// Load reads TOML from path; missing file means defaults.
func Load(path string) (*Config, error) {
	cfg := Default()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return cfg, nil
	}
	if _, err := toml.DecodeFile(path, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// Merge applies overrides by matching toml tag or field name.
func (c *Config) Merge(overrides map[string]any) {
	v := reflect.ValueOf(c).Elem()
	t := v.Type()
	for key, val := range overrides {
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if f.Tag.Get("toml") != key && !strings.EqualFold(f.Name, key) {
				continue
			}
			fv := v.Field(i)
			rv := reflect.ValueOf(val)
			if fv.Type() == rv.Type() {
				fv.Set(rv)
			}
		}
	}
}

func (c *Config) Validate() error {
	switch c.Multiplier {
	case 2, 4, 8:
	default:
		return errors.New("multiplier must be 2, 4 or 8")
	}
	switch c.Encoder {
	case "auto", "hevc_videotoolbox", "h264_videotoolbox", "libx264", "libx265":
	default:
		return fmt.Errorf("unsupported encoder %q", c.Encoder)
	}
	switch c.Quality {
	case "quality", "balanced", "speed":
	default:
		return fmt.Errorf("unsupported quality preset %q", c.Quality)
	}
	switch c.Format {
	case "jpg", "png":
	default:
		return fmt.Errorf("unsupported frame format %q", c.Format)
	}
	switch c.Audio {
	case "copy", "reencode", "drop":
	default:
		return fmt.Errorf("unsupported audio mode %q", c.Audio)
	}
	if c.TargetFPS < 0 {
		return fmt.Errorf("target_fps must be >= 0")
	}
	if c.JPGQuality < 1 || c.JPGQuality > 31 {
		return fmt.Errorf("jpg_quality must be 1-31, got %d", c.JPGQuality)
	}
	if c.Jobs < 1 {
		return fmt.Errorf("jobs must be >= 1")
	}
	if c.ChunkSize < 0 {
		return fmt.Errorf("chunk_size must be >= 0")
	}
	return nil
}

func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "video-interpolate", "config.toml")
}
