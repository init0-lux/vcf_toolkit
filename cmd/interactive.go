// Package cmd implements the CLI interface for vcf-toolkit.
package cmd

import (
	"bufio"
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/init0/vcf-toolkit/internal/convert"
	"github.com/init0/vcf-toolkit/internal/dedupe"
	"github.com/init0/vcf-toolkit/internal/model"
)

const (
	reset  = "\033[0m"
	bold   = "\033[1m"
	cyan   = "\033[36m"
	yellow = "\033[33m"
	green  = "\033[32m"
	dim    = "\033[2m"
)

type menuState struct {
	inputFile  string
	outputFile string
	mode       int
}

var modes = []struct {
	num  int
	desc string
}{
	{1, "Normalize -> CSV"},
	{2, "Normalize + Dedupe -> CSV"},
	{3, "Normalize -> VCF"},
	{4, "Normalize + Dedupe -> VCF"},
	{5, "Normalize -> JSON"},
}

func runInteractive() {
	s := &menuState{}
	scanner := bufio.NewScanner(os.Stdin)

	for {
		renderMenu(s)

		if !scanner.Scan() {
			break
		}
		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}
		if input == "q" || input == "Q" {
			fmt.Println()
			os.Exit(0)
		}

		switch {
		case strings.HasPrefix(strings.ToLower(input), "f="):
			s.inputFile = strings.TrimSpace(input[2:])
			continue
		case strings.HasPrefix(strings.ToLower(input), "o="):
			s.outputFile = strings.TrimSpace(input[2:])
			continue
		}

		var n int
		if _, err := fmt.Sscanf(input, "%d", &n); err == nil && n >= 1 && n <= len(modes) {
			s.mode = n
			executePipeline(s, scanner)
			continue
		}

		// Bare text treated as file path -- run immediately with default mode.
		s.inputFile = input
		if s.mode == 0 {
			s.mode = 1
		}
		executePipeline(s, scanner)
	}
}

func renderMenu(s *menuState) {
	clearScreen()

	fmt.Println()
	hline := strings.Repeat("\u2500", 38)

	fmt.Printf("  %s\u250c%s%s%s\n", cyan, hline, reset, "")
	fmt.Printf("  %s\u2502%s          %svcf-toolkit%s              %s\u2502%s\n", cyan, reset, bold, reset, cyan, reset)
	fmt.Printf("  %s\u2502%s     %sContact Data Processor%s        %s\u2502%s\n", cyan, reset, dim, reset, cyan, reset)
	fmt.Printf("  %s\u2502%s%s%s\n", cyan, hline, reset, "")

	inFile := s.inputFile
	if inFile == "" {
		inFile = dim + "(not set)" + reset
	}
	outFile := s.outputFile
	if outFile == "" {
		outFile = "stdout"
	}
	fmt.Printf("  %s\u2502%s  Input:  %-27s %s\u2502%s\n", cyan, reset, truncate(inFile, 27), cyan, reset)
	fmt.Printf("  %s\u2502%s  Output: %-27s %s\u2502%s\n", cyan, reset, truncate(outFile, 27), cyan, reset)
	fmt.Printf("  %s\u2502%s%s%s\n", cyan, hline, reset, "")

	for _, m := range modes {
		mark := " "
		if s.mode == m.num {
			mark = green + "\u25b6" + reset
		}
		label := fmt.Sprintf("%d) %s", m.num, m.desc)
		fmt.Printf("  %s\u2502%s  %s %-26s %s\u2502%s\n", cyan, reset, mark, label, cyan, reset)
	}

	fmt.Printf("  %s\u2502%s%s%s\n", cyan, reset, strings.Repeat(" ", 38), "")
	fmt.Printf("  %s\u2502%s  %sq)%s  Quit                         %s\u2502%s\n", cyan, reset, dim, reset, cyan, reset)
	fmt.Printf("  %s\u2514%s%s%s\n", cyan, hline, reset, "")
	fmt.Printf("  %s\u25b6%s ", yellow, reset)
}

func clearScreen() {
	fmt.Print("\033[H\033[2J")
}

func truncate(s string, n int) string {
	// Strip ANSI codes for length calculation.
	clean := s
	for _, code := range []string{reset, bold, dim, green, yellow, cyan} {
		clean = strings.ReplaceAll(clean, code, "")
	}
	if len(clean) <= n {
		return s
	}
	return s[:n-1] + "\u2026"
}

func executePipeline(s *menuState, scanner *bufio.Scanner) {
	if s.inputFile == "" {
		fmt.Print("\n  Enter input CSV path: ")
		if !scanner.Scan() {
			return
		}
		s.inputFile = strings.TrimSpace(scanner.Text())
		if s.inputFile == "" {
			fmt.Println("  No file specified. Cancelled.")
			fmt.Print("\n  Press Enter to continue...")
			scanner.Scan()
			return
		}
	}

	w, closeOut, err := openOutputFile(s.outputFile)
	if err != nil {
		fmt.Printf("  Error: %v\n", err)
		fmt.Print("\n  Press Enter to continue...")
		scanner.Scan()
		return
	}
	if closeOut {
		defer w.Close()
	}

	f, err := os.Open(s.inputFile)
	if err != nil {
		fmt.Printf("  Error opening file: %v\n", err)
		fmt.Print("\n  Press Enter to continue...")
		scanner.Scan()
		return
	}

	fmt.Println()

	switch s.mode {
	case 1:
		err = runNormalizeToCSV(f, w)
	case 2:
		err = runNormalizeDedupeToCSV(f, w)
	case 3:
		err = runNormalizeToVCF(f, w)
	case 4:
		err = runNormalizeDedupeToVCF(f, w)
	case 5:
		err = runNormalizeToJSON(f, w)
	}

	f.Close()

	if err != nil {
		fmt.Fprintf(os.Stderr, "  Error: %v\n", err)
	}

	if s.outputFile != "" && err == nil {
		fmt.Printf("  %s\u2713%s Output written to %s\n", green, reset, s.outputFile)
	}

	fmt.Print("\n  Press Enter to continue...")
	scanner.Scan()
}

