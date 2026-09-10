package components

import "github.com/kesonglab/video-interpolate/internal/render"

// Box wraps content in the blue rounded border used by inputs and modals.
func Box(content string, width int) string {
	if width < 20 {
		width = 20
	}
	return render.Box.Width(width).Render(content)
}
