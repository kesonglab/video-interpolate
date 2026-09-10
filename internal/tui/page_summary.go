package tui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/kesonglab/video-interpolate/internal/render"
	"github.com/kesonglab/video-interpolate/internal/tui/components"
)

type summaryState struct {
	pageID
	ctx     *SharedContext
	summary BatchSummary
}

func NewSummaryPage(ctx *SharedContext) Page {
	return &summaryState{ctx: ctx, summary: ctx.Summary}
}

func (p *summaryState) Init() tea.Cmd { return nil }

func (p *summaryState) Update(msg tea.Msg) (Page, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.Key()
		switch {
		case key.Text == "r":
			return p, ResetApp()
		case key.Text == "c":
			return p, copyOutputList(p.paths())
		case key.Code == tea.KeyEsc:
			return p, tea.Quit
		case isEnter(key):
			return p, openOutputDir(p.ctx.OutputDir)
		}
	}
	return p, nil
}

func (p *summaryState) View() tea.View {
	width := p.ctx.Width
	if width < 60 {
		width = 80
	}

	var b strings.Builder
	b.WriteString(components.Banner(appTitle(), authorLine, width))
	b.WriteString("\n\n")
	b.WriteString(render.SectionTitle("Download Results"))
	b.WriteString("\n\n")
	b.WriteString("  " + render.Green.Render("✓") + " Success   " + render.Green.Render(fmt.Sprint(p.summary.Success)))
	b.WriteString("\n")
	b.WriteString("  " + render.Red.Render("✗") + " Failed    " + render.Red.Render(fmt.Sprint(p.summary.Failed)))
	if p.summary.Skipped > 0 {
		b.WriteString("\n  " + render.Yellow.Render("⚠") + " Skipped   " + render.Yellow.Render(fmt.Sprint(p.summary.Skipped)))
	}
	b.WriteString("\n\n")
	if len(p.summary.Outputs) == 0 {
		b.WriteString("  " + render.Dim.Render("(none)") + "\n")
	}
	for _, o := range p.summary.Outputs {
		b.WriteString(p.outputLine(o))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(components.KeyHint([]components.HintPair{
		{Key: "enter", Desc: "open output folder"},
		{Key: "c", Desc: "copy list"},
		{Key: "r", Desc: "restart"},
		{Key: "q", Desc: "quit"},
	}))

	return tea.NewView(b.String())
}

func (p *summaryState) outputLine(o JobOutput) string {
	path := shortPath(o.Path)
	if o.Path == "" {
		path = "(unknown)"
	}
	switch o.Status {
	case "done":
		meta := formatDuration(o.Duration)
		if o.Size > 0 {
			meta = humanBytes(o.Size) + " · " + meta
		}
		return "  " + render.Green.Render("✓") + " " + path + "  " + render.Dim.Render(meta)
	case "failed":
		detail := "failed"
		if o.Error != "" {
			detail = "failed: " + o.Error
		}
		return "  " + render.Red.Render("✗") + " " + path + "  " + render.Red.Render(detail)
	default:
		return "  " + render.Yellow.Render("⚠") + " " + path + "  " + render.Dim.Render("skipped")
	}
}

func (p *summaryState) paths() []string {
	out := make([]string, 0, len(p.summary.Outputs))
	for _, o := range p.summary.Outputs {
		if o.Path != "" {
			out = append(out, o.Path)
		}
	}
	return out
}

func openOutputDir(dir string) tea.Cmd {
	return func() tea.Msg {
		if dir == "" {
			return nil
		}
		opener := "open"
		if runtime.GOOS == "linux" {
			opener = "xdg-open"
		}
		_ = exec.Command(opener, expandHomeDir(dir)).Start()
		return nil
	}
}

func copyOutputList(paths []string) tea.Cmd {
	return func() tea.Msg {
		c := exec.Command("pbcopy")
		c.Stdin = strings.NewReader(strings.Join(paths, "\n"))
		_ = c.Run()
		return nil
	}
}
