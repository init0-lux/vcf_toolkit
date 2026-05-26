package normalize

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/init0-lux/vcf-toolkit/internal/model"
)

// HTTPNameLLM is a provider-agnostic client that calls an HTTP endpoint and
// expects a JSON response containing:
//   - full_name (required)
//   - first_name
//   - last_name
//   - organization
//
// It is intentionally generic so callers can target Gemini/Claude/OpenAI/etc
// via their own gateway service or direct API calls with a suitable request
// template.
type HTTPNameLLM struct {
	URL     string
	Method  string
	Headers map[string]string
	Timeout time.Duration

	// BodyTemplate is JSON that can include the literal substring "{input}".
	// That substring will be replaced with a JSON string containing the raw
	// input (so it stays valid JSON).
	BodyTemplate string

	Client *http.Client
}

type httpNameResult struct {
	FullName     string `json:"full_name"`
	FirstName    string `json:"first_name"`
	LastName     string `json:"last_name"`
	Organization string `json:"organization"`
}

func (c *HTTPNameLLM) NormalizeName(ctx context.Context, raw string) (model.NormalizedName, error) {
	if c == nil {
		return model.NormalizedName{}, errors.New("nil http llm client")
	}
	if strings.TrimSpace(c.URL) == "" {
		return model.NormalizedName{}, errors.New("http llm url is required")
	}
	method := c.Method
	if method == "" {
		method = http.MethodPost
	}
	timeout := c.Timeout
	if timeout == 0 {
		timeout = 15 * time.Second
	}

	body, err := buildHTTPBody(c.BodyTemplate, raw)
	if err != nil {
		return model.NormalizedName{}, err
	}

	client := c.Client
	if client == nil {
		client = http.DefaultClient
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, method, c.URL, bytes.NewReader(body))
	if err != nil {
		return model.NormalizedName{}, err
	}
	for k, v := range c.Headers {
		req.Header.Set(k, v)
	}
	if req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(req)
	if err != nil {
		return model.NormalizedName{}, err
	}
	defer resp.Body.Close()

	b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return model.NormalizedName{}, fmt.Errorf("http llm status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}

	var out httpNameResult
	if err := json.Unmarshal(b, &out); err != nil {
		return model.NormalizedName{}, err
	}

	full := strings.TrimSpace(out.FullName)
	if full == "" {
		return model.NormalizedName{}, errors.New("http llm returned empty full_name")
	}

	return model.NormalizedName{
		FullName:     full,
		FirstName:    strings.TrimSpace(out.FirstName),
		LastName:     strings.TrimSpace(out.LastName),
		Organization: strings.TrimSpace(out.Organization),
		Normalized:   true,
	}, nil
}

func buildHTTPBody(template string, input string) ([]byte, error) {
	if strings.TrimSpace(template) == "" {
		// Default, provider-agnostic shape.
		return json.Marshal(map[string]string{"input": input})
	}

	escaped, err := json.Marshal(input)
	if err != nil {
		return nil, err
	}
	s := strings.ReplaceAll(template, "{input}", string(escaped))
	return []byte(s), nil
}
