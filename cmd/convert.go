package cmd

import (
	"fmt"

	"github.com/init0-lux/vcf-toolkit/internal/convert"
	"github.com/spf13/cobra"
)

var convertCmd = &cobra.Command{
	Use:   "convert [file]",
	Short: "Convert CSV contact data to VCF format",
	Long: `Convert CSV contact data to vCard 4.0 (VCF) format.

Reads a CSV file with contact data and outputs a .vcf file
importable into mobile devices and address book applications.

Automatic header detection supports common column names for
name, phone, email, and organization fields.

Examples:
  vcf-toolkit convert contacts.csv
  cat contacts.csv | vcf-toolkit convert > contacts.vcf
  vcf-toolkit convert contacts.csv --out contacts.vcf
  vcf-toolkit convert contacts.csv --out contacts.vcf --dedupe
  vcf-toolkit convert contacts.csv --verbose
`,
	Args: cobra.MaximumNArgs(1),
	RunE: runConvert,
}

func init() {
	rootCmd.AddCommand(convertCmd)
	convertCmd.Flags().BoolVarP(&dedupeFlag, "dedupe", "d", false, "deduplicate contacts during conversion")
}

func runConvert(cmd *cobra.Command, args []string) error {
	r, err := openInput(args)
	if err != nil {
		return err
	}
	defer r.Close()

	w, err := openOutput()
	if err != nil {
		return err
	}
	defer w.Close()

	summary, err := convert.ConvertCSVToVCF(r, w, convert.Config{
		Deduplicate: dedupeFlag,
		Verbose:     verbose,
		Stderr:      cmd.ErrOrStderr(),
	})
	if err != nil {
		return err
	}

	if verbose {
		logSummary(cmd, summary)
	}
	return nil
}

func logSummary(cmd *cobra.Command, s *convert.ConversionSummary) {
	e := cmd.ErrOrStderr()
	fmt.Fprintf(e, "\nConversion Summary:\n")
	fmt.Fprintf(e, "  Input contacts:      %d\n", s.InputContacts)
	fmt.Fprintf(e, "  Output vCards:       %d\n", s.OutputVCards)
	fmt.Fprintf(e, "  Duplicates removed:  %d\n", s.DuplicatesRemoved)
	fmt.Fprintf(e, "  Parse errors:        %d\n", len(s.ParseErrors))
	for _, pe := range s.ParseErrors {
		fmt.Fprintf(e, "    row %d: %s\n", pe.Row, pe.Message)
	}
}
