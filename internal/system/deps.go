package system

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Dependency struct {
	Name    string
	Path    string
	Version string
	OK      bool
	FixHint string
}

// CheckDependencies probes ffmpeg, ffprobe, rife-ncnn-vulkan.
// rife has a special recursive lookup since the GitHub release zip unpacks
// into a versioned subdirectory like rife-ncnn-vulkan-<date>-macos/.
func CheckDependencies() []Dependency {
	deps := []Dependency{
		probeDep("ffmpeg", "ffmpeg", "brew install ffmpeg"),
		probeDep("ffprobe", "ffprobe", "brew install ffmpeg"),
	}
	d := Dependency{Name: "rife-ncnn-vulkan", FixHint: "download from https://github.com/nihui/rife-ncnn-vulkan"}
	if p, err := FindRifeBinary(); err == nil {
		d.Path = p
		d.OK = true
	}
	deps = append(deps, d)
	return deps
}

func probeDep(name, bin, hint string) Dependency {
	d := Dependency{Name: name, Path: FindExecutable(bin), FixHint: hint}
	if d.Path != "" {
		d.OK = true
		d.Version = GetVersion(bin)
	}
	return d
}

// FindRifeBinary looks for rife-ncnn-vulkan in PATH then recursively in
// ~/rife-ncnn-vulkan/. The GitHub release zip unpacks into a versioned
// subdirectory (e.g. rife-ncnn-vulkan-20221029-macos/) so a fixed path
// check won't find it — we walk the tree and pick the first match.
func FindRifeBinary() (string, error) {
	if p := FindExecutable("rife-ncnn-vulkan"); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	root := filepath.Join(home, "rife-ncnn-vulkan")
	var found string
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if d.Name() == "rife-ncnn-vulkan" {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	if walkErr != nil && !errors.Is(walkErr, fs.ErrNotExist) {
		return "", walkErr
	}
	if found != "" {
		return found, nil
	}
	return "", fmt.Errorf("rife-ncnn-vulkan not found in PATH or %s", root)
}

// FindExecutable is a helper.
func FindExecutable(name string) string {
	p, err := exec.LookPath(name)
	if err != nil {
		return ""
	}
	return p
}

// GetVersion runs `name -version` and parses output (best-effort).
func GetVersion(name string) string {
	out, err := exec.Command(name, "-version").CombinedOutput()
	if err != nil {
		return ""
	}
	for _, f := range strings.Fields(string(out)) {
		if f != "" && f[0] >= '0' && f[0] <= '9' {
			return f
		}
	}
	return strings.TrimSpace(strings.SplitN(string(out), "\n", 2)[0])
}
