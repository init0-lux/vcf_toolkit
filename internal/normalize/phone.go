package normalize

import (
	"regexp"
	"strings"

	"github.com/init0/vcf-toolkit/internal/model"
)

type PhoneConfig struct {
	DefaultCountry   string
	StrictValidation bool
}

func NormalizePhone(raw string, cfg PhoneConfig) model.NormalizedPhone {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return model.NormalizedPhone{Raw: raw, Valid: false}
	}

	// Remove non-numeric characters
	cleaned := removeFormatting(raw)

	// Validate and normalize phone number
	normalized, valid := validateAndFormatPhone(cleaned, cfg)

	return model.NormalizedPhone{
		Raw:        raw,
		Normalized: normalized,
		Valid:      valid,
		Country:    cfg.DefaultCountry,
	}
}

func removeFormatting(phone string) string {
	re := regexp.MustCompile(`\D`)
	return re.ReplaceAllString(phone, "")
}

func validateAndFormatPhone(phone string, cfg PhoneConfig) (string, bool) {
	// check length of the phone number
	if len(phone) < 10 || len(phone) > 15 {
		return "", false
	}

	// Add default country code if missing
	if cfg.DefaultCountry != "" && !strings.HasPrefix(phone, "+") {
		phone = cfg.DefaultCountry + phone
	}

	// normalization to E.164 format
	if strings.HasPrefix(phone, "+") {
		return phone, true
	}

	return "", false
}
