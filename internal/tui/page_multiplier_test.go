package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestPageMultiplier_View(t *testing.T) {
	ctx := newProcessingCtx()
	ctx.Multiplier = 2
	p := NewMultiplierPage(ctx).(*multiplierPage)

	content := stripViewANSI(p.View().Content)
	for _, want := range []string{"x2", "x4", "x8", "▶", "recommended", "heavy"} {
		if !strings.Contains(content, want) {
			t.Errorf("multiplier view missing %q in:\n%s", want, content)
		}
	}
	if p.cursor != 0 {
		t.Fatalf("cursor = %d, want 0", p.cursor)
	}
}

func TestPageMultiplier_NumberJump(t *testing.T) {
	ctx := newProcessingCtx()
	p := NewMultiplierPage(ctx).(*multiplierPage)

	p.Update(keyPress('3', "3"))
	if p.cursor != 2 {
		t.Fatalf("cursor after 3 = %d, want 2", p.cursor)
	}
	p.Update(keyPress(tea.KeyUp, ""))
	if p.cursor != 1 {
		t.Fatalf("cursor after up = %d, want 1", p.cursor)
	}
	p.Update(enterKey())
	if ctx.Multiplier != 4 {
		t.Fatalf("multiplier = %d, want 4", ctx.Multiplier)
	}
}
