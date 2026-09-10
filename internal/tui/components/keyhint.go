package components

import (
	"strings"

	"github.com/kesonglab/video-interpolate/internal/render"
)

// HintPair is one key + action shown in the footer, e.g. enter → next.
type HintPair struct {
	Key  string
	Desc string
}

// KeyHint renders "key action" pairs in the dim gray footer style.
func KeyHint(pairs []HintPair) string {
	var parts []string
	for _, p := range pairs {
		parts = append(parts, render.Dim.Render(p.Key)+" "+p.Desc)
	}
	return strings.Join(parts, " · ")
}
