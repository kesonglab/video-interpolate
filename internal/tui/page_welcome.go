package tui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/kesonglab/video-interpolate/internal/render"
	"github.com/kesonglab/video-interpolate/internal/tui/components"
)

// welcomePage is the landing screen: title, mascot, hint.
type welcomePage struct {
	ctx   *SharedContext
	km    keymap
	frame int
}

func NewWelcomePage(ctx *SharedContext) Page {
	return &welcomePage{ctx: ctx, km: defaultKeymap()}
}

func (p *welcomePage) Init() tea.Cmd { return nil }

func (p *welcomePage) Update(msg tea.Msg) (Page, tea.Cmd) {
	switch msg := msg.(type) {
	case tickMsg:
		p.frame = (p.frame + 1) % len(reelyFrames)
		return p, mascotTick()
	case tea.KeyMsg:
		if p.km.matches(msg, p.km.Enter) {
			return p, Goto(pageFiles)
		}
	}
	return p, nil
}

func (p *welcomePage) View() tea.View {
	width := p.ctx.Width
	if width < 40 {
		width = 80
	}
	title := render.Title.Render("Interpolate")
	sub := render.Subtle.Render("RIFE frame interpolation for macOS")
	hint := render.Subtle.Render("Drag video files or folders here, paste a path with [p] or press [enter] to continue")

	center := lipgloss.JoinVertical(
		lipgloss.Center,
		title,
		"",
		mascot(p.frame, render.Primary),
		"",
		sub,
		"",
		hint,
	)

	full := lipgloss.JoinVertical(
		lipgloss.Center,
		components.Separator(width),
		center,
		components.Separator(width),
	)

	return tea.NewView(full)
}
