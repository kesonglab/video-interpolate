package tui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/kesonglab/video-interpolate/internal/render"
	"github.com/kesonglab/video-interpolate/internal/tui/components"
)

type summaryState struct {
	pageID
	ctx        *SharedContext
	summary    BatchSummary
	showMascot bool
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
	if width < 40 {
		width = 80
	}

	stats := []string{
		fmt.Sprintf("%s %-14s %d", render.OK.Render(render.IconSuccess), "Success", p.summary.Success),
		fmt.Sprintf("%s %-14s %d", render.Danger.Render(render.IconFailed), "Failed", p.summary.Failed),
		fmt.Sprintf("%s %-14s %d", render.Warn.Render(render.IconSkipped), "Skipped", p.summary.Skipped),
		fmt.Sprintf("%s %s", render.Subtle.Render("Total time    "), formatDuration(p.summary.Elapsed)),
	}

	fileLines := []string{render.Subtle.Render("Output files:"), ""}
	if len(p.summary.Outputs) == 0 {
		fileLines = append(fileLines, render.Subtle.Render("  (none)"))
	}
	for _, o := range p.summary.Outputs {
		fileLines = append(fileLines, p.outputLine(o))
	}

	body := lipgloss.JoinVertical(lipgloss.Left,
		components.Header("Interpolate  Summary", "●", "done", render.OK, p.hardwareText(), width),
		components.Separator(width),
		"",
		strings.Join(stats, "\n"),
		"",
		components.SubSeparator(width),
		"",
		strings.Join(fileLines, "\n"),
		"",
		components.Separator(width),
		components.KeyHint([]components.HintPair{
			{Key: "enter", Desc: "open output folder"},
			{Key: "c", Desc: "copy list"},
			{Key: "r", Desc: "restart"},
			{Key: "q", Desc: "quit"},
		}),
	)

	return tea.NewView(body)
}

func (p *summaryState) outputLine(o JobOutput) string {
	path := o.Path
	if path == "" {
		path = "(unknown)"
	}
	switch o.Status {
	case "done":
		meta := formatDuration(o.Duration)
		if o.Size > 0 {
			meta = humanBytes(o.Size) + " · " + meta
		}
		return fmt.Sprintf("%s %s  %s", render.OK.Render(render.IconSuccess), path, render.Subtle.Render(meta))
	case "failed":
		detail := "failed"
		if o.Error != "" {
			detail = "failed: " + o.Error
		}
		return fmt.Sprintf("%s %s  %s", render.Danger.Render(render.IconFailed), path, render.Danger.Render(detail))
	default:
		return fmt.Sprintf("%s %s  %s", render.Warn.Render(render.IconSkipped), path, render.Subtle.Render("skipped"))
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

func (p *summaryState) hardwareText() string {
	hw := "VideoToolbox"
	if p.ctx.Caps != nil && p.ctx.Caps.GPUName != "" {
		hw = p.ctx.Caps.GPUName
	}
	if p.ctx.Encoder != "" {
		return hw + " · " + encoderLabel(p.ctx.Encoder)
	}
	return hw
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
