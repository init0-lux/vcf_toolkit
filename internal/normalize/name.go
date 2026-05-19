package normalize

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/init0/vcf-toolkit/internal/model"
)

var ErrNoLLMClient = errors.New("llm enabled but no LLM client configured")

// NameLLM is a provider-agnostic interface for name normalization.
// Implementations can wrap OpenAI, Gemini, Claude, local models, etc.
type NameLLM interface {
	NormalizeName(ctx context.Context, raw string) (model.NormalizedName, error)
}

type NameConfig struct {
	StripSuffix bool
	OrgPatterns []string
	UseLLM      bool

	// LLMClient is optional; when set and UseLLM=true, it will be used to
	// normalize names. If nil, normalization falls back to rule-based behavior.
	LLMClient NameLLM
}

func NormalizeName(raw string, cfg NameConfig) model.NormalizedName {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return model.NormalizedName{Normalized: false}
	}

	// If LLM mode is enabled, prefer the configured client but always fall back
	// to deterministic rule-based normalization when unavailable or untrusted.
	if cfg.UseLLM {
		// Back-compat for CLI environments: allow presence of an API key env var
		// to signal that a client is likely configured elsewhere.
		if cfg.LLMClient != nil || strings.TrimSpace(os.Getenv("OPENAI_API_KEY")) != "" {
			if normalized, err := callLLMForNameNormalization(raw, cfg); err == nil && normalized.Normalized && normalized.FullName != "" {
				return normalized
			}
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

func callLLMForNameNormalization(raw string, cfg NameConfig) (model.NormalizedName, error) {
	if cfg.LLMClient == nil {
		return model.NormalizedName{}, ErrNoLLMClient
	}
	return cfg.LLMClient.NormalizeName(context.Background(), raw)
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
