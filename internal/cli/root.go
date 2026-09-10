package cli

import (
	"os"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"
	"github.com/kesonglab/video-interpolate/internal/config"
	"github.com/kesonglab/video-interpolate/internal/system"
	"github.com/kesonglab/video-interpolate/internal/tui"
	"github.com/kesonglab/video-interpolate/internal/version"
	"github.com/spf13/cobra"
)

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:     "vif",
		Short:   "AI video frame interpolation TUI",
		Long:    `vif interpolates video frames using RIFE for smoother playback.`,
		Version: version.Full(),
	}
	root.AddCommand(newRunCmd())
	root.AddCommand(newDoctorCmd())
	root.AddCommand(newConfigCmd())
	root.AddCommand(newUpdateCmd())
	root.AddCommand(newVersionCmd())
	root.PersistentFlags().Bool("no-tui", false, "skip the interactive TUI and use the CLI")
	return root
}

// Execute runs the TUI when invoked bare on a terminal, otherwise the cobra
// CLI (help, subcommands, or `--no-tui`).
func Execute() error {
	if len(os.Args) == 1 && term.IsTerminal(os.Stdin.Fd()) && term.IsTerminal(os.Stdout.Fd()) {
		return runTUI()
	}
	return NewRootCmd().Execute()
}

func runTUI() error {
	cfg, err := config.Load(config.DefaultPath())
	if err != nil {
		return err
	}
	caps := system.DetectCapabilities()
	app := tui.New(cfg, caps)
	p := tea.NewProgram(app)
	_, err = p.Run()
	return err
}
