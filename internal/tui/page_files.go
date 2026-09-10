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
	ctx     *SharedContext
	km      keymap
	ti      textinput.Model
	files   []addedFile
	cursor  int
	probeFn func(string) string // probe override for tests
}

func NewFilePickerPage(ctx *SharedContext) Page {
	ti := textinput.New()
	ti.Placeholder = "path or directory..."
	ti.CharLimit = 4096
	ti.Prompt = "▸ "
	ti.Focus()
	return &filePickerPage{ctx: ctx, km: defaultKeymap(), ti: ti, probeFn: probeVideo}
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
			added, missed := p.addFromText(p.ti.Value())
			p.notify(added, len(missed))
			p.ti.SetValue("")
			// enter with an empty box and files already present → advance
			if p.ti.Value() == "" && len(p.files) > 0 {
				p.syncCtx()
				return p, Goto(pageMultiplier)
			}
		case p.km.matches(msg, p.km.Up):
			if p.cursor > 0 {
				p.cursor--
			}
		case p.km.matches(msg, p.km.Down):
			if p.cursor < len(p.files)-1 {
				p.cursor++
			}
		case p.km.matches(msg, p.km.Paste) && p.ti.Value() == "":
			return p, p.paste()
		case p.km.matches(msg, p.km.Back) && p.ti.Value() == "":
			p.removeCursor()
		default:
			if key.Code == 'v' && key.Mod == tea.ModCtrl {
				return p, p.paste()
			}
		}
	case tea.PasteMsg:
		// drag-drop / Cmd+V arrives here as one bracketed-paste payload
		p.ingest(msg.Content)
		return p, nil
	case clipboardMsg:
		p.ingest(strings.Join(msg.lines, "\n"))
		return p, nil
	}

	ti, cmd := p.ti.Update(msg)
	p.ti = ti
	return p, cmd
}

// removeCursor drops the highlighted file and clamps the cursor.
func (p *filePickerPage) removeCursor() {
	if len(p.files) == 0 {
		return
	}
	p.files = append(p.files[:p.cursor], p.files[p.cursor+1:]...)
	if p.cursor >= len(p.files) && p.cursor > 0 {
		p.cursor--
	}
}

func (p *filePickerPage) paste() tea.Cmd {
	return func() tea.Msg { return readClipboard() }
}

// ingest parses pasted/dropped text and adds whatever resolves to video.
func (p *filePickerPage) ingest(text string) {
	added, missed := p.addFromText(text)
	p.notify(added, len(missed))
}

// notify surfaces add/miss counts as an app-level toast.
func (p *filePickerPage) notify(added, missed int) {
	switch {
	case added > 0 && missed > 0:
		p.ctx.AddToast(fmt.Sprintf("已加入 %d 个，%d 个未识别", added, missed))
	case missed > 0:
		p.ctx.AddToast(fmt.Sprintf("%d 个路径未识别", missed))
	case added > 0:
		p.ctx.AddToast(fmt.Sprintf("已加入 %d 个文件", added))
	}
}

// addFromText ingests typed or pasted text: each line is tried whole first
// (paths with plain spaces), then shell-split (escapes/quotes/multi-file).
func (p *filePickerPage) addFromText(text string) (added int, missed []string) {
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if n := p.tryAddPath(line); n > 0 {
			added += n
			continue
		}
		for _, tok := range parsePastedPaths(line) {
			if n := p.tryAddPath(tok); n > 0 {
				added += n
			} else {
				missed = append(missed, tok)
			}
		}
	}
	if added > 0 {
		p.ti.SetValue("")
	}
	return added, missed
}

// tryAddPath resolves one path (file or directory) and appends its videos.
func (p *filePickerPage) tryAddPath(path string) int {
	fi, err := os.Stat(path)
	if err != nil {
		return 0
	}
	var candidates []string
	switch {
	case fi.IsDir():
		candidates = expandPath(path)
	case isVideoExt(path):
		candidates = []string{path}
	default:
		return 0
	}
	n := 0
	for _, c := range candidates {
		if p.hasFile(c) {
			n++ // already queued, not a miss
			continue
		}
		info := p.probe(c)
		if info == "" {
			continue
		}
		p.files = append(p.files, addedFile{Path: c, Info: info, Found: true})
		n++
	}
	return n
}

func (p *filePickerPage) hasFile(path string) bool {
	for _, f := range p.files {
		if f.Path == path {
			return true
		}
	}
	return false
}

// probe runs the injected prober, falling back to the real one.
func (p *filePickerPage) probe(path string) string {
	if p.probeFn != nil {
		return p.probeFn(path)
	}
	return probeVideo(path)
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
	p.ti.SetWidth(width - 6)

	banner := components.Banner(appTitle(), authorLine, width)
	box := components.Box(p.ti.View(), width-2)

	var lines []string
	if len(p.files) == 0 {
		lines = append(lines, "  "+render.Dim.Render("no files yet — drag, type or paste a path"))
	}
	for i, f := range p.files {
		marker := "  "
		if i == p.cursor {
			marker = render.Green.Render("▶ ")
		}
		mark := render.Dim.Render(render.IconFailed)
		if f.Found {
			mark = render.Green.Render(render.IconDone)
		}
		lines = append(lines, marker+mark+" "+filepath.Base(f.Path)+"  "+render.Dim.Render(f.Info))
	}

	body := banner + "\n\n" + box + "\n\n" + strings.Join(lines, "\n")
	body += "\n\n" + components.KeyHint([]components.HintPair{
		{Key: "drag", Desc: "drop files"},
		{Key: "enter", Desc: "next"},
		{Key: "p", Desc: "paste"},
		{Key: "backspace", Desc: "remove"},
		{Key: "esc", Desc: "back"},
	})

	return tea.NewView(body)
}

// expandPath turns a single directory into its matching video files.
func expandPath(path string) []string {
	var out []string
	entries, _ := os.ReadDir(path)
	for _, e := range entries {
		if !e.IsDir() && isVideoExt(e.Name()) {
			out = append(out, filepath.Join(path, e.Name()))
		}
	}
	return out
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
