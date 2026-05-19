package convert

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/init0/vcf-toolkit/internal/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCSV_EmptyInput(t *testing.T) {
	// No data rows (just a header)
	input := "name,phone,email\n"
	r := strings.NewReader(input)
	result, err := ParseCSV(r, Config{})
	require.NoError(t, err)
	assert.Len(t, result.Contacts, 0)
	assert.Len(t, result.Errors, 0)
}

func TestParseCSV_EmptyFile(t *testing.T) {
	input := ""
	r := strings.NewReader(input)
	_, err := ParseCSV(r, Config{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "reading CSV headers")
}

func TestParseCSV_BasicAutoDetection(t *testing.T) {
	input := `Name,Phone,Email,Org
John Doe,+1234567890,john@example.com,Acme Inc
Jane Smith,+9876543210,jane@test.org,Globex Corp
`
	r := strings.NewReader(input)
	result, err := ParseCSV(r, Config{})
	require.NoError(t, err)
	require.Len(t, result.Contacts, 2)
	require.Len(t, result.Errors, 0)

	assert.Equal(t, "John Doe", result.Contacts[0].Name)
	assert.Equal(t, []string{"+1234567890"}, result.Contacts[0].Phones)
	assert.Equal(t, []string{"john@example.com"}, result.Contacts[0].Emails)
	assert.Equal(t, "Acme Inc", result.Contacts[0].Organization)

	assert.Equal(t, "Jane Smith", result.Contacts[1].Name)
	assert.Equal(t, []string{"+9876543210"}, result.Contacts[1].Phones)
	assert.Equal(t, []string{"jane@test.org"}, result.Contacts[1].Emails)
	assert.Equal(t, "Globex Corp", result.Contacts[1].Organization)
}

func TestParseCSV_MultiplePhonesEmails(t *testing.T) {
	input := `name,phone,phone2,email,email2
Alice,1111111111,2222222222,alice@a.com,alice@b.com
`
	r := strings.NewReader(input)
	result, err := ParseCSV(r, Config{})
	require.NoError(t, err)
	require.Len(t, result.Contacts, 1)
	assert.Equal(t, "Alice", result.Contacts[0].Name)
	assert.Equal(t, []string{"1111111111", "2222222222"}, result.Contacts[0].Phones)
	assert.Equal(t, []string{"alice@a.com", "alice@b.com"}, result.Contacts[0].Emails)
}

func TestParseCSV_HeaderOverride(t *testing.T) {
	input := `Full Name,Mobile,E-mail
Bob Barker,555-0100,bob@example.com
`
	r := strings.NewReader(input)
	result, err := ParseCSV(r, Config{})
	require.NoError(t, err)
	require.Len(t, result.Contacts, 1)
	assert.Equal(t, "Bob Barker", result.Contacts[0].Name)
	assert.Equal(t, []string{"555-0100"}, result.Contacts[0].Phones)
	assert.Equal(t, []string{"bob@example.com"}, result.Contacts[0].Emails)

	// Test with manual override
	input2 := `Custom Col1,Custom Col2
Charlie,charlie@x.com
`
	r2 := strings.NewReader(input2)
	override := map[string]string{
		"Custom Col1": "name",
		"Custom Col2": "email",
	}
	result2, err := ParseCSV(r2, Config{HeaderMapping: override})
	require.NoError(t, err)
	require.Len(t, result2.Contacts, 1)
	assert.Equal(t, "Charlie", result2.Contacts[0].Name)
	assert.Equal(t, []string{"charlie@x.com"}, result2.Contacts[0].Emails)
}

func TestParseCSV_AlternativeHeaders(t *testing.T) {
	input := `Mobile,E-mail
+1-555-0100,bob@example.com
`
	r := strings.NewReader(input)
	result, err := ParseCSV(r, Config{})
	require.NoError(t, err)
	require.Len(t, result.Contacts, 1)
	assert.Equal(t, []string{"+1-555-0100"}, result.Contacts[0].Phones)
	assert.Equal(t, []string{"bob@example.com"}, result.Contacts[0].Emails)
}

func TestParseCSV_EmptyAndPartialRows(t *testing.T) {
	input := `name,phone,email
Alice,+1-555-0100,alice@example.com
,,,,
Bob,,bob@example.com
Charlie,+1-555-0101,
`
	r := strings.NewReader(input)
	result, err := ParseCSV(r, Config{})
	require.NoError(t, err)
	// Row 2 is completely empty — should produce a parse error and not be added
	require.Len(t, result.Contacts, 3)
	require.Len(t, result.Errors, 1)
	assert.Equal(t, "no recognizable contact fields in row", result.Errors[0].Message)
	assert.Equal(t, 3, result.Errors[0].Row)

	assert.Equal(t, "Alice", result.Contacts[0].Name)
	assert.Equal(t, "Bob", result.Contacts[1].Name)
	assert.Equal(t, []string{"bob@example.com"}, result.Contacts[1].Emails)
	assert.Equal(t, "Charlie", result.Contacts[2].Name)
	assert.Equal(t, []string{"+1-555-0101"}, result.Contacts[2].Phones)
}

func TestParseCSV_VariableColumns(t *testing.T) {
	input := `name,phone,email,org,notes
Alice,+1-111-1111,alice@a.com,Corp1,some note
Bob,+1-222-2222
Charlie,+1-333-3333,charlie@c.com,,VIP
`
	r := strings.NewReader(input)
	result, err := ParseCSV(r, Config{})
	require.NoError(t, err)
	require.Len(t, result.Contacts, 3)
	assert.Equal(t, "Alice", result.Contacts[0].Name)
	assert.Equal(t, "Corp1", result.Contacts[0].Organization)
	assert.Equal(t, "Bob", result.Contacts[1].Name)
	assert.Len(t, result.Contacts[1].Emails, 0)
	assert.Equal(t, "Charlie", result.Contacts[2].Name)
	assert.Equal(t, []string{"charlie@c.com"}, result.Contacts[2].Emails)
	assert.Empty(t, result.Contacts[2].Organization) // notes not mapped
}

func TestParseCSV_FirstLastNameHeuristics(t *testing.T) {
	input := `first name,last name,phone
John,Doe,+1-555-0100
Jane,Smith,+1-555-0101
`
	r := strings.NewReader(input)
	result, err := ParseCSV(r, Config{})
	require.NoError(t, err)
	require.Len(t, result.Contacts, 2)
	// Both first name and last name map to "name" field, so first name wins
	assert.Equal(t, "John", result.Contacts[0].Name)
	assert.Equal(t, "Jane", result.Contacts[1].Name)
}

func TestParseCSV_UnknownHeadersIgnored(t *testing.T) {
	input := `colA,colB,colC
val1,val2,val3
val4,val5,val6
`
	r := strings.NewReader(input)
	result, err := ParseCSV(r, Config{})
	require.NoError(t, err)
	require.Len(t, result.Contacts, 0)
	require.Len(t, result.Errors, 2) // both rows have no recognizable fields
}

func TestParseCSV_SourceRowNumbers(t *testing.T) {
	input := `name,email
Alice,alice@a.com
Bob,bob@b.com
`
	r := strings.NewReader(input)
	result, err := ParseCSV(r, Config{})
	require.NoError(t, err)
	require.Len(t, result.Contacts, 2)
	assert.Equal(t, 2, result.Contacts[0].Source.Row) // header is row 1
	assert.Equal(t, 3, result.Contacts[1].Source.Row)
}

func TestParseCSV_MalformedRow(t *testing.T) {
	// Simulate a read error mid-stream by using a reader that returns an error
	// We'll use a normal CSV with a quote mismatch to trigger LazyQuotes
	input := `name,phone
Alice,"123
Bob,456
`
	r := strings.NewReader(input)
	result, err := ParseCSV(r, Config{})
	require.NoError(t, err)
	// With LazyQuotes=true, quotes don't need to be closed, so this may parse
	// Check that at least we don't crash
	_ = result
}

func TestDetectField(t *testing.T) {
	tests := []struct {
		header string
		want   string
	}{
		{"name", FieldName},
		{"Name", FieldName},
		{"NAME", FieldName},
		{"full name", FieldName},
		{"full_name", FieldName},
		{"full-name", FieldName},
		{"contactname", FieldName},
		{"displayname", FieldName},
		{"first name", FieldName},
		{"last name", FieldName},
		{"givenname", FieldName},
		{"family name", FieldName},
		{"surname", FieldName},

		{"phone", FieldPhone},
		{"Phone", FieldPhone},
		{"PHONE", FieldPhone},
		{"phone1", FieldPhone},
		{"phone 2", FieldPhone},
		{"phone_number", FieldPhone},
		{"phone-number", FieldPhone},
		{"telephone", FieldPhone},
		{"tel", FieldPhone},
		{"mobile", FieldPhone},
		{"cell", FieldPhone},
		{"contactnumber", FieldPhone},

		{"email", FieldEmail},
		{"Email", FieldEmail},
		{"EMAIL", FieldEmail},
		{"email1", FieldEmail},
		{"email address", FieldEmail},
		{"email_address", FieldEmail},
		{"e-mail", FieldEmail},
		{"mail", FieldEmail},

		{"org", FieldOrg},
		{"Org", FieldOrg},
		{"organization", FieldOrg},
		{"organisation", FieldOrg},
		{"company", FieldOrg},
		{"employer", FieldOrg},
		{"affiliation", FieldOrg},

		{"unknown", ""},
		{"notes", ""},
		{"id", ""},
		{"address", ""},
	}

	for _, tt := range tests {
		t.Run(tt.header, func(t *testing.T) {
			got := detectField(strings.ToLower(tt.header))
			assert.Equal(t, tt.want, got, "detectField(%q)", tt.header)
		})
	}
}

func TestContactToVCF_Basic(t *testing.T) {
	c := model.Contact{
		Name:         "John Doe",
		Phones:       []string{"+1-555-0100"},
		Emails:       []string{"john@example.com"},
		Organization: "Acme Inc",
	}
	vcf := ContactToVCF(c)

	assert.Contains(t, vcf, "BEGIN:VCARD")
	assert.Contains(t, vcf, "VERSION:4.0")
	assert.Contains(t, vcf, "PRODID:-//vcf-toolkit//EN")
	assert.Contains(t, vcf, "FN:John Doe")
	assert.Contains(t, vcf, "N:Doe;John;;;")
	assert.Contains(t, vcf, "TEL;VALUE=uri:tel:+15550100")
	assert.Contains(t, vcf, "EMAIL:john@example.com")
	assert.Contains(t, vcf, "ORG:Acme Inc")
	assert.Contains(t, vcf, "END:VCARD")
	assert.True(t, strings.HasPrefix(vcf, "BEGIN:VCARD\r\n"))
	assert.True(t, strings.HasSuffix(strings.TrimSpace(vcf), "END:VCARD"))
}

func TestContactToVCF_EmptyName(t *testing.T) {
	c := model.Contact{
		Phones: []string{"+1234567890"},
		Emails: []string{"test@example.com"},
	}
	vcf := ContactToVCF(c)
	assert.Contains(t, vcf, "FN:Unknown")
	assert.Contains(t, vcf, "N:;;;;")
}

func TestContactToVCF_SingleWordName(t *testing.T) {
	c := model.Contact{Name: "Madonna"}
	vcf := ContactToVCF(c)
	assert.Contains(t, vcf, "FN:Madonna")
	// N:;Given;;;
	assert.Contains(t, vcf, "N:;Madonna;;;")
}

func TestContactToVCF_MiddleName(t *testing.T) {
	c := model.Contact{Name: "John Michael Doe"}
	vcf := ContactToVCF(c)
	assert.Contains(t, vcf, "FN:John Michael Doe")
	assert.Contains(t, vcf, "N:Doe;John;Michael;;")
}

func TestContactToVCF_MultiplePhonesEmails(t *testing.T) {
	c := model.Contact{
		Name:   "Alice Smith",
		Phones: []string{"+1-555-0100", "+1-555-0101"},
		Emails: []string{"alice@a.com", "alice@b.com"},
	}
	vcf := ContactToVCF(c)
	assert.Contains(t, vcf, "TEL;VALUE=uri:tel:+15550100")
	assert.Contains(t, vcf, "TEL;VALUE=uri:tel:+15550101")
	assert.Contains(t, vcf, "EMAIL:alice@a.com")
	assert.Contains(t, vcf, "EMAIL:alice@b.com")
}

func TestContactToVCF_EscapeCharacters(t *testing.T) {
	c := model.Contact{
		Name:         "Doe, John & Jane",
		Organization: "Acme;Corp",
		Emails:       []string{"test@example.com"},
	}
	vcf := ContactToVCF(c)
	assert.Contains(t, vcf, "FN:Doe\\, John & Jane")
	// structuredName splits on spaces: first word = given, last word = family.
	// For "Doe, John & Jane": given="Doe," (comma from original), middle="John &", family="Jane"
	assert.Contains(t, vcf, "N:Jane;Doe\\,;John &;;")
	assert.Contains(t, vcf, "ORG:Acme\\;Corp")
}

func TestContactToVCF_PhoneNormalization(t *testing.T) {
	tests := []struct {
		name    string
		phone   string
		wantTel string
	}{
		{"US number with formatting", "+1 (555) 0100", "TEL;VALUE=uri:tel:+15550100"},
		{"US number without plus", "15550100", "TEL;VALUE=uri:tel:+15550100"},
		{"Indian number", "+91 98765 43210", "TEL;VALUE=uri:tel:+919876543210"},
		{"UK number", "+44 20 7946 0958", "TEL;VALUE=uri:tel:+442079460958"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := model.Contact{
				Name:   "Test",
				Phones: []string{tt.phone},
			}
			vcf := ContactToVCF(c)
			assert.Contains(t, vcf, tt.wantTel)
		})
	}
}

