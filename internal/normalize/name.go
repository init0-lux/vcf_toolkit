package normalize

import (
	"strings"

	"github.com/init0/vcf-toolkit/internal/model"
)

type NameConfig struct {
	StripSuffix bool
	OrgPatterns []string
	UseLLM      bool
}

func NormalizeName(raw string, cfg NameConfig) model.NormalizedName {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return model.NormalizedName{Normalized: false}
	}

	if cfg.UseLLM {
		if normalized, err := callLLMForNameNormalization(raw); err == nil {
			return normalized
		}
	}

	parts := strings.Fields(raw)
	var organization string

	if cfg.StripSuffix {
		for _, pattern := range cfg.OrgPatterns {
			if strings.HasSuffix(raw, pattern) {
				organization = pattern
				raw = strings.TrimSuffix(raw, pattern)
				raw = strings.TrimSpace(raw)
				parts = strings.Fields(raw)
				break
			}
		}
	}

	if len(parts) == 0 {
		return model.NormalizedName{
			FullName:     raw,
			Organization: organization,
			Normalized:   false,
		}
	}

	if len(parts) == 1 {
		return model.NormalizedName{
			FullName:     raw,
			FirstName:    parts[0],
			Organization: organization,
			Normalized:   true,
		}
	}

	return model.NormalizedName{
		FullName:     raw,
		FirstName:    parts[0],
		LastName:     parts[len(parts)-1],
		Organization: organization,
		Normalized:   true,
	}
}

// callLLMForNameNormalization is a stub for future LLM integration.
// Currently returns Normalized: false to trigger rule-based fallback.
func callLLMForNameNormalization(raw string) (model.NormalizedName, error) {
	return model.NormalizedName{
		FullName:   raw,
		Normalized: false,
	}, nil
}

// ruleBasedNameNormalization applies title-casing and suffix stripping.
// Deprecated: use NormalizeName with UseLLM=false instead.
func ruleBasedNameNormalization(raw string, cfg NameConfig) model.NormalizedName {
	normalized := model.NormalizedName{
		FullName:   strings.Title(raw),
		Normalized: true,
	}

	if cfg.StripSuffix {
		for _, pattern := range cfg.OrgPatterns {
			if strings.HasSuffix(normalized.FullName, pattern) {
				normalized.Organization = pattern
				normalized.FullName = strings.TrimSuffix(normalized.FullName, pattern)
				break
			}
		}
	}

	return normalized
}
