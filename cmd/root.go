// Package cmd implements the CLI interface for vcf-toolkit.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	outputFile string
	jsonOutput bool
	dryRun     bool
	verbose    bool
	dedupeFlag bool
)

var rootCmd = &cobra.Command{
	Use:   "vcf-toolkit",
	Short: "A developer-first toolkit for processing, normalizing, deduplicating, and converting contact data",
	Long: `vcf-toolkit is a developer-first toolkit for processing, normalizing,
deduplicating, and converting contact data.

It operates as a Go SDK and CLI for transforming inconsistent, fragmented
contact datasets into clean, standardized, and deduplicated outputs.

Documentation: https://github.com/init0-lux/vcf-toolkit`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVarP(&outputFile, "out", "o", "", "output file (default: stdout)")
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "verbose output")
}
