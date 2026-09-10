package tui

import (
	"context"
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/kesonglab/video-interpolate/internal/probe"
	"github.com/kesonglab/video-interpolate/internal/render"
	"github.com/kesonglab/video-interpolate/internal/tui/components"
)

type multiplierPage struct {
	ctx    *SharedContext
	km     keymap
	cursor int
}

var multiplierItems = []int{2, 4, 8}

func NewMultiplierPage(ctx *SharedContext) Page {
	cur := 0
	switch ctx.Multiplier {
	case 4:
		cur = 1
	case 8:
		cur = 2
	}
	return &multiplierPage{ctx: ctx, km: defaultKeymap(), cursor: cur}
}

func (p *multiplierPage) Init() tea.Cmd { return nil }

func (p *multiplierPage) Update(msg tea.Msg) (Page, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case p.km.matches(msg, p.km.Esc):
			return p, Goto(pageFiles)
		case p.km.matches(msg, p.km.Up) || msg.Key().Text == "k":
			if p.cursor > 0 {
				p.cursor--
			}
		case p.km.matches(msg, p.km.Down) || msg.Key().Text == "j":
			if p.cursor < len(multiplierItems)-1 {
				p.cursor++
			}
		case p.km.matches(msg, p.km.Enter):
			p.ctx.Multiplier = multiplierItems[p.cursor]
			return p, Goto(pageEncoder)
		}
	}
	return p, nil
}

func (p *multiplierPage) View() tea.View {
	width := p.ctx.Width
	if width < 60 {
		width = 80
	}

	var items []components.MenuItem
	for i, m := range multiplierItems {
		item := components.MenuItem{
			Label: fmt.Sprintf("x%d", m),
			Tag:   "",
		}
		if i == 0 {
			item.Tag = "recommended"
			item.TagStyle = render.OK
		}
		if i == 2 {
			item.Tag = "heavy"
			item.TagStyle = render.Warn
		}
		items = append(items, item)
	}

	card := components.Card(render.IconRife, "Multiplier", []string{
		"",
		components.Menu(items, p.cursor, width),
		"",
		render.Subtle.Render(p.targetHint()),
	}, width)

	body := lipgloss.JoinVertical(lipgloss.Left,
		components.Header("Multiplier", render.IconSystem, p.hardwareText(), render.Primary, "", width),
		components.Separator(width),
		"",
		card,
		"",
		components.KeyHint([]components.HintPair{
			{Key: "↑/↓", Desc: "select"},
			{Key: "enter", Desc: "next"},
			{Key: "esc", Desc: "back"},
		}),
	)

	return tea.NewView(body)
}

// targetHint shows the resulting output fps when a source is probed.
func (p *multiplierPage) targetHint() string {
	if len(p.ctx.Files) == 0 {
		return "output fps depends on your source"
	}
	info, err := probe.Probe(context.Background(), p.ctx.Files[0])
	if err != nil || info.FPS <= 0 {
		return "output fps depends on your source"
	}
	target := info.FPS * float64(multiplierItems[p.cursor])
	return fmt.Sprintf("source %.0ffps → output %.0ffps (x%d)", info.FPS, target, multiplierItems[p.cursor])
}

func (p *multiplierPage) hardwareText() string {
	if p.ctx.Caps != nil && p.ctx.Caps.GPUName != "" {
		return p.ctx.Caps.GPUName
	}
	return "VideoToolbox"
}