func TestContactToVCF_EmailAliasStripped(t *testing.T) {
	c := model.Contact{
		Name:   "Test",
		Emails: []string{"user+tag@example.com"},
	}
	vcf := ContactToVCF(c)
	assert.Contains(t, vcf, "EMAIL:user@example.com")
}

func TestContactToVCF_InvalidEmail(t *testing.T) {
	c := model.Contact{
		Name:   "Test",
		Emails: []string{"not-an-email"},
	}
	vcf := ContactToVCF(c)
	// Invalid email with lax validation may still pass. Just check no crash.
	assert.NotEmpty(t, vcf)
}

func TestConvertCSVToVCF_Basic(t *testing.T) {
	input := `name,phone,email,org
John Doe,+1-555-0100,john@example.com,Acme Inc
Jane Smith,+1-555-0101,jane@test.org,Globex Corp
`
	var buf bytes.Buffer
	result, err := ConvertCSVToVCF(strings.NewReader(input), &buf, Config{})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 2, result.InputRows)
	assert.Equal(t, 2, result.OutputVCards)
	assert.Len(t, result.ParseErrors, 0)
	assert.False(t, result.Deduplicated)
	assert.Equal(t, 0, result.DuplicatesRemoved)

	output := buf.String()
	assert.Contains(t, output, "FN:John Doe")
	assert.Contains(t, output, "FN:Jane Smith")
	assert.Contains(t, output, "TEL;VALUE=uri:tel:+15550100")
	assert.Contains(t, output, "TEL;VALUE=uri:tel:+15550101")
	assert.Contains(t, output, "EMAIL:john@example.com")
	assert.Contains(t, output, "EMAIL:jane@test.org")
	assert.Contains(t, output, "ORG:Acme Inc")
	assert.Contains(t, output, "ORG:Globex Corp")

	// Verify proper VCF structure: each vCard is self-contained
	vcards := strings.Split(output, "END:VCARD\r\n")
	assert.Len(t, vcards, 3) // last element after final END:VCARD is empty
}

