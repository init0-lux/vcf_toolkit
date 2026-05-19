package normalize

import (
	"net/mail"
	"regexp"
	"strings"

	"github.com/init0/vcf-toolkit/internal/model"
)

type EmailConfig struct {
	StripAliases     bool
	StrictValidation bool
}

func NormalizeEmail(raw string, cfg EmailConfig) model.NormalizedEmail {
	rawSuffix := raw
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return model.NormalizedEmail{Raw: rawSuffix, Valid: false}
	}

	// lowercase and strip aliases
	normalized := strings.ToLower(raw)
	if cfg.StripAliases {
		normalized = stripAlias(normalized)
	}

	// validate email format
	valid := validateEmail(normalized, cfg)

	if !valid {
		return model.NormalizedEmail{
			Raw:   rawSuffix,
			Valid: false,
		}
	}

	return model.NormalizedEmail{
		Raw:        rawSuffix,
		Normalized: normalized,
		Valid:      valid,
	}
}

// NormalizeEmailStr normalizes an email to a comparable string:
// lowercase, plus - alias stripped.
// returns empty string if invalid.
// helper for dedupe
func NormalizeEmailStr(email string) string {
	email = strings.TrimSpace(strings.ToLower(email))
	idx := strings.Index(email, "@")
	if idx < 0 {
		return ""
	}
	local := email[:idx]
	domain := email[idx+1:]
	if plus := strings.Index(local, "+"); plus >= 0 {
		local = local[:plus]
	}
	return local + "@" + domain
}

func stripAlias(email string) string {
	return NormalizeEmailStr(email)
}

func validateEmail(email string, cfg EmailConfig) bool {
	if cfg.StrictValidation {
		_, err := mail.ParseAddress(email)
		return err == nil
	}

	// basic regex validation for leniency
	re := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return re.MatchString(email)
}
