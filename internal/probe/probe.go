package probe

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

type VideoInfo struct {
	Path       string
	Width      int
	Height     int
	FPS        float64
	Duration   time.Duration
	FrameCount int
	HasAudio   bool
	Codec      string
	PixelFmt   string
	FileSize   int64
	Bitrate    int64
}

// Probe runs ffprobe JSON and extracts the video stream stats.
func Probe(ctx context.Context, path string) (*VideoInfo, error) {
	out, err := exec.CommandContext(ctx, "ffprobe",
		"-v", "error", "-print_format", "json", "-show_format", "-show_streams", path,
	).Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe: %w", err)
	}

	var parsed ffprobeOutput
	if err := json.Unmarshal(out, &parsed); err != nil {
		return nil, fmt.Errorf("parse ffprobe output: %w", err)
	}

	var video *ffprobeStream
	for i := range parsed.Streams {
		if parsed.Streams[i].CodecType == "video" {
			video = &parsed.Streams[i]
			break
		}
	}
	if video == nil {
		return nil, errors.New("no video stream found")
	}

	info := &VideoInfo{
		Path:       path,
		Width:      video.Width,
		Height:     video.Height,
		Codec:      video.CodecName,
		PixelFmt:   video.PixFmt,
		Duration:   parseDuration(firstNonEmpty(video.Duration, parsed.Format.Duration)),
		FrameCount: parseInt(firstNonEmpty(video.NbFrames, "")),
		FileSize:   parseInt64(parsed.Format.Size),
		Bitrate:    parseInt64(parsed.Format.BitRate),
	}
	if fr := parseFrameRate(video.AvgFrameRate); fr > 0 {
		info.FPS = fr
	}
	for _, s := range parsed.Streams {
		if s.CodecType == "audio" {
			info.HasAudio = true
			break
		}
	}
	return info, nil
}

type ffprobeOutput struct {
	Streams []ffprobeStream `json:"streams"`
	Format  ffprobeFormat   `json:"format"`
}

type ffprobeStream struct {
	CodecType    string `json:"codec_type"`
	CodecName    string `json:"codec_name"`
	Width        int    `json:"width"`
	Height       int    `json:"height"`
	Duration     string `json:"duration"`
	PixFmt       string `json:"pix_fmt"`
	AvgFrameRate string `json:"avg_frame_rate"`
	NbFrames     string `json:"nb_frames"`
}

type ffprobeFormat struct {
	Duration string `json:"duration"`
	Size     string `json:"size"`
	BitRate  string `json:"bit_rate"`
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func parseDuration(s string) time.Duration {
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil || f <= 0 {
		return 0
	}
	return time.Duration(f * float64(time.Second))
}

// parseFrameRate handles both "30" and "30000/1001" forms.
func parseFrameRate(s string) float64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	if strings.Contains(s, "/") {
		parts := strings.SplitN(s, "/", 2)
		num, err1 := strconv.ParseFloat(parts[0], 64)
		den, err2 := strconv.ParseFloat(parts[1], 64)
		if err1 == nil && err2 == nil && den > 0 {
			return num / den
		}
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return f
}

func parseInt(s string) int {
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return 0
	}
	return n
}

func parseInt64(s string) int64 {
	n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0
	}
	return n
}
