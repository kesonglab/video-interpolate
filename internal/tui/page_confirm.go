package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/kesonglab/video-interpolate/internal/render"
	"github.com/kesonglab/video-interpolate/internal/tui/components"
)

type confirmPage struct {
	ctx *SharedContext
	km  keymap
}

func NewConfirmPage(ctx *SharedContext) Page {
	return &confirmPage{ctx: ctx, km: defaultKeymap()}
}

func (p *confirmPage) Init() tea.Cmd { return nil }

func (p *confirmPage) Update(msg tea.Msg) (Page, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch {
		case p.km.matches(msg, p.km.Esc):
			return p, Goto(pageEncoder)
		case p.km.matches(msg, p.km.Enter):
			return p, StartProcessing()
		}
	}
	return p, nil
}

func (p *confirmPage) View() tea.View {
	width := p.ctx.Width
	if width < 60 {
		width = 80
	}

	lines := []string{
		render.SectionTitle("Setup"),
		"",
		"  Files    " + render.Accent.Render(fmt.Sprintf("%d video(s)", len(p.ctx.Files))),
		"  Source   " + render.Accent.Render(fmt.Sprintf("%.0f fps → %.0f fps (x%d)", p.sourceFPS(), p.targetFPS(), p.ctx.Multiplier)),
		"  Encoder  " + render.Accent.Render(encoderLabel(p.ctx.Encoder)+" · "+p.ctx.Quality),
		"  Output   " + render.Accent.Render(p.ctx.OutputDir),
	}

	body := components.Banner(appTitle(), authorLine, width) + "\n\n" +
		strings.Join(lines, "\n") + "\n\n" +
		components.KeyHint([]components.HintPair{
			{Key: "enter", Desc: "start processing"},
			{Key: "esc", Desc: "back"},
			{Key: "q", Desc: "quit"},
		})

	return tea.NewView(body)
}

// sourceFPS is the first probed source's fps, or a placeholder.
func (p *confirmPage) sourceFPS() float64 {
	if p.ctx.SourceFPS > 0 {
		return p.ctx.SourceFPS
	}
	if len(p.ctx.FileInfo) > 0 {
		if _, _, fps, _ := parseMediaSummary(p.ctx.FileInfo[0]); fps > 0 {
			p.ctx.SourceFPS = fps
			return fps
		}
	}
	return 30
}

// targetFPS is source × multiplier, or a sensible placeholder when unprobed.
func (p *confirmPage) targetFPS() float64 {
	return p.sourceFPS() * float64(p.ctx.Multiplier)
}
