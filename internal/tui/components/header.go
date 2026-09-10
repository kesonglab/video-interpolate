package components

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/kesonglab/video-interpolate/internal/render"
)

// Header renders "TITLE  STATUS  RIGHT" with the title in the app's purple and
// the right side padded out to the full width.
func Header(title, statusIcon, statusText string, statusStyle lipgloss.Style, right string, width int) string {
	left := render.Title.Render(title) + "  " + statusStyle.Render(statusIcon+" "+statusText)
	if right == "" {
		return left
	}
	gap := width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 2 {
		gap = 2
	}
	return left + strings.Repeat(" ", gap) + render.Primary.Render(right)
}

// Separator renders a full-width `═` line in the line color.
func Separator(width int) string {
	return render.Line.Render(strings.Repeat("═", width))
}

// SubSeparator renders a `╌` dashed line across the width.
func SubSeparator(width int) string {
	return render.Line.Render(strings.Repeat("╌", width))
}