func openOutputFile(path string) (w io.WriteCloser, closeOut bool, err error) {
	if path != "" {
		w, err = os.Create(path)
		if err != nil {
			return nil, false, err
		}
		return w, true, nil
	}
	return os.Stdout, false, nil
}

// Pipeline runners.

func runNormalizeToCSV(r io.Reader, w io.Writer) error {
	contacts, err := parseAndNormalize(r)
	if err != nil {
		return err
	}
	if len(contacts) == 0 {
		return fmt.Errorf("no contacts found")
	}
	return writeCSV(w, contacts)
}

func runNormalizeDedupeToCSV(r io.Reader, w io.Writer) error {
	contacts, err := parseAndNormalize(r)
	if err != nil {
		return err
	}
	if len(contacts) == 0 {
		return fmt.Errorf("no contacts found")
	}
	cfg := dedupe.DefaultConfig()
	result := dedupe.Deduplicate(toModelContacts(contacts), cfg)
	contacts = fromMergedContacts(result)
	return writeCSV(w, contacts)
}

func runNormalizeToVCF(r io.Reader, w io.Writer) error {
	contacts, err := parseAndNormalize(r)
	if err != nil {
		return err
	}
	if len(contacts) == 0 {
		return fmt.Errorf("no contacts found")
	}
	return writeVCFTo(w, contacts)
}

func runNormalizeDedupeToVCF(r io.Reader, w io.Writer) error {
	contacts, err := parseAndNormalize(r)
	if err != nil {
		return err
	}
	if len(contacts) == 0 {
		return fmt.Errorf("no contacts found")
	}
	cfg := dedupe.DefaultConfig()
	result := dedupe.Deduplicate(toModelContacts(contacts), cfg)
	contacts = fromMergedContacts(result)
	return writeVCFTo(w, contacts)
}

func runNormalizeToJSON(r io.Reader, w io.Writer) error {
	contacts, err := parseAndNormalize(r)
	if err != nil {
		return err
	}
	if len(contacts) == 0 {
		return fmt.Errorf("no contacts found")
	}
	return writeJSON(w, contacts)
}

func writeVCFTo(w io.Writer, contacts []normalizedContact) error {
	pr, pw := io.Pipe()

	go func() {
		cw := csv.NewWriter(pw)
		cw.Write([]string{"name", "phone", "email", "org"})
		for _, c := range contacts {
			cw.Write([]string{
				c.Name,
				strings.Join(c.Phones, "; "),
				strings.Join(c.Emails, "; "),
				c.Org,
			})
		}
		cw.Flush()
		pw.CloseWithError(cw.Error())
	}()

	_, err := convert.ConvertCSVToVCF(pr, w, convert.Config{
		Deduplicate: false,
		Stderr:      os.Stderr,
	})

	return err
}

func parseAndNormalize(r io.Reader) ([]normalizedContact, error) {
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
	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			// Keep interactive mode forgiving; the CLI commands already surface
			// row-level errors in non-interactive flows.
			continue
		}

		nc := normalizeRow(record, headers, mapping)
		if nc.Name != "" || len(nc.Phones) > 0 || len(nc.Emails) > 0 || nc.Org != "" {
			contacts = append(contacts, nc)
		}
	}

	return contacts, nil
}

func toModelContacts(in []normalizedContact) []model.Contact {
	out := make([]model.Contact, 0, len(in))
	for _, c := range in {
		out = append(out, model.Contact{
			Name:         c.Name,
			Phones:       append([]string(nil), c.Phones...),
			Emails:       append([]string(nil), c.Emails...),
			Organization: c.Org,
		})
	}
	return out
}

func fromMergedContacts(result model.DedupeResult) []normalizedContact {
	// Prefer merged contacts when they exist; otherwise return the originals.
	if len(result.Merged) == 0 {
		var out []normalizedContact
		for _, cl := range result.Clusters {
			if len(cl) != 1 {
				continue
			}
			c := cl[0]
			out = append(out, normalizedContact{
				Name:   c.Name,
				Phones: append([]string(nil), c.Phones...),
				Emails: append([]string(nil), c.Emails...),
				Org:    c.Organization,
			})
		}
		return out
	}

	out := make([]normalizedContact, 0, len(result.Merged))
	for _, mc := range result.Merged {
		out = append(out, normalizedContact{
			Name:   mc.Contact.Name,
			Phones: append([]string(nil), mc.Contact.Phones...),
			Emails: append([]string(nil), mc.Contact.Emails...),
			Org:    mc.Contact.Organization,
		})
	}
	return out
}
