package cmd

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/init0-lux/vcf-toolkit/internal/normalize"
	"github.com/spf13/cobra"
)

var normalizeCmd = &cobra.Command{
	Use:   "normalize [file]",
	Short: "Normalize contact data (names, phones, emails)",
	Long: `Normalize contact data from a CSV file or stdin.

Reads a CSV with columns for name, phone, email, and org headers
and outputs normalized data in CSV or JSON format.

Examples:
  vcf-toolkit normalize contacts.csv
  cat contacts.csv | vcf-toolkit normalize
  vcf-toolkit normalize contacts.csv --out clean.csv
  vcf-toolkit normalize contacts.csv --json
`,
	Args: cobra.MaximumNArgs(1),
	RunE: runNormalize,
}

func init() {
	rootCmd.AddCommand(normalizeCmd)
	normalizeCmd.Flags().BoolVarP(&jsonOutput, "json", "j", false, "output as JSON")
}

func runNormalize(cmd *cobra.Command, args []string) error {
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

	contacts, err := parseAndNormalizeRows(r)
	if err != nil {
		return err
	}

	if jsonOutput {
		return writeJSON(w, contacts)
	}
	return writeCSV(w, contacts)
}

type normalizedContact struct {
	Name   string   `json:"name"`
	Phones []string `json:"phones"`
	Emails []string `json:"emails"`
	Org    string   `json:"org"`
}

func parseAndNormalizeRows(r io.Reader) ([]normalizedContact, error) {
	reader := csv.NewReader(r)
	reader.TrimLeadingSpace = true
	reader.FieldsPerRecord = -1
	reader.LazyQuotes = true

	rawHeaders, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("reading CSV headers: %w", err)
	}

	headers := make([]string, len(rawHeaders))
	for i, h := range rawHeaders {
		headers[i] = strings.TrimSpace(h)
	}

	mapping := buildMapping(headers)

	var contacts []normalizedContact
	line := 2

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "line %d: read error: %v\n", line, err)
			line++
			continue
		}

		nc := normalizeRow(record, headers, mapping)
		if nc.Name != "" || len(nc.Phones) > 0 || len(nc.Emails) > 0 || nc.Org != "" {
			contacts = append(contacts, nc)
		}
		line++
	}

	return contacts, nil
}

func normalizeRow(record, headers []string, mapping map[string]string) normalizedContact {
	var nc normalizedContact

	for i, header := range headers {
		if i >= len(record) {
			break
		}
		val := strings.TrimSpace(record[i])
		if val == "" {
			continue
		}

		field, ok := mapping[header]
		if !ok {
			continue
		}

		switch field {
		case "name":
			if normalized := normalize.NormalizeName(val, normalize.NameConfig{}); normalized.Normalized && normalized.FullName != "" {
				nc.Name = normalized.FullName
			} else if nc.Name == "" {
				nc.Name = val
			}
		case "phone":
			if normalized := normalize.NormalizePhone(val, normalize.PhoneConfig{}); normalized.Valid && normalized.Normalized != "" {
				nc.Phones = append(nc.Phones, normalized.Normalized)
			} else {
				nc.Phones = append(nc.Phones, val)
			}
		case "email":
			if normalized := normalize.NormalizeEmail(val, normalize.EmailConfig{StripAliases: true}); normalized.Valid && normalized.Normalized != "" {
				nc.Emails = append(nc.Emails, normalized.Normalized)
			} else {
				nc.Emails = append(nc.Emails, val)
			}
		case "org":
			if nc.Org == "" {
				nc.Org = val
			}
		}
	}

	return nc
}

func buildMapping(headers []string) map[string]string {
	mapping := make(map[string]string, len(headers))
	for _, header := range headers {
		key := fieldKey(header)
		if key != "" {
			mapping[header] = key
		}
	}
	return mapping
}

func fieldKey(header string) string {
	s := strings.ToLower(header)
	s = strings.NewReplacer(" ", "_", "-", "_").Replace(s)

	switch {
	case matchesAny(s,
		"name", "fullname", "full_name",
		"contactname", "displayname",
		"first_name", "firstname", "given_name", "givenname",
		"last_name", "lastname", "family_name", "familyname", "surname"):
		return "name"
	case matchesAny(s,
		"phone", "telephone", "tel", "mobile", "cell",
		"phone_number", "phonenumber", "contactnumber"):
		return "phone"
	case matchesAny(s,
		"email", "e_mail", "email_address", "emailaddress", "mail"):
		return "email"
	case matchesAny(s,
		"org", "organization", "organisation", "company", "employer", "affiliation"):
		return "org"
	}
	return ""
}

func matchesAny(s string, prefixes ...string) bool {
	for _, p := range prefixes {
		if s == p {
			return true
		}
	}
	return false
}

func writeJSON(w io.Writer, contacts []normalizedContact) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(contacts)
}

func writeCSV(w io.Writer, contacts []normalizedContact) error {
	writer := csv.NewWriter(w)
	defer writer.Flush()

	if err := writer.Write([]string{"name", "phone", "email", "org"}); err != nil {
		return err
	}

	for _, c := range contacts {
		if err := writer.Write([]string{
			c.Name,
			strings.Join(c.Phones, "; "),
			strings.Join(c.Emails, "; "),
			c.Org,
		}); err != nil {
			return err
		}
	}

	writer.Flush()
	return writer.Error()
}

// openInput resolves the input source from CLI args or defaults to stdin.
func openInput(args []string) (io.ReadCloser, error) {
	if len(args) > 0 && args[0] != "-" {
		return os.Open(args[0])
	}
	return os.Stdin, nil
}

// openOutput resolves the output destination from --out flag or defaults to stdout.
func openOutput() (io.WriteCloser, error) {
	if outputFile != "" {
		return os.Create(outputFile)
	}
	return os.Stdout, nil
}
