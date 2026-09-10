package tui

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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
			if p.group == groupEncoder && p.encCur > 0 {
				p.encCur--
			} else if p.group == groupPreset && p.preCur > 0 {
				p.preCur--
			}
		case p.km.matches(msg, p.km.Down) || key.Text == "j":
			if p.group == groupEncoder && p.encCur < len(encoderChoices)-1 {
				p.encCur++
			} else if p.group == groupPreset && p.preCur < len(presetChoices)-1 {
				p.preCur++
			}
		case p.km.matches(msg, p.km.Enter):
			p.ctx.Encoder = encoderChoices[p.encCur].Value
			p.ctx.Quality = presetChoices[p.preCur]
			return p, Goto(pageConfirm)
		}
	}
	return p, nil
}

func (p *encoderPage) View() tea.View {
	width := p.ctx.Width
	if width < 60 {
		width = 80
	}

	var encItems []components.MenuItem
	for i := range encoderChoices {
		encItems = append(encItems, components.MenuItem{
			Label:       encoderChoices[i].Label,
			Description: encoderDescs[i],
		})
	}
	var preItems []components.MenuItem
	for i := range presetChoices {
		preItems = append(preItems, components.MenuItem{
			Label:       presetChoices[i],
			Description: presetDescs[i],
		})
	}

	// Only the focused group gets the `▶` cursor; the other shows `○`.
	encCursor := -1
	preCursor := -1
	if p.group == groupEncoder {
		encCursor = p.encCur
	} else {
		preCursor = p.preCur
	}

	encCard := components.Card(render.IconEncode, "Encoder", []string{
		"",
		components.Menu(encItems, encCursor, width),
		"",
	}, width)
	preCard := components.Card(render.IconOutput, "Preset", []string{
		"",
		components.Menu(preItems, preCursor, width),
		"",
	}, width)

	body := lipgloss.JoinVertical(lipgloss.Left,
		components.Header("Encoder", render.IconSystem, p.hardwareText(), render.Primary, "", width),
		components.Separator(width),
		"",
		encCard,
		"",
		preCard,
		"",
		components.KeyHint([]components.HintPair{
			{Key: "↑/↓", Desc: "select"},
			{Key: "tab", Desc: "switch group"},
			{Key: "enter", Desc: "next"},
			{Key: "esc", Desc: "back"},
		}),
	)

	return tea.NewView(body)
}

func (p *encoderPage) hardwareText() string {
	if p.ctx.Caps != nil && p.ctx.Caps.GPUName != "" {
		return p.ctx.Caps.GPUName
	}
	return "VideoToolbox"
}
