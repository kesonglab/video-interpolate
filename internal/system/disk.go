package system

import "syscall"

// AvailableSpace returns free bytes at path.
func AvailableSpace(path string) (uint64, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return 0, err
	}
	return uint64(st.Bavail) * uint64(st.Bsize), nil
}

// EstimateRequiredSpace estimates disk usage for processing a video.
// Rough heuristic: 2x raw frames as jpg + output size.
func EstimateRequiredSpace(width, height int, frameCount int64, multiplier int) uint64 {
	if width <= 0 || height <= 0 || frameCount <= 0 {
		return 0
	}
	if multiplier < 1 {
		multiplier = 1
	}
	// jpg at q2 lands around 1/3 byte per pixel; double for slack.
	perFrame := uint64(width) * uint64(height) / 3
	frames := uint64(frameCount) * uint64(multiplier) * 2
	output := uint64(width) * uint64(height) * 4 / 3
	return perFrame*frames + output
}
