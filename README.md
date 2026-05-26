# vcf-toolkit

`vcf-toolkit` is a Go-native toolkit for cleaning up contact data, deduplicating records, and converting CSV contacts into VCF/vCard output.

It ships as:

- a CLI for day-to-day use
- importable Go packages for SDK-style integration

The library is designed around deterministic, local processing by default. External LLM-based name normalization is opt-in.

## What it does

- Normalize names, phone numbers, and email addresses
- Detect and merge duplicate contacts
- Convert CSV contacts to vCard 4.0 (`.vcf`)
- Parse VCF back into CSV
- Provide a terminal UI for interactive workflows

## Requirements

- Go 1.24.2 or newer

## Install

### CLI

```bash
go install github.com/init0/vcf-toolkit@latest
```

This installs the `vcf-toolkit` binary if your `GOBIN` or `GOPATH/bin` is on `PATH`.

### From source

```bash
git clone https://github.com/init0/vcf-toolkit.git
cd vcf-toolkit
go build ./...
```

## CLI usage

Show help:

```bash
vcf-toolkit --help
vcf-toolkit convert --help
vcf-toolkit dedupe --help
vcf-toolkit normalize --help
vcf-toolkit tui
```

### Convert CSV to VCF

```bash
vcf-toolkit convert contacts.csv > contacts.vcf
cat contacts.csv | vcf-toolkit convert > contacts.vcf
vcf-toolkit convert contacts.csv --out contacts.vcf
vcf-toolkit convert contacts.csv --dedupe --out contacts.vcf
```

### Deduplicate CSV contacts

```bash
vcf-toolkit dedupe contacts.csv > deduped.csv
vcf-toolkit dedupe contacts.csv --json > dedupe-report.json
vcf-toolkit dedupe contacts.csv --dry-run
```

### Normalize CSV contacts

```bash
vcf-toolkit normalize contacts.csv > normalized.csv
vcf-toolkit normalize contacts.csv --json > normalized.json
```

### Interactive TUI

```bash
vcf-toolkit tui
```

## Go SDK usage

Import the public packages from the module root:

- [`convert`](./convert)
- [`dedupe`](./dedupe)
- [`normalize`](./normalize)
- [`vcf`](./vcf)
- [`model`](./model)

### Convert CSV to VCF

```go
package main

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/init0/vcf-toolkit/convert"
)

func main() {
	csvData := "name,phone,email,org\nAlice,+1 212 555 0101,alice@example.com,Acme\n"

	var out bytes.Buffer
	summary, err := convert.ConvertCSVToVCF(
		strings.NewReader(csvData),
		&out,
		convert.Config{
			Deduplicate: true,
		},
	)
	if err != nil {
		panic(err)
	}

	fmt.Println(out.String())
	fmt.Printf("contacts in: %d, vcards out: %d\n", summary.InputContacts, summary.OutputVCards)
}
```

### Deduplicate contacts

```go
package main

import (
	"fmt"

	"github.com/init0/vcf-toolkit/dedupe"
	"github.com/init0/vcf-toolkit/model"
)

func main() {
	contacts := []model.Contact{
		{Name: "Alice", Emails: []string{"alice@example.com"}},
		{Name: "Alice Smith", Emails: []string{"Alice@Example.com"}},
	}

	result := dedupe.Deduplicate(contacts, dedupe.DefaultConfig())
	fmt.Printf("clusters: %d, duplicates: %d\n", len(result.Clusters), result.Report.DuplicatesFound)
}
```

### Normalize primitives

```go
package main

import (
	"fmt"

	"github.com/init0/vcf-toolkit/normalize"
)

func main() {
	name := normalize.NormalizeName("John Doe Inc.", normalize.NameConfig{
		StripSuffix: true,
		OrgPatterns: []string{"Inc."},
	})

	phone := normalize.NormalizePhone("(212) 555-0101", normalize.PhoneConfig{
		DefaultCountry:   "+1",
		StrictValidation: true,
	})

	email := normalize.NormalizeEmail("User+tag@Example.com", normalize.EmailConfig{
		StripAliases: true,
	})

	fmt.Println(name.FullName)
	fmt.Println(phone.Normalized)
	fmt.Println(email.Normalized)
}
```

### Parse VCF

```go
package main

import (
	"fmt"
	"os"

	"github.com/init0/vcf-toolkit/vcf"
)

func main() {
	res, err := vcf.ParseVCF(os.Stdin)
	if err != nil {
		panic(err)
	}
	fmt.Println(len(res.Contacts))
}
```

## CSV input expectations

The converter and dedupe commands expect a header row. Common header variants are auto-detected.

Recognized fields include:

- name: `name`, `full name`, `first name`, `last name`, `displayname`
- phone: `phone`, `mobile`, `cell`, `telephone`
- email: `email`, `e-mail`, `email address`
- organization: `org`, `organization`, `company`

If your source uses custom headers, set a manual header mapping in the Go API:

```go
cfg := convert.Config{
	HeaderMapping: map[string]string{
		"Full Name": "name",
		"Primary Phone": "phone",
		"Work Email": "email",
	},
}
```

## Output formats

### VCF

- Writes vCard 4.0 records
- Supports names, phones, emails, and organization fields
- Preserves multiple phone and email values per contact

### CSV

- Normalized CSV output uses the columns `name`, `phone`, `email`, `org`
- Multiple values are joined with `; `

### JSON

- Available from the CLI for `normalize` and `dedupe`
- Useful for piping into other tools or for inspection

## Determinism and behavior

Default behavior is deterministic for the same input and configuration:

- parsing is sequential
- deduplication output order is stable
- VCF output preserves the processed contact order

External LLM-based name normalization is opt-in and not part of the default deterministic path. If you enable it, output quality and exact wording depend on your configured client.

## Deduplication model

The dedupe engine uses a weighted score across:

- exact email match, including alias normalization
- exact phone match after digit cleanup
- fuzzy name similarity using Jaro-Winkler
- organization match

Default settings are tuned for practical CSV cleanup, not strict identity resolution. For high-volume or high-risk data, validate results on a sample before using the output downstream.

## Phone normalization caveat

Phone handling is heuristic. It removes formatting characters, handles a subset of country codes, and produces a normalized comparable form. It is not a full international numbering library.

If you need strict telecom-grade validation, wrap this package with a dedicated phone-number library in your application.

## LLM-based name normalization

`normalize.HTTPNameLLM` lets you plug in a provider-agnostic HTTP endpoint that returns JSON name metadata.

Use this only when you explicitly want non-local normalization:

```go
cfg := normalize.NameConfig{
	UseLLM: true,
	LLMClient: &normalize.HTTPNameLLM{
		URL: "https://your-endpoint.example/normalize-name",
	},
}
```

If `UseLLM` is false, normalization stays local and deterministic.

## Development

Run tests:

```bash
GOCACHE=/tmp/vcf-toolkit-gocache go test ./...
```

Build the CLI:

```bash
go build -o bin/vcf-toolkit .
```

## Repository layout

- `cmd/`: CLI subcommands
- `convert/`, `dedupe/`, `normalize/`, `vcf/`, `model/`: public Go wrappers
- `internal/`: core implementation
- `tui/`: terminal UI
- `testdata/`: sample CSV and VCF fixtures

## Notes for consumers

- Import the public wrapper packages, not the `internal/...` packages.
- The root module is a CLI entrypoint, so the package at the repository root is `main`.
- If you need stable machine-to-machine output, keep LLM normalization disabled.

