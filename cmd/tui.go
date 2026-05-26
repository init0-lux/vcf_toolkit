package cmd

import (
	"os"

	"github.com/init0-lux/vcf-toolkit/tui"
	"github.com/spf13/cobra"
)

var tuiCmd = &cobra.Command{
	Use:   "tui",
	Short: "Launch the interactive TUI",
	Long: `Launch the interactive terminal UI.

Use this when you want a guided, configurable workflow for normalization,
deduplication, and CSV->VCF conversion.
`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return tui.Run(os.Stdin, os.Stdout, cmd.ErrOrStderr())
	},
}

func init() {
	rootCmd.AddCommand(tuiCmd)
}
