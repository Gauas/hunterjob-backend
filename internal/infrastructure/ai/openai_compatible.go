package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/hunterjob/hunterjob/api/internal/application/search"
)

// OpenAICompatible implements the Chat Completions protocol used by OpenAI-compatible APIs.
// The model is part of AI_API_URL as a query parameter: ?model=<provider-model-id>.
// This keeps provider configuration limited to AI_API_URL and AI_API_KEY.
type OpenAICompatible struct {
	APIURL string
	APIKey string
	Client *http.Client
}

func (p OpenAICompatible) ParseSearchQuery(ctx context.Context, query string) (search.Intent, error) {
	endpoint, model, err := parseEndpoint(p.APIURL)
	if err != nil {
		return search.Intent{}, err
	}

	schema := map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"properties": map[string]any{
			"roles":          map[string]any{"type": "array", "items": map[string]string{"type": "string"}},
			"locations":      map[string]any{"type": "array", "items": map[string]string{"type": "string"}},
			"levels":         map[string]any{"type": "array", "items": map[string]string{"type": "string"}},
			"skills":         map[string]any{"type": "array", "items": map[string]string{"type": "string"}},
			"experience_max": map[string]string{"type": "integer"},
		},
		"required": []string{"roles", "locations", "levels", "skills", "experience_max"},
	}

	payload := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": "Extract job-search filters. Never infer facts not present in the query."},
			{"role": "user", "content": query},
		},
		"response_format": map[string]any{
			"type": "json_schema",
			"json_schema": map[string]any{
				"name":   "search_intent",
				"strict": true,
				"schema": schema,
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return search.Intent{}, err
	}

	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return search.Intent{}, err
	}
	req.Header.Set("Authorization", "Bearer "+p.APIKey)
	req.Header.Set("Content-Type", "application/json")

	res, err := client.Do(req)
	if err != nil {
		return search.Intent{}, err
	}
	defer res.Body.Close()
	if res.StatusCode/100 != 2 {
		return search.Intent{}, fmt.Errorf("AI API request failed: %s", res.Status)
	}

	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err = json.NewDecoder(res.Body).Decode(&response); err != nil {
		return search.Intent{}, err
	}
	if len(response.Choices) == 0 || response.Choices[0].Message.Content == "" {
		return search.Intent{}, fmt.Errorf("AI API returned no structured output")
	}

	var intent search.Intent
	if err = json.Unmarshal([]byte(response.Choices[0].Message.Content), &intent); err != nil {
		return search.Intent{}, fmt.Errorf("invalid AI search intent: %w", err)
	}
	return intent, nil
}

func parseEndpoint(raw string) (endpoint, model string, err error) {
	if raw == "" {
		return "", "", fmt.Errorf("AI_API_URL is not configured")
	}
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme != "https" || parsed.Host == "" {
		return "", "", fmt.Errorf("AI_API_URL must be an HTTPS endpoint")
	}
	model = parsed.Query().Get("model")
	if model == "" {
		return "", "", fmt.Errorf("AI_API_URL must include ?model=<model-id>")
	}
	parsed.RawQuery = ""
	return parsed.String(), model, nil
}
