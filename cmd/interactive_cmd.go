package cmd

import "github.com/spf13/cobra"

var interactiveCmd = &cobra.Command{
	Use:   "interactive",
	Short: "Run an interactive menu-driven workflow",
	Long: `Interactive mode provides a menu-driven UI for common workflows.

Note: The CLI is designed to be non-interactive by default for scripting.
`,
	Run: func(cmd *cobra.Command, args []string) {
		runInteractive()
	},
}

func init() {
	rootCmd.AddCommand(interactiveCmd)
}

