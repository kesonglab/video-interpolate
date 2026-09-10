package config

// Default returns the built-in defaults; the config editor will expose these.
func Default() *Config {
	return &Config{
		OutputDir:    "~/Downloads/补帧60fps/",
		Multiplier:   2,
		Encoder:      "auto",
		Quality:      "balanced",
		GPU:          0,
		Threads:      "4:4:4",
		Format:       "jpg",
		JPGQuality:   2,
		HWDecode:     "auto",
		Jobs:         1,
		SkipExisting: true,
		Audio:        "copy",
		ChunkSize:    3000,
	}
}
