// Package convert implements CSV to VCF conversion with automatic
// header mapping, multi-field support, and error logging for malformed rows.
//
// It follows the vCard 4.0 specification (RFC 6350) for output formatting.
package convert

import (
	"encoding/csv"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/init0/vcf-toolkit/internal/dedupe"
	"github.com/init0/vcf-toolkit/internal/model"
	"github.com/init0/vcf-toolkit/internal/normalize"
)

type Config struct {
	// default country code for phone normalization
	// If empty, phones without a leading "+" will not be normalized to E.164.
	DefaultCountry string

	// HeaderMapping allows manual override of column-to-field mapping.
	// Keys are CSV column headers, values are field names:
	// "name", "phone", "email", "org".
	// If nil or empty, automatic detection is used.
	HeaderMapping map[string]string
	Deduplicate   bool
	Verbose       bool
	Stderr        io.Writer
}

// HeaderMapping constants
const (
	FieldName  = "name"
	FieldPhone = "phone"
	FieldEmail = "email"
	FieldOrg   = "org"
)

// logf writes a formatted log message to the configured stderr.
func (cfg Config) logf(format string, args ...interface{}) {
	if cfg.Stderr != nil {
		fmt.Fprintf(cfg.Stderr, format+"\n", args...)
	}
}

// ParseResult holds the results of CSV parsing.
type ParseResult struct {
	Contacts []model.Contact
	Errors   []model.ParseError
}

// ParseCSV reads CSV data from the reader, parses headers, and maps
// columns to contact fields. Returns the list of contacts and any parse errors.
func ParseCSV(r io.Reader, cfg Config) (*ParseResult, error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1 // allow variable number of fields per row
	reader.ReuseRecord = false
	reader.LazyQuotes = true

	// header row
	rawHeaders, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("reading CSV headers: %w", err)
	}

	headers := make([]string, len(rawHeaders))
	for i, h := range rawHeaders {
		headers[i] = strings.TrimSpace(h)
	}

	// header-to-field mapping
	mapping := buildHeaderMapping(headers, cfg.HeaderMapping)

	if cfg.Verbose {
		cfg.logf("Detected header mapping:")
		for h, f := range mapping {
			cfg.logf("  %q -> %s", h, f)
		}
	}

	result := &ParseResult{}

	line := 2 // 1-indexed, header is line 1
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

		// Only add contacts that have at least some content
		if c.Name != "" || len(c.Phones) > 0 || len(c.Emails) > 0 || c.Organization != "" {
			c.Source.Row = line
			result.Contacts = append(result.Contacts, c)
		}

		line++
	}

	return result, nil
}

// contactFromRow parses a single CSV row into a Contact using the
// provided header mapping. Returns the contact and any parse errors.
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

	// Flag rows that produced no contact data at all
	if c.Name == "" && len(c.Phones) == 0 && len(c.Emails) == 0 && c.Organization == "" {
		errs = append(errs, model.ParseError{
			Field:   "row",
			Message: "no recognizable contact fields in row",
		})
	}

	return c, errs
}

// buildHeaderMapping creates a canonical header-to-field mapping.
// It uses the user-provided mapping as an override on top of automatic detection.
func buildHeaderMapping(headers []string, override map[string]string) map[string]string {
	mapping := make(map[string]string, len(headers))

	// Normalize overrides to lowercase for case-insensitive matching
	normalizedOverride := make(map[string]string, len(override))
	for k, v := range override {
		normalizedOverride[strings.ToLower(k)] = v
	}

	for _, header := range headers {
		lower := strings.ToLower(header)

		// Check override first
		if field, ok := normalizedOverride[lower]; ok {
			mapping[header] = field
			continue
		}

		// Automatic detection
		field := detectField(lower)
		if field != "" {
			mapping[header] = field
		}
	}

	return mapping
}

