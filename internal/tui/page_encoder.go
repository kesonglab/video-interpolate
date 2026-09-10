package tui

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/kesonglab/video-interpolate/internal/render"
	"github.com/kesonglab/video-interpolate/internal/tui/components"
)

type encoderGroup int

const (
	groupEncoder encoderGroup = iota
	groupPreset
)

// encoderChoice maps a UI label to the config encoder value.
var encoderChoices = []struct {
	Label string
	Value string
}{
	{"VideoToolbox h264", "h264_videotoolbox"},
	{"VideoToolbox hevc", "hevc_videotoolbox"},
	{"libx264", "libx264"},
	{"libx265", "libx265"},
}

var encoderDescs = []string{
	"hardware · fastest · default",
	"hardware · smaller files",
	"software · compatibility",
	"software · best compression",
}

var presetChoices = []string{"quality", "balanced", "speed"}
var presetDescs = []string{"slower, smaller output", "default", "faster, larger output"}

type encoderPage struct {
	ctx    *SharedContext
	km     keymap
	group  encoderGroup
	encCur int
	preCur int
}

func NewEncoderPage(ctx *SharedContext) Page {
	encCur := 1 // hevc_videotoolbox
	switch ctx.Encoder {
	case "h264_videotoolbox":
		encCur = 0
	case "libx264":
		encCur = 2
	case "libx265":
		encCur = 3
	}
	preCur := 1 // balanced
	switch ctx.Quality {
	case "quality":
		preCur = 0
	case "speed":
		preCur = 2
	}
	return &encoderPage{ctx: ctx, km: defaultKeymap(), encCur: encCur, preCur: preCur}
}

func (p *encoderPage) Init() tea.Cmd { return nil }

func (p *encoderPage) Update(msg tea.Msg) (Page, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.Key()
		switch {
		case p.km.matches(msg, p.km.Esc):
			return p, Goto(pageMultiplier)
		case p.km.matches(msg, p.km.Tab):
			p.group = (p.group + 1) % 2
		case p.km.matches(msg, p.km.Up) || key.Text == "k":
			p.move(-1)
		case p.km.matches(msg, p.km.Down) || key.Text == "j":
			p.move(1)
		case key.Text >= "1" && key.Text <= "4":
			if p.jump(int(key.Text[0] - '1')) {
				return p, nil
			}
		case p.km.matches(msg, p.km.Enter):
			p.ctx.Encoder = encoderChoices[p.encCur].Value
			p.ctx.Quality = presetChoices[p.preCur]
			return p, Goto(pageConfirm)
		}
	}
	return p, nil
}

// move steps the cursor within the focused group.
func (p *encoderPage) move(delta int) {
	if p.group == groupEncoder {
		if n := p.encCur + delta; n >= 0 && n < len(encoderChoices) {
			p.encCur = n
		}
		return
	}
	if n := p.preCur + delta; n >= 0 && n < len(presetChoices) {
		p.preCur = n
	}
}

// jump selects item n in the focused group; false when out of range.
func (p *encoderPage) jump(n int) bool {
	if p.group == groupEncoder {
		if n < len(encoderChoices) {
			p.encCur = n
			return true
		}
		return false
	}
	if n < len(presetChoices) {
		p.preCur = n
		return true
	}
	return false
}

func (p *encoderPage) View() tea.View {
	width := p.ctx.Width
	if width < 60 {
		width = 80
	}

	var b strings.Builder
	b.WriteString(components.Banner(appTitle(), "Step 3/4  Encoder & Preset", width))
	b.WriteString("\n\n")
	b.WriteString(render.SectionTitle("Encoder"))
	b.WriteString("\n")
	encCursor := -1
	if p.group == groupEncoder {
		encCursor = p.encCur
	}
	for i := range encoderChoices {
		b.WriteString(render.MenuRow(encCursor, i, fmt.Sprint(i+1), encoderChoices[i].Label, encoderDescs[i]))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(render.SectionTitle("Preset"))
	b.WriteString("\n")
	preCursor := -1
	if p.group == groupPreset {
		preCursor = p.preCur
	}
	for i := range presetChoices {
		b.WriteString(render.MenuRow(preCursor, i, fmt.Sprint(i+1), presetChoices[i], presetDescs[i]))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(components.KeyHint([]components.HintPair{
		{Key: "↑/↓", Desc: "navigate"},
		{Key: "tab", Desc: "switch"},
		{Key: "1-4", Desc: "jump"},
		{Key: "enter", Desc: "confirm"},
		{Key: "esc", Desc: "back"},
	}))

	return tea.NewView(b.String())
}
