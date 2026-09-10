package cli

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/BurntSushi/toml"
	"github.com/kesonglab/video-interpolate/internal/config"
	"github.com/spf13/cobra"
)

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage configuration",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "path",
		Short: "Print the config file path",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(cmd.OutOrStdout(), config.DefaultPath())
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "edit",
		Short: "Open the config in $EDITOR",
		Run: func(cmd *cobra.Command, args []string) {
			p := config.DefaultPath()
			if _, err := os.Stat(p); os.IsNotExist(err) {
				if err := writeDefaultConfig(p); err != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "edit: %v\n", err)
					return
				}
			}
			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "vi"
			}
			ec := exec.Command(editor, p)
			ec.Stdin = os.Stdin
			ec.Stdout = os.Stdout
			ec.Stderr = os.Stderr
			if err := ec.Run(); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "edit: %v\n", err)
			}
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "reset",
		Short: "Write a fresh default config",
		Run: func(cmd *cobra.Command, args []string) {
			if err := writeDefaultConfig(config.DefaultPath()); err != nil {
				fmt.Fprintf(cmd.ErrOrStderr(), "reset: %v\n", err)
				return
			}
			fmt.Fprintln(cmd.OutOrStdout(), "wrote", config.DefaultPath())
		},
	})
	return cmd
}

func writeDefaultConfig(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(config.Default())
}