// detectField tries to automatically determine the field type from a
// normalized (lowercase) CSV column header.
func detectField(header string) string {
	// Normalize: strip spaces, underscores, hyphens, dots
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

// normalizeHeader strips punctuation and reduces whitespace for matching.
// Separators like _, -, and spaces are all unified to "_" for pattern matching.
func normalizeHeader(header string) string {
	re := regexp.MustCompile(`[ _\-]+`)
	return re.ReplaceAllString(header, "_")
}

// ContactToVCF converts a single Contact to a vCard 4.0 string.
func ContactToVCF(c model.Contact) string {
	var b strings.Builder

	b.WriteString("BEGIN:VCARD\r\n")
	b.WriteString("VERSION:4.0\r\n")
	b.WriteString("PRODID:-//vcf-toolkit//EN\r\n")

	// FN (Formatted Name) — required in vCard 4.0 (RFC 6350 §6.2.1)
	fn := c.Name
	if fn == "" {
		fn = "Unknown"
	}
	b.WriteString(fmt.Sprintf("FN:%s\r\n", escapeVCF(fn)))

	// N (Structured Name) — N:Family;Given;Middle;Prefix;Suffix
	n := structuredName(c.Name)
	b.WriteString(fmt.Sprintf("N:%s\r\n", n))

	// Organization
	if c.Organization != "" {
		b.WriteString(fmt.Sprintf("ORG:%s\r\n", escapeVCF(c.Organization)))
	}

	// Phone numbers — use VALUE=uri with tel: URI scheme (RFC 6350 §6.4.1)
	for _, phone := range c.Phones {
		normalized := normalize.NormalizePhone(phone, normalize.PhoneConfig{
			DefaultCountry: "",
		})
		// Ensure the tel: URI always has a "+" prefix for E.164 compliance.
		// NormalizePhone may return digits without "+" when no default country
		// is set and the input lacks a leading "+".
		tel := normalized.Normalized
		if normalized.Valid && tel != "" {
			if !strings.HasPrefix(tel, "+") {
				tel = "+" + tel
			}
			b.WriteString(fmt.Sprintf("TEL;VALUE=uri:tel:%s\r\n", tel))
		} else {
			// Fallback: extract digits and prepend "+".
			cleaned := normalize.PhoneDigits(phone)
			if cleaned != "" {
				b.WriteString(fmt.Sprintf("TEL;VALUE=uri:tel:+%s\r\n", cleaned))
			} else {
				b.WriteString(fmt.Sprintf("TEL:%s\r\n", escapeVCF(phone)))
			}
		}
	}

	// Emails
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

// structuredName converts a full name string into a vCard N: property value.
// Format: N:Family;Given;Middle;Prefix;Suffix
// Returns ";;;;" with FN fallback if parsing fails.
func structuredName(fullName string) string {
	fullName = strings.TrimSpace(fullName)
	if fullName == "" {
		return ";;;;"
	}

	parts := strings.Fields(fullName)
	if len(parts) == 1 {
		// Single word — use as given name, leave family empty
		return fmt.Sprintf(";%s;;;", escapeVCF(parts[0]))
	}

	// Heuristic: first word is given name, last word is family name
	given := escapeVCF(parts[0])
	family := escapeVCF(parts[len(parts)-1])

	// Middle names (everything between first and last)
	middle := ""
	if len(parts) > 2 {
		middleParts := parts[1 : len(parts)-1]
		middle = escapeVCF(strings.Join(middleParts, " "))
	}

	return fmt.Sprintf("%s;%s;%s;;", family, given, middle)
}

// escapeVCF escapes special characters in VCF property values.
// RFC 6350 requires escaping: \, ; , and newlines.
func escapeVCF(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, ";", "\\;")
	s = strings.ReplaceAll(s, ",", "\\,")
	s = strings.ReplaceAll(s, "\r\n", "\\n")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\r", "\\n")
	return s
}

// ConvertCSVToVCF reads CSV data from r, parses it into contacts,
// and writes the VCF output to w. Returns a summary of the conversion.
func ConvertCSVToVCF(r io.Reader, w io.Writer, cfg Config) (*ConversionSummary, error) {
	result, err := ParseCSV(r, cfg)
	if err != nil {
		return nil, fmt.Errorf("parsing CSV: %w", err)
	}

	if cfg.Verbose {
		cfg.logf("Parsed %d contacts, %d parse errors", len(result.Contacts), len(result.Errors))
	}

	// Optionally deduplicate
	contacts := result.Contacts
	if cfg.Deduplicate && len(contacts) > 0 {
		cfg.logf("Deduplicating %d contacts...", len(contacts))
		dc := dedupe.DefaultConfig()
		dedupeResult := dedupe.Deduplicate(contacts, dc)

		// Use merged contacts if any duplicates were found
		if len(dedupeResult.Merged) > 0 {
			// Build the final list: singles (not in any merged cluster) + merged
			mergedIDs := make(map[string]bool)
			for _, mc := range dedupeResult.Merged {
				mergedIDs[mc.Contact.Name+mc.Contact.Organization] = true
			}

			var final []model.Contact
			for _, c := range contacts {
				key := c.Name + c.Organization
				if !mergedIDs[key] {
					final = append(final, c)
				}
			}
			for _, mc := range dedupeResult.Merged {
				final = append(final, mc.Contact)
			}
			contacts = final
		}

		cfg.logf("After deduplication: %d contacts (removed %d duplicates)",
			len(contacts), len(result.Contacts)-len(contacts))
	}

	// Write VCF
	for i, c := range contacts {
		vcf := ContactToVCF(c)
		_, err := io.WriteString(w, vcf)
		if err != nil {
			return nil, fmt.Errorf("writing VCF for contact %d: %w", i+1, err)
		}
	}

	summary := &ConversionSummary{
		InputRows:         len(result.Contacts),
		OutputVCards:      len(contacts),
		ParseErrors:       result.Errors,
		Deduplicated:      cfg.Deduplicate,
		InputContacts:     len(result.Contacts),
		DuplicatesRemoved: len(result.Contacts) - len(contacts),
	}

	return summary, nil
}

// ConversionSummary holds statistics about a CSV to VCF conversion.
type ConversionSummary struct {
	InputRows         int
	OutputVCards      int
	ParseErrors       []model.ParseError
	Deduplicated      bool
	InputContacts     int
	DuplicatesRemoved int
}
