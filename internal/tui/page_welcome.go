package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/kesonglab/video-interpolate/internal/render"
	"github.com/kesonglab/video-interpolate/internal/tui/components"
)

// welcomePage is the landing screen: banner, intro, key hints.
type welcomePage struct {
	ctx *SharedContext
	km  keymap
}

func NewWelcomePage(ctx *SharedContext) Page {
	return &welcomePage{ctx: ctx, km: defaultKeymap()}
}

func (p *welcomePage) Init() tea.Cmd { return nil }

func (p *welcomePage) Update(msg tea.Msg) (Page, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if p.km.matches(msg, p.km.Enter) {
			return p, Goto(pageFiles)
		}
	}
	return p, nil
}

func (p *welcomePage) View() tea.View {
	width := p.ctx.Width
	if width < 60 {
		width = 80
	}

	banner := components.Banner(appTitle(), authorLine, width)
	content := strings.Join([]string{
		"  " + render.Sel.Render("欢迎使用 vif"),
		"  " + render.Dim.Render("把视频拖进来，选倍率和编码器，剩下的交给 RIFE。"),
		"",
		"  " + render.Accent.Render("[enter]") + " 开始    " + render.Accent.Render("[q]") + " 退出",
	}, "\n")
	footer := render.Dim.Render("enter start · q quit")

	return tea.NewView(banner + "\n\n" + content + "\n\n" + footer)
}
