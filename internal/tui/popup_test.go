package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/kesonglab/video-interpolate/internal/render"
)

func TestPopup_Tab(t *testing.T) {
	p := NewPopup("test", "Title", "Body", []string{"A", "B", "C"}, render.OK).Open()

	p, _ = p.Update(keyPress(tea.KeyTab, ""))
	if p.cursor != 1 {
		t.Fatalf("cursor after tab = %d, want 1", p.cursor)
	}
	p, _ = p.Update(keyPress(tea.KeyTab, ""))
	if p.cursor != 2 {
		t.Fatalf("cursor after second tab = %d, want 2", p.cursor)
	}
	p, _ = p.Update(keyPress(tea.KeyTab, ""))
	if p.cursor != 0 {
		t.Fatalf("cursor should wrap to 0, got %d", p.cursor)
	}
	p, _ = p.Update(keyPress(tea.KeyLeft, ""))
	if p.cursor != 0 {
		t.Fatalf("left at index 0 should stay, got %d", p.cursor)
	}
}

func TestPopup_Enter(t *testing.T) {
	p := NewPopup("confirm", "Title", "Body", []string{"No", "Yes"}, render.Warn).Open()

	p, cmd := p.Update(enterKey())
	if cmd == nil {
		t.Fatal("enter returned no command")
	}
	res, ok := cmd().(popupResultMsg)
	if !ok {
		t.Fatalf("enter returned %T, want popupResultMsg", cmd())
	}
	if res.ID != "confirm" || !res.Chosen || res.ButtonIndex != 0 {
		t.Fatalf("result = %+v", res)
	}
	if p.Active() {
		t.Fatal("popup should close after a choice")
	}
}

func TestPopup_Esc(t *testing.T) {
	p := NewPopup("confirm", "Title", "Body", []string{"No", "Yes"}, render.Warn).Open()

	_, cmd := p.Update(keyPress(tea.KeyEsc, ""))
	if cmd == nil {
		t.Fatal("esc returned no command")
	}
	res, ok := cmd().(popupResultMsg)
	if !ok {
		t.Fatalf("esc returned %T, want popupResultMsg", cmd())
	}
	if res.Chosen {
		t.Fatal("esc should report Chosen=false")
	}
}
