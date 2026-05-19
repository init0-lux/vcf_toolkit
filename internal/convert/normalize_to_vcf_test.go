package convert

import (
	"bytes"
	"strings"
	"testing"

	"github.com/init0/vcf-toolkit/internal/normalize"
)

func TestNormalizeCSVToVCF_NormalizesEmailAndPhone(t *testing.T) {
	in := "name,phone,email,org\n" +
		"John Doe,(123) 456-7890,John+tag@Example.com,Acme\n"

	var out bytes.Buffer
	summary, err := NormalizeCSVToVCF(strings.NewReader(in), &out, NormalizeVCFConfig{
		Name: normalize.NameConfig{
			UseLLM: false,
		},
		Phone: normalize.PhoneConfig{
			DefaultCountry:   "+1",
			StrictValidation: true,
		},
		Email: normalize.EmailConfig{
			StripAliases:     true,
			StrictValidation: false,
		},
		Deduplicate: false,
	})
	if err != nil {
		t.Fatalf("NormalizeCSVToVCF error: %v", err)
	}
	if summary.OutputVCards != 1 {
		t.Fatalf("expected 1 vcard, got %d", summary.OutputVCards)
	}

	s := out.String()
	if !strings.Contains(s, "BEGIN:VCARD") || !strings.Contains(s, "VERSION:4.0") {
		t.Fatalf("expected vcard output, got:\n%s", s)
	}
	if !strings.Contains(s, "FN:John Doe") {
		t.Fatalf("expected FN, got:\n%s", s)
	}
	// Phone should be normalized to +11234567890 and emitted as a tel: URI.
	if !strings.Contains(s, "TEL;VALUE=uri:tel:+11234567890") {
		t.Fatalf("expected normalized phone, got:\n%s", s)
	}
	// Email should be lowercased and alias stripped.
	if !strings.Contains(s, "EMAIL:john@example.com") {
		t.Fatalf("expected normalized email, got:\n%s", s)
	}
}

func TestNormalizeCSVToVCF_DeduplicateAfterNormalization(t *testing.T) {
	in := "name,phone,email\n" +
		"Alice,+1 (212) 555-0101,alice@example.com\n" +
		"Alice Smith,+12125550101,ALICE+spam@example.com\n"

	var out bytes.Buffer
	summary, err := NormalizeCSVToVCF(strings.NewReader(in), &out, NormalizeVCFConfig{
		Phone: normalize.PhoneConfig{
			DefaultCountry:   "+1",
			StrictValidation: true,
		},
		Email: normalize.EmailConfig{
			StripAliases:     true,
			StrictValidation: false,
		},
		Deduplicate: true,
	})
	if err != nil {
		t.Fatalf("NormalizeCSVToVCF error: %v", err)
	}
	if summary.OutputVCards != 1 {
		t.Fatalf("expected 1 vcard after dedupe, got %d", summary.OutputVCards)
	}
	if summary.DuplicatesRemoved < 1 {
		t.Fatalf("expected duplicates removed, got %d", summary.DuplicatesRemoved)
	}
}
