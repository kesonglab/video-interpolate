package render

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

var (
	Title    = lipgloss.NewStyle().Foreground(lipgloss.Color("#C79FD7")).Bold(true)
	Primary  = lipgloss.NewStyle().Foreground(lipgloss.Color("#BD93F9"))
	Subtle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#737373"))
	Warn     = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFD75F"))
	Danger   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF5F5F")).Bold(true)
	OK       = lipgloss.NewStyle().Foreground(lipgloss.Color("#A5D6A7"))
	Line     = lipgloss.NewStyle().Foreground(lipgloss.Color("#404040"))
	AlertBar = lipgloss.NewStyle().Foreground(lipgloss.Color("#2B1200")).Background(lipgloss.Color("#FFD75F")).Bold(true).Padding(0, 1)
)

// icons used across the app
const (
	IconSource   = "◉"
	IconRife     = "▶"
	IconEncode   = "◈"
	IconOutput   = "▦"
	IconSystem   = "⚙"
	IconSuccess  = "✓"
	IconFailed   = "✗"
	IconSkipped  = "⚠"
	IconCursor   = "▶"
	IconRadioOn  = "●"
	IconRadioOff = "○"
)

// ProgressBar returns a 16-char bar with color tiered by percentage.
func ProgressBar(pct float64) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	filled := int(pct / 100 * 16)
	bar := strings.Repeat("█", filled) + strings.Repeat("░", 16-filled)
	switch {
	case pct >= 85:
		return Danger.Render(bar)
	case pct >= 60:
		return Warn.Render(bar)
	default:
		return OK.Render(bar)
	}
}

// MiniBar returns a 5-char compact bar.
func MiniBar(pct float64) string {
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	filled := int(pct / 100 * 5)
	bar := strings.Repeat("▮", filled) + strings.Repeat("▯", 5-filled)
	switch {
	case pct >= 85:
		return Danger.Render(bar)
	case pct >= 60:
		return Warn.Render(bar)
	default:
		return OK.Render(bar)
	}
}

var sparkBlocks = []rune{'▁', '▂', '▃', '▄', '▅', '▆', '▇', '█'}

// Sparkline renders an 8-level trend, colored by value.
func Sparkline(data []float64, width int) string {
	if width <= 0 || len(data) == 0 {
		return ""
	}
	if len(data) > width {
		data = data[:width]
	}
	var sb strings.Builder
	for _, v := range data {
		if v < 0 {
			v = 0
		}
		if v > 1 {
			v = 1
		}
		ch := string(sparkBlocks[int(v*7)])
		switch {
		case v >= 0.85:
			sb.WriteString(Danger.Render(ch))
		case v >= 0.6:
			sb.WriteString(Warn.Render(ch))
		default:
			sb.WriteString(OK.Render(ch))
		}
	}
	return sb.String()
}

// Card renders a titled panel. Placeholder until Phase 1b.
func Card(title, body string) string {
	return Title.Render(title) + "\n" + body
}

// StatusCard renders a job status line. Placeholder until Phase 1b.
func StatusCard(title string, progress float64, status string) string {
	return fmt.Sprintf("%s %s %s %s", Title.Render(title), status, MiniBar(progress), ProgressBar(progress))
}
