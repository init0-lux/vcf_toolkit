package convert

import (
	"fmt"
	"io"
	"strings"

	"github.com/init0/vcf-toolkit/internal/dedupe"
	"github.com/init0/vcf-toolkit/internal/model"
	"github.com/init0/vcf-toolkit/internal/normalize"
)

type NormalizeVCFConfig struct {
	// CSV parsing controls (same semantics as Config).
	DefaultCountry string
	HeaderMapping  map[string]string

	// Normalization controls.
	Name  normalize.NameConfig
	Phone normalize.PhoneConfig
	Email normalize.EmailConfig

	// If true, runs deduplication after normalization.
	Deduplicate bool

	Verbose bool
	Stderr  io.Writer
}

func (cfg NormalizeVCFConfig) logf(format string, args ...interface{}) {
	if cfg.Stderr != nil {
		fmt.Fprintf(cfg.Stderr, format+"\n", args...)
	}
}

// NormalizeCSVToVCF parses CSV contacts, normalizes their fields, optionally
// deduplicates, and writes vCard 4.0 output.
func NormalizeCSVToVCF(r io.Reader, w io.Writer, cfg NormalizeVCFConfig) (*ConversionSummary, error) {
	parseCfg := Config{
		DefaultCountry: cfg.DefaultCountry,
		HeaderMapping:  cfg.HeaderMapping,
		Deduplicate:    false, // dedupe happens after normalization
		Verbose:        cfg.Verbose,
		Stderr:         cfg.Stderr,
	}

	result, err := ParseCSV(r, parseCfg)
	if err != nil {
		return nil, fmt.Errorf("parsing CSV: %w", err)
	}

	contacts := make([]model.Contact, 0, len(result.Contacts))
	for _, c := range result.Contacts {
		contacts = append(contacts, normalizeContact(c, cfg))
	}

	if cfg.Deduplicate && len(contacts) > 0 {
		cfg.logf("Deduplicating %d contacts...", len(contacts))
		dc := dedupe.DefaultConfig()
		dedupeResult := dedupe.Deduplicate(contacts, dc)

		var final []model.Contact
		mergeIdx := 0
		for _, cluster := range dedupeResult.Clusters {
			if len(cluster) <= 1 {
				if len(cluster) == 1 {
					final = append(final, cluster[0])
				}
				continue
			}
			if mergeIdx < len(dedupeResult.Merged) {
				final = append(final, dedupeResult.Merged[mergeIdx].Contact)
				mergeIdx++
				continue
			}
			final = append(final, cluster[0])
		}
		contacts = final
		cfg.logf("After deduplication: %d contacts (removed %d duplicates)",
			len(contacts), len(result.Contacts)-len(contacts))
	}

	for _, c := range contacts {
		if _, err := io.WriteString(w, ContactToVCF(c)); err != nil {
			return nil, fmt.Errorf("writing VCF: %w", err)
		}
	}

	return &ConversionSummary{
		InputRows:         len(result.Contacts),
		OutputVCards:      len(contacts),
		ParseErrors:       result.Errors,
		Deduplicated:      cfg.Deduplicate,
		InputContacts:     len(result.Contacts),
		DuplicatesRemoved: len(result.Contacts) - len(contacts),
	}, nil
}

func normalizeContact(c model.Contact, cfg NormalizeVCFConfig) model.Contact {
	out := c

	// Name normalization may also provide organization hints.
	if strings.TrimSpace(out.Name) != "" {
		n := normalize.NormalizeName(out.Name, cfg.Name)
		if n.Normalized && n.FullName != "" {
			out.Name = n.FullName
		}
		if out.Organization == "" && n.Organization != "" {
			out.Organization = n.Organization
		}
	}

	// Phone normalization.
	out.Phones = nil
	for _, p := range c.Phones {
		np := normalize.NormalizePhone(p, cfg.Phone)
		if np.Valid && np.Normalized != "" {
			out.Phones = append(out.Phones, np.Normalized)
		} else if strings.TrimSpace(p) != "" {
			out.Phones = append(out.Phones, strings.TrimSpace(p))
		}
	}

	// Email normalization.
	out.Emails = nil
	for _, e := range c.Emails {
		ne := normalize.NormalizeEmail(e, cfg.Email)
		if ne.Valid && ne.Normalized != "" {
			out.Emails = append(out.Emails, ne.Normalized)
		} else if strings.TrimSpace(e) != "" {
			out.Emails = append(out.Emails, strings.TrimSpace(e))
		}
	}

	return out
}