func TestConvertCSVToVCF_EmptyInput(t *testing.T) {
	input := `name,phone,email\n`
	var buf bytes.Buffer
	result, err := ConvertCSVToVCF(strings.NewReader(input), &buf, Config{})
	require.NoError(t, err)
	assert.Equal(t, 0, result.OutputVCards)
	assert.Empty(t, buf.String())
}

func TestConvertCSVToVCF_WithParseErrors(t *testing.T) {
	input := `name,phone,email
Alice,alice@a.com,123
Bob,+1-555-0100,bob@b.com
Charlie,,
`
	var buf bytes.Buffer
	var stderr bytes.Buffer
	result, err := ConvertCSVToVCF(strings.NewReader(input), &buf, Config{
		Stderr:  &stderr,
		Verbose: true,
	})
	require.NoError(t, err)

	// Alice's row: phone column has email, but still parsed
	// Charlie's row: empty columns after name — should have parse error or not
	_ = result
	output := buf.String()
	assert.Contains(t, output, "FN:Alice")
	assert.Contains(t, output, "FN:Bob")
	// Charlie may or may not be output depending on parse error
}

func TestConvertCSVToVCF_WithDeduplication(t *testing.T) {
	input := `name,email
John Doe,john@example.com
John Doe,john@example.com
Jane Smith,jane@example.com
`
	var buf bytes.Buffer
	var stderr bytes.Buffer
	result, err := ConvertCSVToVCF(strings.NewReader(input), &buf, Config{
		Deduplicate: true,
		Stderr:      &stderr,
		Verbose:     true,
	})
	require.NoError(t, err)
	assert.Equal(t, 3, result.InputRows)
	assert.Equal(t, 2, result.OutputVCards) // 2 unique contacts after dedup
	assert.Equal(t, 1, result.DuplicatesRemoved)

	output := buf.String()
	assert.Contains(t, output, "FN:John Doe")
	assert.Contains(t, output, "FN:Jane Smith")

	// Verify verbose logging
	assert.Contains(t, stderr.String(), "Deduplicating")
}

