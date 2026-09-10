package system

import (
	"fmt"
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
func CheckDependencies() []Dependency {
	probes := []struct {
		name string
		bin  string
		hint string
	}{
		{"ffmpeg", "ffmpeg", "brew install ffmpeg"},
		{"ffprobe", "ffprobe", "brew install ffmpeg"},
		{"rife-ncnn-vulkan", "rife-ncnn-vulkan", "download from https://github.com/nihui/rife-ncnn-vulkan"},
	}
	deps := make([]Dependency, 0, len(probes))
	for _, p := range probes {
		d := Dependency{Name: p.name, Path: FindExecutable(p.bin)}
		if d.Path != "" {
			d.OK = true
			d.Version = GetVersion(p.bin)
		} else {
			d.FixHint = p.hint
		}
		deps = append(deps, d)
	}
	return deps
}

// FindRifeBinary looks for rife-ncnn-vulkan in PATH then ~/rife-ncnn-vulkan.
func FindRifeBinary() (string, error) {
	if p := FindExecutable("rife-ncnn-vulkan"); p != "" {
		return p, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	for _, c := range []string{
		filepath.Join(home, "rife-ncnn-vulkan", "rife-ncnn-vulkan"),
		filepath.Join(home, "rife-ncnn-vulkan", "bin", "rife-ncnn-vulkan"),
	} {
		if fi, err := os.Stat(c); err == nil && !fi.IsDir() {
			return c, nil
		}
	}
	return "", fmt.Errorf("rife-ncnn-vulkan not found")
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
