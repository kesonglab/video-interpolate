package tui

import (
	"context"
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
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
		key := msg.Key()
		switch {
		case p.km.matches(msg, p.km.Esc):
			return p, Goto(pageFiles)
		case p.km.matches(msg, p.km.Up) || key.Text == "k":
			if p.cursor > 0 {
				p.cursor--
			}
		case p.km.matches(msg, p.km.Down) || key.Text == "j":
			if p.cursor < len(multiplierItems)-1 {
				p.cursor++
			}
		case key.Text == "1" || key.Text == "2" || key.Text == "3":
			p.cursor = int(key.Text[0] - '1')
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

	fps := p.sourceFPS()
	var rows []string
	for i, m := range multiplierItems {
		desc := "output fps depends on your source"
		if fps > 0 {
			desc = fmt.Sprintf("%.0f fps (from %.0f fps source)", fps*float64(m), fps)
		}
		row := render.MenuRow(p.cursor, i, fmt.Sprint(i+1), fmt.Sprintf("x%d", m), desc)
		if i == 0 {
			row += "  " + render.Green.Render("[recommended]")
		}
		if i == 2 {
			row += "  " + render.Yellow.Render("[heavy]")
		}
		rows = append(rows, row)
	}

	body := components.Banner(appTitle(), authorLine, width) + "\n\n" +
		strings.Join(rows, "\n") + "\n\n" +
		components.KeyHint([]components.HintPair{
			{Key: "↑/↓", Desc: "navigate"},
			{Key: "Enter", Desc: "confirm"},
			{Key: "1-3", Desc: "jump"},
			{Key: "esc", Desc: "back"},
		})

	return tea.NewView(body)
}

// sourceFPS returns the first source's fps, probing if we haven't yet.
func (p *multiplierPage) sourceFPS() float64 {
	if p.ctx.SourceFPS > 0 {
		return p.ctx.SourceFPS
	}
	if len(p.ctx.FileInfo) > 0 {
		if _, _, fps, _ := parseMediaSummary(p.ctx.FileInfo[0]); fps > 0 {
			p.ctx.SourceFPS = fps
			return fps
		}
	}
	if len(p.ctx.Files) > 0 {
		if info, err := probe.Probe(context.Background(), p.ctx.Files[0]); err == nil && info.FPS > 0 {
			p.ctx.SourceFPS = info.FPS
			return info.FPS
		}
	}
	return 0
}
