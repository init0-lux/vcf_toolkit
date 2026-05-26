// Package convert implements CSV to VCF conversion per RFC 6350 (vCard 4.0).
package convert

import (
	"encoding/csv"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/init0-lux/vcf-toolkit/internal/dedupe"
	"github.com/init0-lux/vcf-toolkit/internal/model"
	"github.com/init0-lux/vcf-toolkit/internal/normalize"
)

type Config struct {
	DefaultCountry string
	HeaderMapping  map[string]string
	Deduplicate    bool
	Verbose        bool
	Stderr         io.Writer
}

const (
	FieldName  = "name"
	FieldPhone = "phone"
	FieldEmail = "email"
	FieldOrg   = "org"
)

func (cfg Config) logf(format string, args ...interface{}) {
	if cfg.Stderr != nil {
		fmt.Fprintf(cfg.Stderr, format+"\n", args...)
	}
}

type ParseResult struct {
	Contacts []model.Contact
	Errors   []model.ParseError
}

func ParseCSV(r io.Reader, cfg Config) (*ParseResult, error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1
	reader.ReuseRecord = false
	reader.LazyQuotes = true

	rawHeaders, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("reading CSV headers: %w", err)
	}

	headers := make([]string, len(rawHeaders))
	for i, h := range rawHeaders {
		headers[i] = strings.TrimSpace(h)
	}

	mapping := buildHeaderMapping(headers, cfg.HeaderMapping)

	if cfg.Verbose {
		cfg.logf("Detected header mapping:")
		for h, f := range mapping {
			cfg.logf("  %q -> %s", h, f)
		}
	}

	result := &ParseResult{}
	line := 2

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			result.Errors = append(result.Errors, model.ParseError{
				Row:     line,
				Message: fmt.Sprintf("read error: %v", err),
			})
			line++
			continue
		}

		c, errs := contactFromRow(record, headers, mapping)
		for i := range errs {
			errs[i].Row = line
		}
		if len(errs) > 0 {
			result.Errors = append(result.Errors, errs...)
		}

		if c.Name != "" || len(c.Phones) > 0 || len(c.Emails) > 0 || c.Organization != "" {
			c.Source.Row = line
			result.Contacts = append(result.Contacts, c)
		}

		line++
	}

	return result, nil
}

func contactFromRow(row []string, headers []string, mapping map[string]string) (model.Contact, []model.ParseError) {
	var c model.Contact
	var errs []model.ParseError

	for i, header := range headers {
		if i >= len(row) {
			continue
		}
		val := strings.TrimSpace(row[i])
		if val == "" {
			continue
		}

		field, ok := mapping[header]
		if !ok {
			continue
		}

		switch field {
		case FieldName:
			if c.Name == "" {
				c.Name = val
			}
		case FieldPhone:
			c.Phones = append(c.Phones, val)
		case FieldEmail:
			c.Emails = append(c.Emails, val)
		case FieldOrg:
			if c.Organization == "" {
				c.Organization = val
			}
		}
	}

	if c.Name == "" && len(c.Phones) == 0 && len(c.Emails) == 0 && c.Organization == "" {
		errs = append(errs, model.ParseError{
			Field:   "row",
			Message: "no recognizable contact fields in row",
		})
	}

	return c, errs
}

func buildHeaderMapping(headers []string, override map[string]string) map[string]string {
	mapping := make(map[string]string, len(headers))

	normalizedOverride := make(map[string]string, len(override))
	for k, v := range override {
		normalizedOverride[strings.ToLower(k)] = v
	}

	for _, header := range headers {
		lower := strings.ToLower(header)

		if field, ok := normalizedOverride[lower]; ok {
			mapping[header] = field
			continue
		}

		if field := detectField(lower); field != "" {
			mapping[header] = field
		}
	}

	return mapping
}

func detectField(header string) string {
	normalized := normalizeHeader(header)
	if normalized == "" {
		return ""
	}

	switch {
	case regexp.MustCompile(`^(name|fullname|full_name|full-name|contactname|displayname)$`).MatchString(normalized):
		return FieldName
	case regexp.MustCompile(`^(first_name|given_name|firstname|givenname)$`).MatchString(normalized):
		return FieldName
	case regexp.MustCompile(`^(last_name|family_name|lastname|familyname|surname)$`).MatchString(normalized):
		return FieldName

	case regexp.MustCompile(`^phone\d*$`).MatchString(normalized):
		return FieldPhone
	case regexp.MustCompile(`^(telephone|tel|mobile|cell|phone.number|phonenumber|contactnumber)$`).MatchString(normalized):
		return FieldPhone
	case regexp.MustCompile(`^phone_\d+$`).MatchString(normalized):
		return FieldPhone

	case regexp.MustCompile(`^email\d*$`).MatchString(normalized):
		return FieldEmail
	case regexp.MustCompile(`^(e_mail|email_address|emailaddress|mail)$`).MatchString(normalized):
		return FieldEmail
	case regexp.MustCompile(`^email_\d+$`).MatchString(normalized):
		return FieldEmail

	case regexp.MustCompile(`^(org|organization|organisation|company|employer|affiliation)$`).MatchString(normalized):
		return FieldOrg
	}

	return ""
}

