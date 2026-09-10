package components

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/kesonglab/video-interpolate/internal/render"
)

// MenuItem is one selectable row in a single-choice menu.
type MenuItem struct {
	Label       string
	Description string
	Tag         string // "recommended" / "heavy"
	TagStyle    lipgloss.Style
}

// Menu renders a vertical radio list. cursor is the index with the `▶` marker;
// the rest get `○`.
func Menu(items []MenuItem, cursor int, width int) string {
	out := make([]string, len(items))
	for i, item := range items {
		mark := render.Subtle.Render(render.IconRadioOff)
		if i == cursor {
			mark = render.Primary.Render(render.IconCursor)
		}
		label := item.Label
		if i == cursor {
			label = render.Primary.Render(label)
		}
		var sb strings.Builder
		sb.WriteString(mark + " " + label)
		if item.Description != "" {
			sb.WriteString("  " + render.Subtle.Render(item.Description))
		}
		if item.Tag != "" {
			style := item.TagStyle
			if style.GetForeground() == nil {
				style = render.OK
			}
			sb.WriteString("  " + style.Render("["+item.Tag+"]"))
		}
		out[i] = sb.String()
	}
	return strings.Join(out, "\n")
}
