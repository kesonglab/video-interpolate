package components

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/kesonglab/video-interpolate/internal/render"
)

// MenuItem is one selectable row in a single-choice menu.
type MenuItem struct {
	Key         string
	Label       string
	Description string
	Tag         string // "recommended" / "heavy"
	TagStyle    lipgloss.Style
}

// Menu renders a queen-style vertical list. cursor < 0 means nothing selected.
func Menu(items []MenuItem, cursor int, width int) string {
	out := make([]string, len(items))
	for i, item := range items {
		line := render.MenuRow(cursor, i, item.Key, item.Label, item.Description)
		if item.Tag != "" {
			style := item.TagStyle
			if style.GetForeground() == nil {
				style = render.Green
			}
			line += "  " + style.Render("["+item.Tag+"]")
		}
		out[i] = line
	}
	return strings.Join(out, "\n")
}
