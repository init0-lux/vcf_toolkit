package convert

import (
	"io"

	internal "github.com/init0/vcf-toolkit/internal/convert"
	"github.com/init0/vcf-toolkit/model"
)

// Public SDK wrapper around the internal implementation.

type Config = internal.Config
type ParseResult = internal.ParseResult
type ConversionSummary = internal.ConversionSummary

const (
	FieldName  = internal.FieldName
	FieldPhone = internal.FieldPhone
	FieldEmail = internal.FieldEmail
	FieldOrg   = internal.FieldOrg
)

var ContactToVCF = internal.ContactToVCF

func ParseCSV(r io.Reader, cfg Config) (*ParseResult, error) { return internal.ParseCSV(r, cfg) }

func ConvertCSVToVCF(r io.Reader, w io.Writer, cfg Config) (*ConversionSummary, error) {
	return internal.ConvertCSVToVCF(r, w, cfg)
}

// Re-export for convenience.
type Contact = model.Contact

