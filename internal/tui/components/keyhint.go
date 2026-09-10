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

// KeyHint renders "[key] action" pairs separated by double spaces.
func KeyHint(pairs []HintPair) string {
	var parts []string
	for _, p := range pairs {
		parts = append(parts, render.Subtle.Render(p.Key)+" "+p.Desc)
	}
	return strings.Join(parts, "  ")
}
