package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/kesonglab/video-interpolate/internal/probe"
	"github.com/kesonglab/video-interpolate/internal/render"
	"github.com/kesonglab/video-interpolate/internal/tui/components"
)

// addedFile is one confirmed source video plus its probed preview line.
type addedFile struct {
	Path  string
	Info  string // "1920x1080 · 24fps · 00:02:15"
	Found bool
}

// filePickerPage collects source files from a text input, drag-and-drop lines
// or the clipboard, then hands off to the multiplier screen.
type filePickerPage struct {
	ctx    *SharedContext
	km     keymap
	ti     textinput.Model
	files  []addedFile
	notice string
}

func NewFilePickerPage(ctx *SharedContext) Page {
	ti := textinput.New()
	ti.Placeholder = "path or directory..."
	ti.CharLimit = 4096
	ti.Prompt = "▸ "
	ti.Focus()
	return &filePickerPage{ctx: ctx, km: defaultKeymap(), ti: ti}
}

func (p *filePickerPage) Init() tea.Cmd {
	return p.ti.Focus()
}

func (p *filePickerPage) Update(msg tea.Msg) (Page, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.Key()
		switch {
		case p.km.matches(msg, p.km.Esc):
			p.ctx.Files = nil
			p.ctx.FileInfo = nil
			return p, Goto(pageWelcome)
		case p.km.matches(msg, p.km.Enter):
			added, notice := p.addFromText(p.ti.Value())
			p.files = append(p.files, added...)
			p.notice = notice
			p.ti.SetValue("")
			// enter with an empty box and files already present → advance
			if p.ti.Value() == "" && len(p.files) > 0 {
				p.syncCtx()
				return p, Goto(pageMultiplier)
			}
		case p.km.matches(msg, p.km.Paste) && p.ti.Value() == "":
			return p, p.paste()
		case p.km.matches(msg, p.km.Back) && p.ti.Value() == "":
			if n := len(p.files); n > 0 {
				p.files = p.files[:n-1]
			}
		default:
			if key.Code == 'v' && key.Mod == tea.ModCtrl {
				return p, p.paste()
			}
		}
	case tea.PasteMsg:
		return p, p.paste()
	case clipboardMsg:
		for _, line := range msg.lines {
			added, _ := p.addFromText(line)
			p.files = append(p.files, added...)
		}
		p.ti.SetValue("")
		return p, nil
	}

	ti, cmd := p.ti.Update(msg)
	p.ti = ti
	return p, cmd
}

func (p *filePickerPage) paste() tea.Cmd {
	return func() tea.Msg { return readClipboard() }
}

func (p *filePickerPage) addFromText(text string) ([]addedFile, string) {
	var out []addedFile
	for _, line := range expandInput(text) {
		for _, path := range expandPath(line) {
			info := probeVideo(path)
			if info == "" {
				continue
			}
			out = append(out, addedFile{Path: path, Info: info, Found: true})
		}
		if len(out) == 0 {
			return nil, render.Warn.Render("⚠ not found: " + line)
		}
	}
	return out, ""
}

func (p *filePickerPage) syncCtx() {
	seen := map[string]bool{}
	for _, f := range p.files {
		if !seen[f.Path] {
			seen[f.Path] = true
			p.ctx.Files = append(p.ctx.Files, f.Path)
			p.ctx.FileInfo = append(p.ctx.FileInfo, f.Info)
		}
	}
}

func (p *filePickerPage) View() tea.View {
	width := p.ctx.Width
	if width < 60 {
		width = 80
	}
	p.ti.SetWidth(width - 4)

	var lines []string
	if len(p.files) == 0 {
		lines = append(lines, render.Subtle.Render("no files yet — drag, type or paste a path"))
	}
	for _, f := range p.files {
		mark := render.Subtle.Render(render.IconRadioOff)
		if f.Found {
			mark = render.OK.Render(render.IconSuccess)
		}
		lines = append(lines, "  "+mark+" "+filepath.Base(f.Path)+"  "+render.Subtle.Render(f.Info))
	}
	listCard := components.Card(render.IconSource, "Sources", lines, width)

	body := lipgloss.JoinVertical(lipgloss.Left,
		components.Header("FilePicker", render.IconSystem, p.hardwareText(), render.Primary, "", width),
		components.Separator(width),
		"",
		p.ti.View(),
		"",
		listCard,
		"",
	)

	if p.notice != "" {
		body += "\n" + p.notice
	}
	body += "\n\n" + components.KeyHint([]components.HintPair{
		{Key: "enter", Desc: "add / next"},
		{Key: "p", Desc: "paste"},
		{Key: "backspace", Desc: "remove"},
		{Key: "esc", Desc: "back"},
	})

	return tea.NewView(body)
}

func (p *filePickerPage) hardwareText() string {
	if p.ctx.Caps != nil && p.ctx.Caps.GPUName != "" {
		return p.ctx.Caps.GPUName
	}
	return "VideoToolbox"
}

// expandInput splits a drag line on spaces, rejoining escaped `\ ` pairs.
func expandInput(line string) []string {
	line = strings.TrimSpace(line)
	if line == "" {
		return nil
	}
	if !strings.Contains(line, `\ `) {
		return []string{line}
	}
	var out []string
	for _, part := range strings.Fields(line) {
		out = append(out, strings.ReplaceAll(part, `\ `, " "))
	}
	return out
}

// expandPath turns a single path (file or directory) into matching videos.
func expandPath(path string) []string {
	if fi, err := os.Stat(path); err == nil && !fi.IsDir() {
		return []string{path}
	}
	if fi, err := os.Stat(path); err == nil && fi.IsDir() {
		var out []string
		entries, _ := os.ReadDir(path)
		for _, e := range entries {
			if !e.IsDir() && videoExt(e.Name()) {
				out = append(out, filepath.Join(path, e.Name()))
			}
		}
		return out
	}
	return nil
}

func videoExt(name string) bool {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".mp4", ".mkv", ".mov", ".avi", ".webm", ".flv", ".ts", ".m4v", ".wmv":
		return true
	}
	return false
}

// probeVideo returns a "WxH · fps · dur" summary, or "" if it can't probe.
func probeVideo(path string) string {
	info, err := probe.Probe(context.Background(), path)
	if err != nil || info.FPS <= 0 {
		return ""
	}
	return fmt.Sprintf("%dx%d · %.0ffps · %s", info.Width, info.Height, info.FPS, fmtDur(info.Duration))
}

func fmtDur(d time.Duration) string {
	if d <= 0 {
		return "00:00"
	}
	d = d.Round(time.Second)
	if d >= time.Hour {
		return fmt.Sprintf("%02d:%02d:%02d", int(d.Hours()), int(d.Minutes())%60, int(d.Seconds())%60)
	}
	return fmt.Sprintf("%02d:%02d", int(d.Minutes()), int(d.Seconds())%60)
}

// clipboardMsg carries pasted lines from the clipboard command.
type clipboardMsg struct{ lines []string }

// readClipboard returns the parsed clipboard as a clipboardMsg.
func readClipboard() clipboardMsg {
	out, err := exec.Command("pbpaste").Output()
	if err != nil {
		return clipboardMsg{}
	}
	var lines []string
	for _, l := range strings.Split(string(out), "\n") {
		if s := strings.TrimSpace(l); s != "" {
			lines = append(lines, s)
		}
	}
	return clipboardMsg{lines: lines}
}