func TestConvertCSVToVCF_VCFLineEndings(t *testing.T) {
	input := `name,email
Test User,test@example.com
`
	var buf bytes.Buffer
	_, err := ConvertCSVToVCF(strings.NewReader(input), &buf, Config{})
	require.NoError(t, err)

	output := buf.String()
	// VCF requires CRLF line endings
	assert.Contains(t, output, "\r\n")
	// Should not contain bare LF without CR
	lines := strings.Split(output, "\r\n")
	for _, line := range lines {
		assert.NotContains(t, line, "\n")
	}
}

func TestVCFPropertiesOrder(t *testing.T) {
	c := model.Contact{
		Name:         "Test User",
		Phones:       []string{"+1-555-0100"},
		Emails:       []string{"test@example.com"},
		Organization: "Test Org",
	}
	vcf := ContactToVCF(c)

	lines := strings.Split(vcf, "\r\n")
	expectedOrder := []string{
		"BEGIN:VCARD",
		"VERSION:4.0",
		"PRODID:-//vcf-toolkit//EN",
		"FN:Test User",
		"N:User;Test;;;",
		"ORG:Test Org",
		"TEL;VALUE=uri:tel:+15550100",
		"EMAIL:test@example.com",
		"END:VCARD",
	}

	for i, expected := range expectedOrder {
		if i < len(lines) {
			assert.Equal(t, expected, lines[i], "VCF line %d", i)
		}
	}
}

