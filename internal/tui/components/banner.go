package components

import (
	"strings"

	"github.com/kesonglab/video-interpolate/internal/render"
)

// Banner renders the queen 3-line header: sep, crown title + author, sep.
func Banner(title, author string, width int) string {
	if width < 20 {
		width = 20
	}
	sep := render.Sep.Render(strings.Repeat("━", width))
	line := render.Title.Render("  👑 " + title + "  ")
	if author != "" {
		line += render.Gray.Render(author)
	}
	return sep + "\n" + line + "\n" + sep
}
