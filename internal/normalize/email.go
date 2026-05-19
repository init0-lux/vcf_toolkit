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

func stripAlias(email string) string {
	atIndex := strings.Index(email, "@")
	if atIndex == -1 {
		return email
	}

	localPart := email[:atIndex]
	domain := email[atIndex:]

	// remove alias ("+spam" in "user+spam@example.com")
	plusIndex := strings.Index(localPart, "+")
	if plusIndex != -1 {
		localPart = localPart[:plusIndex]
	}

	return localPart + domain
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
