# Product Requirements Document (PRD)

## Product Name

**vcf-toolkit**

---

## 1. Overview

vcf-toolkit is a developer-first toolkit for processing, normalizing, deduplicating, and converting contact data. It is designed as a **Go-native SDK and CLI**, with additional distribution layers (including an NPM wrapper) to maximize accessibility across ecosystems.

The core problem addressed is the **inconsistency and fragmentation of contact data** across sources such as CSV exports, registration forms, CRM systems, and manually curated datasets. These datasets frequently contain:

- Inconsistent naming formats
- Duplicate entries
- Invalid or unstandardized phone numbers
- Poorly structured fields
- Missing or ambiguous metadata

vcf-toolkit provides a deterministic, composable, and scriptable solution for transforming such data into clean, standardized, and deduplicated outputs suitable for downstream systems or direct usage (e.g., mobile contact imports).

---

## 2. Product Positioning

vcf-toolkit is positioned as:

> A low-level, reliable data processing tool for contact datasets, designed for engineers and power users.

It is not a GUI application, nor a CRM system. It operates as:

- A Go SDK for integration into backend systems
- A CLI tool for direct data processing
- A thin NPM-distributed binary wrapper for accessibility

---

## 3. Goals

### 3.1 Primary Goals

1. Provide a **deterministic and reliable contact normalization system**.
2. Enable **high-confidence deduplication with transparent reasoning**.
3. Offer a **Unix-style CLI interface** that integrates seamlessly into pipelines.
4. Serve as a **Go-native SDK** for integration into other applications.
5. Provide **frictionless distribution across ecosystems** (Go, binary, NPM wrapper).

---

### 3.2 Secondary Goals

1. Support large datasets (10k–100k contacts) efficiently.
2. Enable extensibility through configurable rules and future plugin systems.
3. Provide sensible defaults optimized for real-world usage (e.g., phone normalization).
4. Ensure consistent behavior across platforms.

---

## 4. Non-Goals

- Not a graphical application
- Not an interactive-first tool
- Not a cloud-hosted service
- Not a CRM or contact management platform
- Not a real-time streaming processor

---

## 5. Target Users

### 5.1 Primary Users

- Backend engineers processing user/contact data
- Developers building CRM or onboarding systems
- Hackathon participants handling registration data
- Operators managing CSV-based datasets

### 5.2 Secondary Users

- Student organizations managing member lists
- Event organizers exporting/importing contacts
- Individuals performing bulk contact cleanup

---

## 6. Core Use Cases

### 6.1 Contact Dataset Cleanup

Input: CSV with inconsistent formatting and duplicates
Output: Clean, normalized, deduplicated dataset

---

### 6.2 CSV to VCF Conversion

Input: CSV file from forms or exports
Output: Valid `.vcf` file importable into mobile devices

---

### 6.3 CRM Data Ingestion

Input: Multiple contact datasets from different sources
Output: Unified and deduplicated contact list

---

### 6.4 Pipeline Integration

Usage within scripts and pipelines for automated processing:

```bash
cat contacts.csv | vcf-toolkit dedupe > clean.csv
```

---

## 7. Functional Requirements

---

### 7.1 Name Normalization

#### Description

Transforms raw name strings into structured and standardized formats.

#### Capabilities

- Remove suffixes (e.g., organization tags)
- Extract organization identifiers
- Normalize casing (Title Case)
- Trim whitespace and noise
- Unicode normalization
- Optional nickname expansion

#### Input

- Raw string

#### Output

- Structured name object including:
  - Full name
  - Optional first/last name
  - Extracted organization (if present)
  - Normalization status

#### Configurability

- Enable/disable suffix stripping
- Provide custom organization patterns
- Locale-aware formatting options

---

### 7.2 Phone Number Normalization

#### Description

Standardizes phone numbers into a canonical format.

#### Capabilities

- Remove formatting characters
- Normalize to international format (E.164)
- Validate number structure
- Infer country code where missing

#### Output

- Normalized phone number
- Validation status
- Optional country metadata

#### Configurability

- Default country
- Strict validation mode

---

### 7.3 Email Normalization

#### Description

Standardizes email addresses for comparison and storage.

#### Capabilities

- Lowercasing
- Alias stripping (e.g., `+tag`)
- Provider-specific normalization (optional)
- Validation

#### Output

- Normalized email
- Validation status

---

### 7.4 CSV to VCF Conversion

#### Description

Converts CSV contact data into VCF format.

#### Capabilities

- Automatic header mapping
- Support for multiple phone/email fields
- Fallback handling for missing names
- Optional deduplication during conversion
- Error logging for malformed rows

#### Output

- Valid `.vcf` file compliant with vCard standards

