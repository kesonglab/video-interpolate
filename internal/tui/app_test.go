package tui

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/kesonglab/video-interpolate/internal/config"
)

func keyPress(code rune, text string) tea.KeyMsg {
	return tea.KeyPressMsg(tea.Key{Code: code, Text: text})
}

func enterKey() tea.KeyMsg { return keyPress(tea.KeyEnter, "") }

func newTestApp() *App {
	return New(config.Default(), nil)
}

// pumpNav runs Update and re-dispatches the navigation/quit messages pages
// return. Other commands (cursor blink etc.) are left alone so we don't spin.
func pumpNav(a *App, msg tea.Msg) {
	_, cmd := a.Update(msg)
	if cmd == nil {
		return
	}
	switch res := cmd().(type) {
	case gotoMsg:
		a.Update(res)
	}
}

func TestAppStateTransitions(t *testing.T) {
	a := newTestApp()
	a.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	if a.page != pageWelcome {
		t.Fatalf("start page = %d, want %d", a.page, pageWelcome)
	}

	// enter on welcome → files
	pumpNav(a, enterKey())
	if a.page != pageFiles {
		t.Fatalf("after enter page = %d, want %d", a.page, pageFiles)
	}

	// esc on files → welcome
	pumpNav(a, keyPress(tea.KeyEsc, ""))
	if a.page != pageWelcome {
		t.Fatalf("after esc page = %d, want %d", a.page, pageWelcome)
	}
}

func TestAppQuit(t *testing.T) {
	a := newTestApp()
	a.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	pumpNav(a, keyPress('q', "q"))
	if !a.quit {
		t.Fatal("quit flag not set")
	}
}

func TestAppWindowSize(t *testing.T) {
	a := newTestApp()
	a.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	if a.width != 100 || a.height != 50 {
		t.Fatalf("size = %dx%d, want 100x50", a.width, a.height)
	}
	if a.ctx.Width != 100 || a.ctx.Height != 50 {
		t.Fatalf("ctx size = %dx%d", a.ctx.Width, a.ctx.Height)
	}
}

// TestPageViews drives Init+Update+View on every page and checks the output
// contains the expected markers.
func TestPageViews(t *testing.T) {
	a := newTestApp()
	a.Update(tea.WindowSizeMsg{Width: 100, Height: 40})

	cases := []struct {
		page pageID
		want []string
	}{
		{pageWelcome, []string{"Interpolate", "RIFE frame interpolation"}},
		{pageFiles, []string{"FilePicker", "Sources"}},
		{pageMultiplier, []string{"Multiplier", "x2", "x4", "x8"}},
		{pageEncoder, []string{"Encoder", "VideoToolbox", "libx264", "Preset"}},
		{pageConfirm, []string{"Confirm", "Encoder", "Output"}},
	}
	for _, c := range cases {
		page := a.pages[c.page]
		if cmd := page.Init(); cmd != nil {
			cmd()
		}
		page.Update(tea.WindowSizeMsg{Width: 100, Height: 40})
		content := page.View().Content
		for _, want := range c.want {
			if !strings.Contains(content, want) {
				t.Errorf("page %d missing %q in:\n%s", c.page, want, content)
			}
		}
	}
}
