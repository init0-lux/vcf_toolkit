# Product Requirements Document (PRD)

## vcf_toolkit

---

## 1. Overview

VCF Toolkit is a developer-first toolkit for cleaning, normalizing, deduplicating, and converting contact data across formats. It is designed primarily as an NPM package with a strong CLI interface, enabling engineers and operators to process messy contact datasets reliably and deterministically.

The product addresses a common but under-served problem: **contact data is messy, inconsistent, and fragmented**, especially in contexts like hackathons, college organizations, CRM imports, and event registrations.

VCF Toolkit provides:

* Deterministic normalization
* Explainable deduplication
* Seamless CSV → VCF conversion
* Extensible and composable APIs

---

## 2. Goals

### Primary Goals

1. Provide a **reliable contact normalization layer** for developers.
2. Enable **high-confidence deduplication with explainability**.
3. Offer a **frictionless CLI** for non-programmatic workflows.
4. Become a **default utility in hackathons and data ingestion pipelines**.

### Secondary Goals

1. Support global datasets with locale-aware normalization.
2. Enable extensibility via plugins and custom rules.
3. Maintain high performance for large datasets (10k–100k contacts).

---

## 3. Non-Goals

* Not a full CRM system
* Not a UI-heavy product (CLI-first)
* Not a cloud service (initially)
* Not focused on real-time streaming pipelines (batch-oriented)

---

## 4. Target Users

### Primary Users

* Developers working with user/contact datasets
* Hackathon participants
* Backend engineers building CRM or onboarding flows
* Growth/ops engineers handling CSV imports

### Secondary Users

* College clubs managing member databases
* Event organizers exporting/importing contacts
* Individuals migrating phone contacts

---

## 5. Key Use Cases

### Use Case 1: Hackathon Registration Cleanup

* Input: messy CSV with inconsistent names, duplicate entries
* Output: clean, deduplicated dataset

### Use Case 2: CSV to Phone Import

* Input: CSV from Google Forms
* Output: VCF file importable into phones

### Use Case 3: CRM Ingestion Pipeline

* Input: multiple CSVs from different sources
* Output: unified, deduplicated contact database

### Use Case 4: WhatsApp Contact Preparation

* Normalize phone numbers
* Remove duplicates
* Export to VCF

---

## 6. Functional Requirements

---

### 6.1 Name Normalization Module

#### Description

Processes raw name strings and extracts structured identity components.

#### Features

* Remove suffixes (e.g., “CSI VIT”, “IEEE”, “ACM”)
* Extract organization tags
* Normalize casing (Title Case)
* Unicode normalization (NFC/NFKC)
* Nickname expansion (optional)
* Strip noise (extra whitespace, punctuation)

#### Input

```ts
string
```

#### Output

```ts
{
  fullName: string;
  firstName?: string;
  lastName?: string;
  organization?: string;
  normalized: boolean;
}
```

#### Config Options

```ts
{
  stripSuffix?: boolean;
  orgPatterns?: string[];
  expandNicknames?: boolean;
  locale?: string;
}
```

---

### 6.2 Phone Normalization Module

#### Description

Standardizes phone numbers into E.164 format.

#### Features

* Country code inference (default: IN → +91)
* Remove formatting characters
* Validate number length and format
* Handle multiple phone formats

#### Input

```ts
string
```

#### Output

```ts
{
  raw: string;
  normalized: string; // +919876543210
  valid: boolean;
  country?: string;
}
```

#### Config

```ts
{
  defaultCountry?: string;
  strict?: boolean;
}
```

---

### 6.3 Email Normalization Module

#### Features

* Lowercasing
* Gmail dot removal (optional)
* Alias stripping (+tag)
* Validation

#### Output

```ts
{
  normalized: string;
  valid: boolean;
}
```

---

### 6.4 CSV → VCF Converter

#### Description

Converts structured CSV data into VCF format.

#### Features

* Header auto-mapping:

  * name, full_name → FN
  * phone, mobile → TEL
  * email → EMAIL
* Multi-value support (multiple phones/emails)
* Default fallbacks for missing fields
* Optional deduplication during conversion
* Error logging for invalid rows

#### CLI Example

```bash
npx aux-contacts csv2vcf input.csv -o contacts.vcf --dedupe
```

#### Output

* `.vcf` file compliant with vCard 3.0+

---