---

### 7.5 Deduplication Engine

#### Description

Identifies and groups duplicate contacts using multiple signals.

#### Matching Signals

- Email (exact match)
- Phone (normalized exact match)
- Name (fuzzy matching)
- Organization (optional signal)

#### Capabilities

- Confidence scoring
- Configurable thresholds
- Cluster-based output (grouping duplicates)
- Deterministic matching behavior
- Explainability via reason tracing

#### Output

- Clusters of related contacts
- Merged contact list (optional)
- Deduplication report

---

### 7.6 Contact Merge Logic

#### Description

Combines duplicate contacts into a single canonical representation.

#### Rules

- Prefer non-null fields
- Merge phone numbers and emails
- Preserve metadata
- Maintain traceability of sources

---

### 7.7 CLI Interface

#### Description

Provides a Unix-style command-line interface.

#### Requirements

- Fully non-interactive by default
- Comprehensive `--help` and subcommand help
- Support for piping and redirection
- Deterministic output

#### Example Commands

```bash
vcf-toolkit normalize contacts.csv
vcf-toolkit dedupe contacts.csv --out clean.csv
vcf-toolkit convert contacts.csv --to vcf
```

#### Flags

- `--out`
- `--json`
- `--dry-run`
- `--verbose`
- `--dedupe`

---

### 7.8 SDK Usage (Go)

#### Description

Expose all functionality as a Go module.

#### Requirements

- Importable via `go get`
- Stable and minimal API surface
- No side effects (no logging, no global state)
- Deterministic outputs

#### Usage Contexts

- Backend services
- Data pipelines
- Internal tooling

---

## 8. Distribution Requirements

vcf-toolkit must support multiple distribution channels while maintaining a single source of truth.

### 8.1 Primary Distribution

- Go module (`go get`)

### 8.2 Binary Distribution

- Precompiled static binaries
- Hosted via release artifacts

### 8.3 NPM Wrapper

- Thin wrapper that:
  - Detects platform
  - Downloads appropriate binary
  - Caches locally
  - Executes transparently

#### Purpose

- Lower barrier for JavaScript ecosystem users
- Provide zero-install experience

---

## 9. Non-Functional Requirements

### 9.1 Performance

- Efficient processing of large datasets (≥10k contacts)
- Optimized deduplication (avoid naive O(n²) where possible)

---

### 9.2 Reliability

- Deterministic behavior across runs
- No silent failures
- Clear error reporting

---

### 9.3 Usability

- Minimal setup
- Clear CLI interface
- Strong documentation

---

### 9.4 Extensibility

- Config-driven behavior
- Future plugin support

---

### 9.5 Portability

- Consistent behavior across operating systems
- Minimal dependency footprint

---

## 10. Data Model

### Contact Structure

Fields include:

- Name
- Phone numbers (multiple)
- Emails (multiple)
- Organization
- Metadata

The model must support:

- Partial data
- Multiple values per field
- Extensibility

---

## 11. Error Handling

### Requirements

- Invalid rows must not crash execution
- Errors should include:
  - Location (row/field)
  - Reason
  - Suggested correction (if possible)

### CLI Behavior

- Errors printed to stderr
- Optional verbose/debug modes

---

## 12. Metrics for Success

### Adoption

- Number of downloads (binary + NPM wrapper)
- Go module usage

### Performance

- Processing time for standard datasets
- Deduplication accuracy

### Reliability

- Error rates in real-world datasets
- Stability across versions

---

## 13. Risks

| Risk                           | Mitigation                              |
| ------------------------------ | --------------------------------------- |
| Incorrect deduplication        | Configurable thresholds, explainability |
| Data loss during merge         | Conservative merge strategy             |
| Cross-platform inconsistencies | Extensive testing                       |
| Distribution complexity        | Unified release pipeline                |

---

## 14. Roadmap

### v1

- Core normalization (name, phone, email)
- Basic deduplication
- CSV → VCF conversion
- CLI interface
- Go SDK

### v2

- Confidence scoring improvements
- Explainability enhancements
- Performance optimizations

### v3

- Plugin system
- Advanced configuration
- Additional format support

---

## 15. Open Questions

1. Default deduplication threshold values
2. Level of aggressiveness in merging
3. Scope of provider-specific email normalization
4. Extent of locale-specific name handling

---

## 16. Summary

vcf-toolkit is a systems-oriented tool focused on solving a specific, recurring problem in data processing workflows. Its design prioritizes determinism, composability, and usability across environments, while maintaining a strong foundation as a Go-native SDK and CLI utility.

The product’s success depends on its ability to remain simple, predictable, and robust, while offering enough flexibility to handle real-world variability in contact data.
