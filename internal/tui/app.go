package tui

import (
	"context"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/kesonglab/video-interpolate/internal/config"
	"github.com/kesonglab/video-interpolate/internal/pipeline"
	"github.com/kesonglab/video-interpolate/internal/system"
)

// pageID enumerates the screens.
type pageID int

const (
	pageWelcome pageID = iota
	pageFiles
	pageMultiplier
	pageEncoder
	pageConfirm
	pageProcessing
	pageSummary
)

// StartFunc kicks off the pipeline once Confirm is accepted. Optional override
// for tests; when nil the app runs the real orchestrator.
type StartFunc func(ctx *SharedContext) tea.Cmd

// App is the top-level model owning the page stack, the mascot clock and the
// shared context.
type App struct {
	page        pageID
	pages       map[pageID]Page
	ctx         *SharedContext
	km          keymap
	quit        bool
	mascotFrame int
	width       int
	height      int
	Start       StartFunc
}

func New(cfg *config.Config, caps *system.Capabilities) *App {
	ctx := NewContext(cfg, caps)
	a := &App{ctx: ctx, km: defaultKeymap()}
	a.buildPages()
	return a
}

// buildPages (re)creates every page against the current context.
func (a *App) buildPages() {
	a.pages = map[pageID]Page{
		pageWelcome:    NewWelcomePage(a.ctx),
		pageFiles:      NewFilePickerPage(a.ctx),
		pageMultiplier: NewMultiplierPage(a.ctx),
		pageEncoder:    NewEncoderPage(a.ctx),
		pageConfirm:    NewConfirmPage(a.ctx),
		pageProcessing: NewProcessingPage(a.ctx),
		pageSummary:    NewSummaryPage(a.ctx),
	}
}

func (a *App) Init() tea.Cmd {
	return tea.Batch(a.pages[a.page].Init(), mascotTick())
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Global handlers first: quit, resize, mascot clock.
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.width, a.height = msg.Width, msg.Height
		a.ctx.Width, a.ctx.Height = msg.Width, msg.Height
		return a, nil
	case tea.QuitMsg:
		a.quit = true
		return a, tea.Quit
	case tickMsg:
		a.mascotFrame = (a.mascotFrame + 1) % len(reelyFrames)
		a.ctx.MascotFrame = a.mascotFrame
		return a, mascotTick()
	case tea.KeyMsg:
		// Processing owns `q` so it can ask before quitting.
		if a.page != pageProcessing && a.km.matches(msg, a.km.Quit) {
			a.quit = true
			return a, tea.Quit
		}
	case startProcessingMsg:
		if a.Start != nil {
			cmd := a.Start(a.ctx)
			a.page = pageProcessing
			return a, tea.Batch(cmd, a.pages[pageProcessing].Init())
		}
		a.startPipeline()
		a.page = pageProcessing
		return a, a.pages[pageProcessing].Init()
	case batchDoneMsg:
		a.ctx.Summary = msg.Summary
		a.pages[pageSummary] = NewSummaryPage(a.ctx)
		a.page = pageSummary
		return a, a.pages[pageSummary].Init()
	case resetMsg:
		a.reset()
		return a, a.pages[pageWelcome].Init()
	}

	// Page-local update.
	p, cmd := a.pages[a.page].Update(msg)
	a.pages[a.page] = p
	if cmd != nil {
		cmds = append(cmds, cmd)
	}

	// Handle transitions the page requested.
	switch msg := msg.(type) {
	case gotoMsg:
		a.page = msg.to
		cmds = append(cmds, a.pages[a.page].Init())
	}

	return a, tea.Batch(cmds...)
}

// startPipeline builds a config snapshot, launches the orchestrator on a
// goroutine and exposes its event channel to the processing page.
func (a *App) startPipeline() {
	if a.ctx.EventsCh != nil {
		return
	}
	cfg := *a.ctx.Cfg
	cfg.Multiplier = a.ctx.Multiplier
	cfg.Encoder = a.ctx.Encoder
	cfg.Quality = a.ctx.Quality
	cfg.OutputDir = a.ctx.OutputDir

	events := make(chan pipeline.Event, 256)
	a.ctx.EventsCh = events

	orch := pipeline.New(&cfg, a.ctx.Caps, events)
	orch.Enqueue(a.ctx.Files)
	a.ctx.Orch = orch

	runCtx, cancel := context.WithCancel(context.Background())
	a.ctx.CancelRun = cancel
	go func() {
		defer close(events)
		_ = orch.Run(runCtx)
	}()
}

// reset clears the run context and rebuilds every page for a fresh start.
func (a *App) reset() {
	a.ctx.Files = nil
	a.ctx.FileInfo = nil
	a.ctx.SourceFPS = 0
	a.ctx.EventsCh = nil
	a.ctx.Orch = nil
	a.ctx.CancelRun = nil
	a.ctx.Summary = BatchSummary{}
	a.page = pageWelcome
	a.buildPages()
}

func (a *App) View() tea.View {
	v := tea.NewView(a.pages[a.page].View().Content)
	v.AltScreen = true
	return v
}

// tickMsg drives the mascot animation.
type tickMsg time.Time

func mascotTick() tea.Cmd {
	return tea.Tick(time.Millisecond*200, func(t time.Time) tea.Msg { return tickMsg(t) })
}

// gotoMsg switches pages; pages return it via Goto.
type gotoMsg struct{ to pageID }

func Goto(to pageID) tea.Cmd {
	return func() tea.Msg { return gotoMsg{to} }
}

// resetMsg asks the app to wipe the flow and start over.
type resetMsg struct{}

func ResetApp() tea.Cmd {
	return func() tea.Msg { return resetMsg{} }
}

func StartProcessing() tea.Cmd {
	return func() tea.Msg { return startProcessingMsg{} }
}
