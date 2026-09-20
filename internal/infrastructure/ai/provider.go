package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/hunterjob/hunterjob/api/internal/application/search"
	"net/http"
	"strings"
	"time"
)

type Deterministic struct{}

func (Deterministic) ParseSearchQuery(_ context.Context, q string) (search.Intent, error) {
	l := strings.ToLower(q)
	in := search.Intent{}
	for _, v := range []string{"backend engineer", "software engineer", "devops", "frontend engineer", "data engineer"} {
		if strings.Contains(l, strings.Split(v, " ")[0]) {
			in.Roles = append(in.Roles, v)
		}
	}
	for _, v := range []string{"da nang", "hanoi", "ha noi", "ho chi minh", "remote"} {
		if strings.Contains(l, v) {
			in.Locations = append(in.Locations, v)
		}
	}
	for _, v := range []string{"intern", "fresher", "junior", "senior"} {
		if strings.Contains(l, v) {
			in.Levels = append(in.Levels, v)
		}
	}
	for _, v := range []string{"Go", "Java", "Docker", "Kubernetes", "PostgreSQL", "AWS", "React", "Python"} {
		if strings.Contains(l, strings.ToLower(v)) {
			in.Skills = append(in.Skills, v)
		}
	}
	if strings.Contains(l, "0-2") || strings.Contains(l, "two years") {
		in.ExperienceMax = 2
	}
	return in, nil
}

type OpenAI struct {
	Key, Model string
	Client     *http.Client
}

func (p OpenAI) ParseSearchQuery(ctx context.Context, q string) (search.Intent, error) {
	if p.Key == "" {
		return search.Intent{}, fmt.Errorf("OPENAI_API_KEY is not configured")
	}
	schema := map[string]any{"type": "object", "additionalProperties": false, "properties": map[string]any{"roles": map[string]any{"type": "array", "items": map[string]string{"type": "string"}}, "locations": map[string]any{"type": "array", "items": map[string]string{"type": "string"}}, "levels": map[string]any{"type": "array", "items": map[string]string{"type": "string"}}, "skills": map[string]any{"type": "array", "items": map[string]string{"type": "string"}}, "experience_max": map[string]string{"type": "integer"}}, "required": []string{"roles", "locations", "levels", "skills", "experience_max"}}
	body := map[string]any{"model": p.Model, "store": false, "input": "Extract job-search filters from this Vietnamese or English query. Return only the schema fields. Query: " + q, "text": map[string]any{"format": map[string]any{"type": "json_schema", "name": "search_intent", "strict": true, "schema": schema}}}
	raw, e := json.Marshal(body)
	if e != nil {
		return search.Intent{}, e
	}
	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	req, e := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.openai.com/v1/responses", bytes.NewReader(raw))
	if e != nil {
		return search.Intent{}, e
	}
	req.Header.Set("Authorization", "Bearer "+p.Key)
	req.Header.Set("Content-Type", "application/json")
	res, e := client.Do(req)
	if e != nil {
		return search.Intent{}, e
	}
	defer res.Body.Close()
	if res.StatusCode/100 != 2 {
		return search.Intent{}, fmt.Errorf("OpenAI request failed: %s", res.Status)
	}
	var response struct {
		Output []struct {
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"output"`
	}
	if e = json.NewDecoder(res.Body).Decode(&response); e != nil {
		return search.Intent{}, e
	}
	for _, out := range response.Output {
		for _, c := range out.Content {
			if c.Text != "" {
				var in search.Intent
				e = json.Unmarshal([]byte(c.Text), &in)
				return in, e
			}
		}
	}
	return search.Intent{}, fmt.Errorf("OpenAI returned no structured output")
}
