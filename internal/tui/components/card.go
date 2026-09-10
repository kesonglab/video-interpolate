package components

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/kesonglab/video-interpolate/internal/render"
)

// Card renders a Mole-style titled panel: "ICON TITLE ╌╌╌..." header then the
// body lines below.
func Card(icon, title string, lines []string, width int) string {
	if width < 20 {
		width = 20
	}
	iconTitle := render.Title.Render(icon + " " + title)
	lineLen := width - lipgloss.Width(iconTitle) - 2
	if lineLen < 0 {
		lineLen = 0
	}
	header := iconTitle + "  " + render.Line.Render(strings.Repeat("╌", lineLen))
	out := append([]string{header}, lines...)
	return strings.Join(out, "\n")
}