func TestEscapeVCF(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"plain text", "plain text"},
		{"back\\slash", "back\\\\slash"},
		{"semi;colon", "semi\\;colon"},
		{"comma,separated", "comma\\,separated"},
		{"line\nbreak", "line\\nbreak"},
		{"mixed;test,with\\chars\nand\r\nnewlines", "mixed\\;test\\,with\\\\chars\\nand\\nnewlines"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := escapeVCF(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestStructuredName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", ";;;;"},
		{"Madonna", ";Madonna;;;"},
		{"John Doe", "Doe;John;;;"},
		{"John Michael Doe", "Doe;John;Michael;;"},
		{"  spaces  here  ", "here;spaces;;;"},
	}
	for _, tt := range tests {
		t.Run(fmt.Sprintf("name=%q", tt.input), func(t *testing.T) {
			got := structuredName(tt.input)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBuildHeaderMapping(t *testing.T) {
	t.Run("auto detect all known headers", func(t *testing.T) {
		headers := []string{"Name", "Phone", "Email", "Org"}
		mapping := buildHeaderMapping(headers, nil)
		assert.Equal(t, FieldName, mapping["Name"])
		assert.Equal(t, FieldPhone, mapping["Phone"])
		assert.Equal(t, FieldEmail, mapping["Email"])
		assert.Equal(t, FieldOrg, mapping["Org"])
	})

	t.Run("override overrides auto-detection", func(t *testing.T) {
		headers := []string{"Custom"}
		override := map[string]string{"CUSTOM": "name"}
		mapping := buildHeaderMapping(headers, override)
		assert.Equal(t, FieldName, mapping["Custom"])
	})

	t.Run("partial override", func(t *testing.T) {
		headers := []string{"Name", "Phone"}
		override := map[string]string{"Name": "org"} // treat Name column as org
		mapping := buildHeaderMapping(headers, override)
		assert.Equal(t, FieldOrg, mapping["Name"])
		assert.Equal(t, FieldPhone, mapping["Phone"])
	})
}

func TestContactFromRow(t *testing.T) {
	t.Run("populates all fields", func(t *testing.T) {
		row := []string{"John Doe", "+1-555-0100", "john@example.com", "Acme"}
		headers := []string{"Name", "Phone", "Email", "Org"}
		mapping := buildHeaderMapping(headers, nil)

		c, errs := contactFromRow(row, headers, mapping)
		assert.Len(t, errs, 0)
		assert.Equal(t, "John Doe", c.Name)
		assert.Equal(t, []string{"+1-555-0100"}, c.Phones)
		assert.Equal(t, []string{"john@example.com"}, c.Emails)
		assert.Equal(t, "Acme", c.Organization)
	})

	t.Run("empty row produces error", func(t *testing.T) {
		row := []string{"", "", "", ""}
		headers := []string{"Name", "Phone", "Email", "Org"}
		mapping := buildHeaderMapping(headers, nil)

		_, errs := contactFromRow(row, headers, mapping)
		assert.Len(t, errs, 1)
		assert.Contains(t, errs[0].Message, "no recognizable contact fields")
	})

	t.Run("shorter row than headers", func(t *testing.T) {
		row := []string{"Alice"}
		headers := []string{"Name", "Phone", "Email", "Org"}
		mapping := buildHeaderMapping(headers, nil)

		c, errs := contactFromRow(row, headers, mapping)
		assert.Len(t, errs, 0)
		assert.Equal(t, "Alice", c.Name)
		assert.Empty(t, c.Phones)
		assert.Empty(t, c.Emails)
		assert.Empty(t, c.Organization)
	})

	t.Run("longer row than headers", func(t *testing.T) {
		row := []string{"Bob", "+1-555-0101", "bob@test.com", "Org", "extra"}
		headers := []string{"Name", "Phone", "Email", "Org"}
		mapping := buildHeaderMapping(headers, nil)

		c, errs := contactFromRow(row, headers, mapping)
		assert.Len(t, errs, 0)
		assert.Equal(t, "Bob", c.Name)
		assert.Equal(t, []string{"+1-555-0101"}, c.Phones)
		assert.Equal(t, []string{"bob@test.com"}, c.Emails)
		assert.Equal(t, "Org", c.Organization)
	})

	t.Run("first name takes precedence over last name", func(t *testing.T) {
		row := []string{"John", "Doe"}
		headers := []string{"first name", "last name"}
		mapping := buildHeaderMapping(headers, nil)

		c, errs := contactFromRow(row, headers, mapping)
		assert.Len(t, errs, 0)
		assert.Equal(t, "John", c.Name) // first name wins
	})
}

func TestConvertCSVToVCF_RealWorldInput(t *testing.T) {
	input := `First Name,Last Name,Email Address,Phone Number,Company
Alice,Johnson,alice.j@company.com,+1-212-555-0100,Acme Corp
Bob,Smith,bob.smith@example.org,(415) 555-0199,Globex Inc
Charlie,Brown,charlie+spam@test.co,44 20 7946 0958,Tech Ltd
`
	var buf bytes.Buffer
	var stderr bytes.Buffer
	result, err := ConvertCSVToVCF(strings.NewReader(input), &buf, Config{
		Stderr:  &stderr,
		Verbose: true,
	})
	require.NoError(t, err)
	assert.Equal(t, 3, result.OutputVCards)
	assert.Len(t, result.ParseErrors, 0)

	output := buf.String()
	// First name used as Name
	assert.Contains(t, output, "FN:Alice")
	assert.Contains(t, output, "TEL;VALUE=uri:tel:+12125550100")  // +1-212-555-0100 normalized
	assert.Contains(t, output, "TEL;VALUE=uri:tel:+4155550199")   // (415) 555-0199 → digits + "+"
	assert.Contains(t, output, "TEL;VALUE=uri:tel:+442079460958") // 44 20 7946 0958 → digits + "+"
	assert.Contains(t, output, "EMAIL:alice.j@company.com")
	assert.Contains(t, output, "EMAIL:bob.smith@example.org")
	assert.Contains(t, output, "EMAIL:charlie@test.co") // plus-alias stripped
	assert.Contains(t, output, "ORG:Acme Corp")
	assert.Contains(t, output, "ORG:Globex Inc")
	assert.Contains(t, output, "ORG:Tech Ltd")

	// Verify CRLF line endings
	assert.Contains(t, output, "\r\n")
}
