package tui

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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
		fmt.Sprintf("%s %s", render.Subtle.Render("Files   "), render.Primary.Render(fmt.Sprintf("%d video(s)", len(p.ctx.Files)))),
		fmt.Sprintf("%s %s", render.Subtle.Render("Source  "), render.Primary.Render(fmt.Sprintf("%.0f fps → %.0f fps (x%d)", p.sourceFPS(), p.targetFPS(), p.ctx.Multiplier))),
		fmt.Sprintf("%s %s", render.Subtle.Render("Encoder "), render.Primary.Render(encoderLabel(p.ctx.Encoder)+" · "+p.ctx.Quality)),
		fmt.Sprintf("%s %s", render.Subtle.Render("Output  "), render.Primary.Render(p.ctx.OutputDir)),
	}
	card := components.Card(render.IconSource, "Setup", lines, width)

	body := lipgloss.JoinVertical(lipgloss.Left,
		components.Header("Confirm", render.IconSystem, p.hardwareText(), render.Primary, "", width),
		components.Separator(width),
		"",
		card,
		"",
		components.KeyHint([]components.HintPair{
			{Key: "enter", Desc: "start processing"},
			{Key: "esc", Desc: "back to encoder"},
			{Key: "q", Desc: "quit"},
		}),
	)

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

func (p *confirmPage) hardwareText() string {
	if p.ctx.Caps != nil && p.ctx.Caps.GPUName != "" {
		return p.ctx.Caps.GPUName
	}
	return "VideoToolbox"
}
