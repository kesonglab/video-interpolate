package cli

import (
	"fmt"

	"github.com/kesonglab/video-interpolate/internal/render"
	"github.com/kesonglab/video-interpolate/internal/system"
	"github.com/spf13/cobra"
)

func newDoctorCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check environment dependencies",
		RunE: func(cmd *cobra.Command, args []string) error {
			deps := system.CheckDependencies()
			caps := system.DetectCapabilities()

			fmt.Fprintln(cmd.OutOrStdout(), "Dependencies:")
			for _, d := range deps {
				mark := "✗"
				style := render.Danger
				if d.OK {
					mark = "✓"
					style = render.OK
				}
				fmt.Fprintf(cmd.OutOrStdout(), "  %s %s", style.Render(mark), d.Name)
				if d.Path != "" {
					fmt.Fprintf(cmd.OutOrStdout(), " %s", render.Subtle.Render(d.Path))
				}
				if !d.OK && d.FixHint != "" {
					fmt.Fprintf(cmd.OutOrStdout(), "\n    %s", render.Warn.Render(d.FixHint))
				}
				fmt.Fprintln(cmd.OutOrStdout())
			}

			fmt.Fprintln(cmd.OutOrStdout())
			fmt.Fprintln(cmd.OutOrStdout(), "Hardware:")
			if caps.HasVideoToolbox {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s VideoToolbox\n", render.OK.Render("✓"))
			} else {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s VideoToolbox (fallback to software)\n", render.Subtle.Render("○"))
			}
			if caps.HasMetal {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s Metal\n", render.OK.Render("✓"))
			}
			if caps.GPUName != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s GPU: %s\n", render.Subtle.Render("·"), caps.GPUName)
			}
			if caps.BestEncoder != "" {
				fmt.Fprintf(cmd.OutOrStdout(), "  %s Best encoder: %s\n", render.Subtle.Render("·"), caps.BestEncoder)
			}
			return nil
		},
	}
	return cmd
}
