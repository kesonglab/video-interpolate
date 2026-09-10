package system

import (
	"os/exec"
	"strings"
)

type Capabilities struct {
	HasVideoToolbox bool
	HasMetal        bool
	GPUCount        int
	GPUName         string
	BestEncoder     string // "hevc_videotoolbox" / "h264_videotoolbox" / "libx264"
}

// DetectCapabilities runs ffmpeg -hide_banner -hwaccels to check VideoToolbox.
// GPU name comes from system_profiler on macOS.
func DetectCapabilities() *Capabilities {
	caps := &Capabilities{GPUCount: 1}

	if out, err := exec.Command("ffmpeg", "-hide_banner", "-hwaccels").CombinedOutput(); err == nil {
		caps.HasVideoToolbox = strings.Contains(string(out), "videotoolbox")
	}

	// system_profiler is macOS-only; skip silently elsewhere.
	if out, err := exec.Command("system_profiler", "SPDisplaysDataType").CombinedOutput(); err == nil {
		s := string(out)
		caps.HasMetal = strings.Contains(s, "Metal")
		caps.GPUName = gpuName(s)
	}

	if caps.HasVideoToolbox {
		caps.BestEncoder = "hevc_videotoolbox"
	} else {
		caps.BestEncoder = "libx264"
	}
	return caps
}

func gpuName(out string) string {
	const marker = "Chipset Model:"
	for _, line := range strings.Split(out, "\n") {
		if i := strings.Index(line, marker); i >= 0 {
			return strings.TrimSpace(line[i+len(marker):])
		}
	}
	return ""
}