func normalizeHeader(header string) string {
	re := regexp.MustCompile(`[ _\-]+`)
	return re.ReplaceAllString(header, "_")
}

func ContactToVCF(c model.Contact) string {
	var b strings.Builder

	b.WriteString("BEGIN:VCARD\r\n")
	b.WriteString("VERSION:4.0\r\n")
	b.WriteString("PRODID:-//vcf-toolkit//EN\r\n")

	fn := c.Name
	if fn == "" {
		fn = "Unknown"
	}
	b.WriteString(fmt.Sprintf("FN:%s\r\n", escapeVCF(fn)))
	b.WriteString(fmt.Sprintf("N:%s\r\n", structuredName(c.Name)))

	if c.Organization != "" {
		b.WriteString(fmt.Sprintf("ORG:%s\r\n", escapeVCF(c.Organization)))
	}

	for _, phone := range c.Phones {
		normalized := normalize.NormalizePhone(phone, normalize.PhoneConfig{})
		tel := normalized.Normalized
		if normalized.Valid && tel != "" {
			if !strings.HasPrefix(tel, "+") {
				tel = "+" + tel
			}
			b.WriteString(fmt.Sprintf("TEL;VALUE=uri:tel:%s\r\n", tel))
		} else if cleaned := normalize.PhoneDigits(phone); cleaned != "" {
			b.WriteString(fmt.Sprintf("TEL;VALUE=uri:tel:+%s\r\n", cleaned))
		} else {
			b.WriteString(fmt.Sprintf("TEL:%s\r\n", escapeVCF(phone)))
		}
	}

	for _, email := range c.Emails {
		normalized := normalize.NormalizeEmail(email, normalize.EmailConfig{
			StripAliases:     true,
			StrictValidation: false,
		})
		if normalized.Valid && normalized.Normalized != "" {
			b.WriteString(fmt.Sprintf("EMAIL:%s\r\n", normalized.Normalized))
		} else {
			b.WriteString(fmt.Sprintf("EMAIL:%s\r\n", escapeVCF(email)))
		}
	}

	b.WriteString("END:VCARD\r\n")
	return b.String()
}

func structuredName(fullName string) string {
	fullName = strings.TrimSpace(fullName)
	if fullName == "" {
		return ";;;;"
	}

	parts := strings.Fields(fullName)
	if len(parts) == 1 {
		return fmt.Sprintf(";%s;;;", escapeVCF(parts[0]))
	}

	given := escapeVCF(parts[0])
	family := escapeVCF(parts[len(parts)-1])

	var middle string
	if len(parts) > 2 {
		middleParts := parts[1 : len(parts)-1]
		middle = escapeVCF(strings.Join(middleParts, " "))
	}

	return fmt.Sprintf("%s;%s;%s;;", family, given, middle)
}

func escapeVCF(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, ";", "\\;")
	s = strings.ReplaceAll(s, ",", "\\,")
	s = strings.ReplaceAll(s, "\r\n", "\\n")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\n")
	return s
}

func ConvertCSVToVCF(r io.Reader, w io.Writer, cfg Config) (*ConversionSummary, error) {
	result, err := ParseCSV(r, cfg)
	if err != nil {
		return nil, fmt.Errorf("parsing CSV: %w", err)
	}

	if cfg.Verbose {
		cfg.logf("Parsed %d contacts, %d parse errors", len(result.Contacts), len(result.Errors))
	}

	contacts := result.Contacts
	if cfg.Deduplicate && len(contacts) > 0 {
		cfg.logf("Deduplicating %d contacts...", len(contacts))
		dc := dedupe.DefaultConfig()
		dedupeResult := dedupe.Deduplicate(contacts, dc)

		// Preserve all singletons, replace duplicate clusters with their merged
		// representative. Avoid heuristic "name+org" identity keys, which can
		// drop distinct contacts.
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
			// Fallback: should not happen, but keep one contact rather than drop.
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

type ConversionSummary struct {
	InputRows         int
	OutputVCards      int
	ParseErrors       []model.ParseError
	Deduplicated      bool
	InputContacts     int
	DuplicatesRemoved int
}
