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
		normalized, err := callLLMForNameNormalization(raw)
		if err == nil {
			return normalized
		}
	}

	return ruleBasedNameNormalization(raw, cfg)
}

func callLLMForNameNormalization(raw string) (model.NormalizedName, error) {
	// Example API call payload
	payload := map[string]string{"name": raw}

	// API call to the LLM service
	response, err := llmAPIClient.Call("normalizeName", payload)
	if err != nil {
		return model.NormalizedName{}, err
	}

	// Parse the response into the NormalizedName structure
	normalized := model.NormalizedName{
		FullName:     response["fullName"],
		FirstName:    response["firstName"],
		LastName:     response["lastName"],
		Organization: response["organization"],
		Normalized:   true,
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
