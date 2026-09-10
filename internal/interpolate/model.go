package interpolate

import (
	"io/fs"
	"path/filepath"
)

var modelPriority = []string{"rife-v4.6", "rife-v4", "rife-v2.3", "rife-anime"}

// DetectModel picks the best model in rifeBinDir.
// Priority: rife-v4.6 > rife-v4 > rife-v2.3 > rife-anime.
func DetectModel(rifeBinDir string) string {
	if rifeBinDir == "" {
		return ""
	}
	abs, err := filepath.Abs(rifeBinDir)
	if err != nil {
		return ""
	}

	found := map[string]string{}
	_ = filepath.WalkDir(abs, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		for _, m := range modelPriority {
			if d.Name() == m {
				found[m] = path
			}
		}
		return nil
	})

	for _, m := range modelPriority {
		if p, ok := found[m]; ok {
			return p
		}
	}
	return ""
}
