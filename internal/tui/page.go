package tui

import (
	"context"

	tea "charm.land/bubbletea/v2"
	"github.com/kesonglab/video-interpolate/internal/config"
	"github.com/kesonglab/video-interpolate/internal/pipeline"
	"github.com/kesonglab/video-interpolate/internal/system"
)

// Page is the per-screen contract. Update returns the next page and any cmds;
// a page returns itself to stay put.
type Page interface {
	Init() tea.Cmd
	Update(msg tea.Msg) (Page, tea.Cmd)
	View() tea.View
}

// SharedContext is threaded through every page and carries the running
// config plus whatever earlier screens have collected.
type SharedContext struct {
	Cfg        *config.Config
	Caps       *system.Capabilities
	Width      int
	Height     int
	Files      []string // populated by FilePicker
	FileInfo   []string // probed "1920x1080 · 24fps · 00:02:15" per file
	Multiplier int      // 2 / 4 / 8
	Encoder    string
	Quality    string // quality/balanced/speed
	OutputDir  string
	SourceFPS  float64 // first source fps, set once probed

	// Phase 2b: live pipeline wiring.
	EventsCh    chan pipeline.Event
	Orch        *pipeline.Orchestrator
	CancelRun   context.CancelFunc
	Summary     BatchSummary
	MascotFrame int
}

// NewContext seeds defaults from the loaded config.
func NewContext(cfg *config.Config, caps *system.Capabilities) *SharedContext {
	if caps == nil {
		caps = system.DetectCapabilities()
	}
	return &SharedContext{
		Cfg:        cfg,
		Caps:       caps,
		Multiplier: cfg.Multiplier,
		Encoder:    cfg.Encoder,
		Quality:    cfg.Quality,
		OutputDir:  cfg.OutputDir,
	}
}
