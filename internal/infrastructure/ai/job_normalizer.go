package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/hunterjob/hunterjob/api/internal/company"
	"github.com/hunterjob/hunterjob/api/internal/crawler"
)

// NormalizeJobs converts one company's API/HTML response into the project's job schema.
// Structured output is used so provider-specific response shapes never leak into storage.
func (p OpenAICompatible) NormalizeJobs(ctx context.Context, source company.Source, response crawler.Response) ([]crawler.NormalizedJob, error) {
	endpoint, model, err := parseEndpoint(p.APIURL)
	if err != nil {
		return nil, err
	}

	stringArray := map[string]any{"type": "array", "items": map[string]any{"type": "string"}}
	location := map[string]any{
		"type": "object", "additionalProperties": false,
		"properties": map[string]any{
			"city": map[string]any{"type": "string"}, "country": map[string]any{"type": "string"}, "remote": map[string]any{"type": "boolean"},
		},
		"required": []string{"city", "country", "remote"},
	}
	experience := map[string]any{
		"type": "object", "additionalProperties": false,
		"properties": map[string]any{"min_years": map[string]any{"type": "integer"}, "max_years": map[string]any{"type": "integer"}},
		"required":   []string{"min_years", "max_years"},
	}
	jobSchema := map[string]any{
		"type": "object", "additionalProperties": false,
		"properties": map[string]any{
			"title": map[string]any{"type": "string"}, "normalized_title": map[string]any{"type": "string"},
			"locations": map[string]any{"type": "array", "items": location}, "levels": stringArray,
			"employment_type": map[string]any{"type": "string"}, "experience": experience, "skills": stringArray,
			"description": map[string]any{"type": "string"}, "original_url": map[string]any{"type": "string"},
			"apply_url": map[string]any{"type": "string"}, "expired_at": map[string]any{"type": []string{"string", "null"}},
		},
		"required": []string{"title", "normalized_title", "locations", "levels", "employment_type", "experience", "skills", "description", "original_url", "apply_url", "expired_at"},
	}
	schema := map[string]any{
		"type": "object", "additionalProperties": false,
		"properties": map[string]any{"jobs": map[string]any{"type": "array", "items": jobSchema}},
		"required":   []string{"jobs"},
	}

	input := fmt.Sprintf("Source URL: %s\nContent-Type: %s\nResponse body:\n%s", response.URL, response.ContentType, response.Body)
	payload := map[string]any{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": "Extract every real job from this single company's response. Return only facts present in the response. Keep URLs absolute or source-relative. Use RFC3339 for expired_at; use null when no expiration is provided. An empty jobs array is valid."},
			{"role": "user", "content": input},
		},
		"response_format": map[string]any{"type": "json_schema", "json_schema": map[string]any{"name": "normalized_jobs", "strict": true, "schema": schema}},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+p.APIKey)
	req.Header.Set("Content-Type", "application/json")
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode/100 != 2 {
		return nil, fmt.Errorf("AI job normalization failed: %s", res.Status)
	}
	var completion struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(res.Body).Decode(&completion); err != nil {
		return nil, err
	}
	if len(completion.Choices) == 0 || completion.Choices[0].Message.Content == "" {
		return nil, fmt.Errorf("AI API returned no normalized jobs")
	}
	var output struct {
		Jobs []crawler.NormalizedJob `json:"jobs"`
	}
	if err := json.Unmarshal([]byte(completion.Choices[0].Message.Content), &output); err != nil {
		return nil, fmt.Errorf("invalid normalized jobs: %w", err)
	}
	return output.Jobs, nil
}
