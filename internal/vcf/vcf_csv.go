package vcf

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"
)

type CSVConfig struct {
	// Separator used when joining multiple values into a single CSV cell.
	// Default is "; " to match existing CLI output.
	MultiValueSeparator string
}

func ConvertVCFToCSV(r io.Reader, w io.Writer, cfg CSVConfig) (*ParseResult, error) {
	if cfg.MultiValueSeparator == "" {
		cfg.MultiValueSeparator = "; "
	}

	res, err := ParseVCF(r)
	if err != nil {
		return nil, err
	}

	cw := csv.NewWriter(w)
	defer cw.Flush()

	if err := cw.Write([]string{"name", "phone", "email", "org"}); err != nil {
		return nil, fmt.Errorf("write csv header: %w", err)
	}

	for _, c := range res.Contacts {
		if err := cw.Write([]string{
			c.Name,
			strings.Join(c.Phones, cfg.MultiValueSeparator),
			strings.Join(c.Emails, cfg.MultiValueSeparator),
			c.Organization,
		}); err != nil {
			return nil, fmt.Errorf("write csv row: %w", err)
		}
	}

	cw.Flush()
	if err := cw.Error(); err != nil {
		return nil, fmt.Errorf("flush csv: %w", err)
	}
	return res, nil
}
