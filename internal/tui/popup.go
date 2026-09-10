package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/kesonglab/video-interpolate/internal/render"
)

// Popup is a centered modal overlay with title, body, and buttons.
type Popup struct {
	ID         string
	Title      string
	Body       string
	Buttons    []string
	cursor     int
	active     bool
	titleStyle lipgloss.Style
}

func NewPopup(id, title, body string, buttons []string, titleStyle lipgloss.Style) Popup {
	return Popup{
		ID:         id,
		Title:      title,
		Body:       body,
		Buttons:    buttons,
		titleStyle: titleStyle,
	}
}

// Active reports whether the popup is currently shown.
func (p Popup) Active() bool { return p.active }

// Open shows the popup and resets the cursor to the first button.
func (p Popup) Open() Popup {
	p.active = true
	p.cursor = 0
	return p
}

// Update handles key input while active; inactive popups swallow nothing.
func (p Popup) Update(msg tea.Msg) (Popup, tea.Cmd) {
	if !p.active {
		return p, nil
	}
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return p, nil
	}
	key := km.Key()
	switch {
	case key.Code == tea.KeyLeft || (key.Code == tea.KeyTab && key.Mod == tea.ModShift):
		if p.cursor > 0 {
			p.cursor--
		}
	case key.Code == tea.KeyRight || key.Code == tea.KeyTab:
		if len(p.Buttons) > 0 {
			p.cursor = (p.cursor + 1) % len(p.Buttons)
		}
	case isEnter(key):
		id, cursor := p.ID, p.cursor
		p.active = false
		return p, func() tea.Msg {
			return popupResultMsg{ID: id, Chosen: true, ButtonIndex: cursor}
		}
	case key.Code == tea.KeyEsc || key.Code == tea.KeyEscape:
		id := p.ID
		p.active = false
		return p, func() tea.Msg { return popupResultMsg{ID: id} }
	}
	return p, nil
}

// View draws the popup centered over under, on a width×height canvas.
func (p Popup) View(under string, width, height int) string {
	if !p.active {
		return under
	}
	if width < 10 {
		width = 10
	}
	if height < 5 {
		height = 5
	}

	box := p.box()
	bw, bh := lipgloss.Width(box), lipgloss.Height(box)
	x := (width - bw) / 2
	y := (height - bh) / 2
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}

	canvas := lipgloss.NewCanvas(width, height)
	// Compositor applies each layer's X/Y; Canvas.Compose alone ignores them.
	comp := lipgloss.NewCompositor(
		lipgloss.NewLayer(under),
		lipgloss.NewLayer(box).X(x).Y(y).Z(1),
	)
	canvas.Compose(comp)
	return canvas.Render()
}

func (p Popup) box() string {
	title := p.titleStyle.Render(p.Title)

	btns := make([]string, len(p.Buttons))
	for i, b := range p.Buttons {
		label := " " + b + " "
		if i == p.cursor {
			btns[i] = render.Primary.Bold(true).Render("[" + label + "]")
		} else {
			btns[i] = render.Subtle.Render("[" + label + "]")
		}
	}

	content := title
	if p.Body != "" {
		content += "\n\n" + p.Body
	}
	if len(btns) > 0 {
		content += "\n\n" + strings.Join(btns, "  ")
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(render.ColorBlue).
		Padding(1, 2).
		Render(content)
}

func isEnter(key tea.Key) bool {
	return key.Code == tea.KeyEnter || key.Code == tea.KeyReturn || key.Code == tea.KeyKpEnter
}
