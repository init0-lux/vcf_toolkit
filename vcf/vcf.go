package vcf

import (
	"io"

	internal "github.com/init0/vcf-toolkit/internal/vcf"
	"github.com/init0/vcf-toolkit/model"
)

type ParseResult = internal.ParseResult
type CSVConfig = internal.CSVConfig

func ParseVCF(r io.Reader) (*ParseResult, error) { return internal.ParseVCF(r) }

func ConvertVCFToCSV(r io.Reader, w io.Writer, cfg CSVConfig) (*ParseResult, error) {
	return internal.ConvertVCFToCSV(r, w, cfg)
}

// Convenience alias for SDK users.
type Contact = model.Contact
