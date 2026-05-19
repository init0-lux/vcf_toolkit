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
	parts := strings.Fields(raw)
	if raw == "" {
		return model.NormalizedName{FullName: raw, Normalized: false}
	}

	if cfg.UseLLM {
		normalized, err := callLLMForNameNormalization(raw)
		if err == nil {
			return normalized
		}
	}

	organization := ""
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

	if len(parts) == 1 {
		return model.NormalizedName{
			FullName:     raw,
			FirstName:    parts[0],
			Organization: organization,
			Normalized:   true,
		}
	} else if len(parts) > 1 {
		return model.NormalizedName{
			FullName:     raw,
			FirstName:    parts[0],
			LastName:     parts[len(parts)-1],
			Organization: organization,
			Normalized:   true,
		}
	}
	return model.NormalizedName{FullName: raw, Organization: organization, Normalized: false}
}

func callLLMForNameNormalization(raw string) (model.NormalizedName, error) {
	// api call to be implemented
	return model.NormalizedName{}, nil

	// parse the response into the NormalizedName structure
	normalized := model.NormalizedName{

		Normalized: true,
	}

	return normalized, nil
}

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
