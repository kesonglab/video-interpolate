package encode

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// ExtractAudio copies audio stream from Input to Output as-is.
// Returns false if no audio stream exists.
func ExtractAudio(ctx context.Context, input, output string) (bool, error) {
	cmd := exec.CommandContext(ctx, "ffmpeg", "-y", "-v", "error",
		"-i", input, "-vn", "-acodec", "copy", output)
	out, err := cmd.CombinedOutput()
	if err != nil {
		// no audio stream makes ffmpeg exit non-zero
		return false, fmt.Errorf("ffmpeg: %v: %s", err, out)
	}
	return true, nil
}

// MuxAudio copies the audio track into videoPath, replacing it in place.
func MuxAudio(ctx context.Context, videoPath, audioPath string) error {
	tmp := videoPath + ".mux.mp4"
	cmd := exec.CommandContext(ctx, "ffmpeg", "-y", "-v", "error",
		"-i", videoPath, "-i", audioPath,
		"-map", "0:v:0", "-map", "1:a:0",
		"-c", "copy", "-movflags", "+faststart", tmp)
	out, err := cmd.CombinedOutput()
	if err != nil {
		os.Remove(tmp)
		return fmt.Errorf("ffmpeg mux: %v: %s", err, out)
	}
	return os.Rename(tmp, videoPath)
}