### 6.5 Deduplication Engine

#### Description

Identifies and merges duplicate contacts using multiple signals.

#### Matching Signals

| Signal       | Type  | Weight |
| ------------ | ----- | ------ |
| Email        | Exact | High   |
| Phone        | Exact | High   |
| Name         | Fuzzy | Medium |
| Organization | Fuzzy | Low    |

#### Features

* Fuzzy matching (Levenshtein / Jaro-Winkler)
* Confidence scoring
* Explainability (reasoning output)
* Configurable thresholds
* Cluster-based output (not just filtering)

#### Input

```ts
Contact[]
```

#### Output

```ts
{
  clusters: Contact[][];
  merged: Contact[];
  report: {
    duplicatesFound: number;
    confidenceDistribution: number[];
  }
}
```

#### Config

```ts
{
  threshold?: number; // default: 0.8
  strategy?: "safe" | "aggressive";
}
```

---

### 6.6 Contact Merge Logic

#### Rules

* Prefer non-null values
* Combine phone numbers and emails
* Preserve metadata
* Maintain source traceability

#### Output

```ts
MergedContact
```

---

### 6.7 CLI Interface

#### Commands

```bash
aux normalize <file>
aux dedupe <file>
aux convert <file> --to vcf
```

#### Flags

* `--out <file>`
* `--dry-run`
* `--verbose`
* `--json`
* `--dedupe`

#### Behavior

* Should not crash on invalid rows
* Logs errors with line references
* Supports piping

---

## 7. Non-Functional Requirements

### Performance

* Handle 10k contacts under 2 seconds
* Deduplication optimized via:

  * Hash maps (email/phone)
  * Blocking strategies

### Reliability

* Deterministic outputs
* No silent failures

### Extensibility

* Plugin system (v2)
* Config-driven rules

### Usability

* Minimal setup
* Sensible defaults (India-first)

---

## 8. Architecture

### Modules

```
/core
/normalize
/dedupe
/convert
/cli
/plugins (future)
```

### Design Principles

* Functional, stateless modules
* Composable APIs
* Strong typing (TypeScript-first)

---

## 9. API Design

### Example

```ts
import {
  normalizeName,
  normalizePhone,
  dedupeContacts,
  csvToVcf
} from "@aux/contacts";
```

---

## 10. Data Model

### Contact

```ts
type Contact = {
  id?: string;
  name?: string;
  phones?: string[];
  emails?: string[];
  organization?: string;
  metadata?: Record<string, any>;
};
```

---

## 11. Error Handling

* Invalid inputs → warnings, not crashes
* Structured error objects
* CLI logs:

  * row number
  * field issue
  * suggested fix

---

## 12. Metrics for Success

### Adoption Metrics

* Weekly downloads (NPM)
* CLI usage frequency
* GitHub stars

### Performance Metrics

* Deduplication accuracy (>90% expected)
* False positive rate (<5%)

---

## 13. Risks

| Risk                       | Mitigation                  |
| -------------------------- | --------------------------- |
| Incorrect deduplication    | Explainability + thresholds |
| Locale-specific edge cases | Configurable rules          |
| Performance degradation    | Blocking + indexing         |
| Over-complex API           | Strong defaults             |

---

## 14. Future Roadmap

### v1

* Normalization (name, phone, email)
* Basic dedupe
* CSV → VCF
* CLI

### v2

* Confidence scoring
* Explainability
* Plugin system
* Performance optimization

### v3

* Web UI
* API service
* CRM integrations

---

## 15. Competitive Landscape

Current tools:

* Basic CSV converters (limited)
* CRM tools (heavyweight, non-developer-friendly)
* Ad-hoc scripts (non-reusable)

### Differentiation

* Developer-first
* Deterministic dedupe
* CLI excellence
* India-first defaults

---

## 16. Open Questions

1. Should nickname expansion be enabled by default?
2. What should be the default dedupe threshold?
3. How aggressive should merging be in v1?
4. Should plugins be supported in v1 or deferred?

---

## 17. Appendix

### Example Flow

Input CSV:

```
Name,Phone,Email
Ojaswi Om - CSI VIT,9876543210,ojaswi@gmail.com
Ojaswi Om,+919876543210,ojaswi@gmail.com
```

Output:

* One merged contact
* Cleaned name
* Normalized phone
* Deduplicated entry

---
