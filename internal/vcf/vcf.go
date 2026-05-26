// Package vcf implements minimal vCard parsing and conversion.
//
// Goals:
// - Accept common vCard 3.0 and 4.0 exports.
// - Parse only the fields vcf-toolkit cares about (FN/N, TEL, EMAIL, ORG).
// - Be tolerant: ignore unknown properties and keep values mostly raw.
package vcf

import (
	"bufio"
	"fmt"
	"io"
	"strings"

	"github.com/init0-lux/vcf-toolkit/internal/model"
)

type ParseResult struct {
	Contacts []model.Contact
	Errors   []model.ParseError
}

// ParseVCF parses a vCard stream (3.0/4.0) and returns extracted contacts.
// It supports line unfolding per RFC (continuation lines begin with space or tab).
func ParseVCF(r io.Reader) (*ParseResult, error) {
	sc := bufio.NewScanner(r)
	// vCard lines can be long (PHOTO, etc.). We ignore most properties but still
	// need to scan them safely.
	buf := make([]byte, 64*1024)
	sc.Buffer(buf, 2*1024*1024)

	res := &ParseResult{}

	var cur []string
	flushCard := func() {
		if len(cur) == 0 {
			return
		}
		cardIndex := len(res.Contacts) + 1
		c, errs := parseCard(cur, cardIndex)
		if c.Name != "" || len(c.Phones) > 0 || len(c.Emails) > 0 || c.Organization != "" {
			res.Contacts = append(res.Contacts, c)
		}
		res.Errors = append(res.Errors, errs...)
		cur = nil
	}

	var prev string
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, " ") || strings.HasPrefix(line, "\t") {
			// Unfold: append continuation without the leading whitespace.
			prev += strings.TrimLeft(line, " \t")
			continue
		}
		if prev != "" {
			cur = append(cur, prev)
		}
		prev = line
	}
	if prev != "" {
		cur = append(cur, prev)
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("reading vcf: %w", err)
	}

	// A stream can contain multiple cards; we flush at END:VCARD and also at EOF.
	var cards [][]string
	var card []string
	for _, l := range cur {
		card = append(card, l)
		if strings.EqualFold(strings.TrimSpace(l), "END:VCARD") {
			cards = append(cards, card)
			card = nil
		}
	}
	if len(card) > 0 {
		cards = append(cards, card)
	}
	for _, c := range cards {
		cur = c
		flushCard()
	}

	return res, nil
}

func parseCard(lines []string, cardIndex int) (model.Contact, []model.ParseError) {
	var c model.Contact
	var errs []model.ParseError

	var fn string
	var n string

	for _, line := range lines {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}

		prop, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}

		prop = strings.TrimSpace(prop)
		val = strings.TrimSpace(val)

		name := propName(prop)
		switch strings.ToUpper(name) {
		case "BEGIN", "END", "VERSION", "PRODID":
			continue
		case "FN":
			fn = unescapeText(val)
		case "N":
			n = unescapeText(val)
		case "ORG":
			if c.Organization == "" {
				c.Organization = unescapeText(val)
			}
		case "TEL":
			t := parseTelValue(val)
			if t != "" {
				c.Phones = append(c.Phones, t)
			} else {
				errs = append(errs, model.ParseError{Row: cardIndex, Field: "TEL", Message: "empty tel value"})
			}
		case "EMAIL":
			e := unescapeText(val)
			if e != "" {
				c.Emails = append(c.Emails, e)
			} else {
				errs = append(errs, model.ParseError{Row: cardIndex, Field: "EMAIL", Message: "empty email value"})
			}
		}
	}

	// Prefer FN if present; fall back to N.
	if fn != "" {
		c.Name = strings.TrimSpace(fn)
	} else if n != "" {
		c.Name = strings.TrimSpace(formatN(n))
	}

	return c, errs
}

func propName(prop string) string {
	// "TEL;TYPE=CELL;VALUE=uri" => "TEL"
	if i := strings.IndexByte(prop, ';'); i >= 0 {
		return prop[:i]
	}
	return prop
}

func parseTelValue(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}
	v = unescapeText(v)
	// vCard 4 often uses "tel:+123" in a URI value.
	if strings.HasPrefix(strings.ToLower(v), "tel:") {
		v = v[4:]
	}
	return strings.TrimSpace(v)
}

func formatN(n string) string {
	// N is "family;given;additional;prefix;suffix"
	parts := strings.Split(n, ";")
	for len(parts) < 5 {
		parts = append(parts, "")
	}
	family := strings.TrimSpace(parts[0])
	given := strings.TrimSpace(parts[1])
	additional := strings.TrimSpace(parts[2])
	prefix := strings.TrimSpace(parts[3])
	suffix := strings.TrimSpace(parts[4])

	var out []string
	if prefix != "" {
		out = append(out, prefix)
	}
	if given != "" {
		out = append(out, given)
	}
	if additional != "" {
		out = append(out, additional)
	}
	if family != "" {
		out = append(out, family)
	}
	if suffix != "" {
		out = append(out, suffix)
	}
	return strings.Join(out, " ")
}

func unescapeText(s string) string {
	// Minimal vCard text unescape.
	// vCard allows escaping: \n, \N, \\, \;, \,
	s = strings.ReplaceAll(s, `\\`, `\`)
	s = strings.ReplaceAll(s, `\n`, "\n")
	s = strings.ReplaceAll(s, `\N`, "\n")
	s = strings.ReplaceAll(s, `\;`, `;`)
	s = strings.ReplaceAll(s, `\,`, `,`)
	return s
}
