package vcf

import (
	"bytes"
	"strings"
	"testing"
)

func TestParseVCF_VCard30(t *testing.T) {
	in := "BEGIN:VCARD\r\n" +
		"VERSION:3.0\r\n" +
		"FN:John Doe\r\n" +
		"N:Doe;John;;;\r\n" +
		"ORG:Acme\\, Inc.\r\n" +
		"TEL;TYPE=CELL:+1-212-555-0101\r\n" +
		"EMAIL;TYPE=INTERNET:john@example.com\r\n" +
		"END:VCARD\r\n"

	res, err := ParseVCF(strings.NewReader(in))
	if err != nil {
		t.Fatalf("ParseVCF error: %v", err)
	}
	if len(res.Contacts) != 1 {
		t.Fatalf("expected 1 contact, got %d", len(res.Contacts))
	}
	c := res.Contacts[0]
	if c.Name != "John Doe" {
		t.Fatalf("expected name %q, got %q", "John Doe", c.Name)
	}
	if c.Organization != "Acme, Inc." {
		t.Fatalf("expected org %q, got %q", "Acme, Inc.", c.Organization)
	}
	if len(c.Phones) != 1 || c.Phones[0] != "+1-212-555-0101" {
		t.Fatalf("unexpected phones: %#v", c.Phones)
	}
	if len(c.Emails) != 1 || c.Emails[0] != "john@example.com" {
		t.Fatalf("unexpected emails: %#v", c.Emails)
	}
}

func TestParseVCF_VCard40_TelURIAndUnfold(t *testing.T) {
	// Unfolded FN line.
	in := "BEGIN:VCARD\r\n" +
		"VERSION:4.0\r\n" +
		"FN:Jane \r\n" +
		" Doe\r\n" +
		"N:Doe;Jane;;;\r\n" +
		"TEL;VALUE=uri:tel:+919876543210\r\n" +
		"EMAIL:jane@example.com\r\n" +
		"END:VCARD\r\n"

	res, err := ParseVCF(strings.NewReader(in))
	if err != nil {
		t.Fatalf("ParseVCF error: %v", err)
	}
	if len(res.Contacts) != 1 {
		t.Fatalf("expected 1 contact, got %d", len(res.Contacts))
	}
	c := res.Contacts[0]
	if c.Name != "Jane Doe" {
		t.Fatalf("expected name %q, got %q", "Jane Doe", c.Name)
	}
	if len(c.Phones) != 1 || c.Phones[0] != "+919876543210" {
		t.Fatalf("unexpected phones: %#v", c.Phones)
	}
}

func TestConvertVCFToCSV_Basic(t *testing.T) {
	in := "BEGIN:VCARD\r\n" +
		"VERSION:3.0\r\n" +
		"FN:Alice\r\n" +
		"TEL:+1 555 0100\r\n" +
		"TEL:+1 555 0101\r\n" +
		"EMAIL:alice@example.com\r\n" +
		"END:VCARD\r\n"

	var out bytes.Buffer
	_, err := ConvertVCFToCSV(strings.NewReader(in), &out, CSVConfig{})
	if err != nil {
		t.Fatalf("ConvertVCFToCSV error: %v", err)
	}

	s := out.String()
	if !strings.Contains(s, "name,phone,email,org") {
		t.Fatalf("missing header, got:\n%s", s)
	}
	if !strings.Contains(s, "Alice") {
		t.Fatalf("missing contact row, got:\n%s", s)
	}
	if !strings.Contains(s, "+1 555 0100") || !strings.Contains(s, "+1 555 0101") {
		t.Fatalf("missing phones, got:\n%s", s)
	}
}
