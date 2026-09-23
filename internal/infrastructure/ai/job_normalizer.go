package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hunterjob/hunterjob/api/internal/company"
	"github.com/hunterjob/hunterjob/api/internal/crawler"
)

const normalizationBatchSize = 2

const normalizationPrompt = `Extract every active job from this single company's response and return one JSON object with a "jobs" array. Return JSON only.
Each job must contain exactly these fields:
- source_job_id: string; copy the provider's stable job id/key, or "" when absent
- title: string
- normalized_title: string
- locations: array of {city:string,country:string,remote:boolean}
- levels: array of strings
- employment_type: string
- experience: {min_years:integer,max_years:integer}; use 0 when unknown
- skills: array of strings
- description: string
- original_url: absolute or source-relative job detail URL, or "" when absent
- apply_url: absolute or source-relative application URL, or "" when absent
- expired_at: RFC3339 string or null
Use only facts present in the response. Never invent a URL. Exclude records marked expired. An empty jobs array is valid.`

// NormalizeJobs converts one company's API/HTML response into the project's job schema.
// JSON item lists are sent in small batches so large descriptions cannot monopolize one
// model request. The configured gateway supports json_object but not strict json_schema.
func (p OpenAICompatible) NormalizeJobs(ctx context.Context, source company.Source, response crawler.Response) ([]crawler.NormalizedJob, error) {
	endpoint, model, err := parseEndpoint(p.APIURL)
	if err != nil {
		return nil, err
	}
	batches := normalizationBatches(response.Body, normalizationBatchSize)
	if len(batches) == 0 {
		return []crawler.NormalizedJob{}, nil
	}
	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: 60 * time.Second}
	}
	jobs := make([]crawler.NormalizedJob, 0)
	for index, batch := range batches {
		batchJobs, batchErr := p.normalizeJobBatch(ctx, client, endpoint, model, response, batch)
		if batchErr != nil {
			return nil, fmt.Errorf("normalize job batch %d/%d: %w", index+1, len(batches), batchErr)
		}
		jobs = append(jobs, batchJobs...)
	}
	return jobs, nil
}

func (p OpenAICompatible) normalizeJobBatch(ctx context.Context, client *http.Client, endpoint, model string, response crawler.Response, batch []byte) ([]crawler.NormalizedJob, error) {
	input := fmt.Sprintf("Source URL: %s\nContent-Type: %s\nResponse body:\n%s", response.URL, response.ContentType, batch)
	payload := map[string]any{
		"model":       model,
		"temperature": 0,
		"stream":      false,
		"messages": []map[string]string{
			{"role": "system", "content": normalizationPrompt},
			{"role": "user", "content": input},
		},
		"response_format": map[string]string{"type": "json_object"},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
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
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}
	content, err := completionContent(raw)
	if err != nil {
		return nil, err
	}
	return decodeNormalizedJobs(content)
}

func normalizationBatches(body []byte, batchSize int) [][]byte {
	if batchSize < 1 {
		batchSize = 1
	}
	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(body, &envelope); err != nil {
		return [][]byte{body}
	}
	rawItems, ok := envelope["items"]
	if !ok {
		return [][]byte{body}
	}
	var items []json.RawMessage
	if err := json.Unmarshal(rawItems, &items); err != nil {
		return [][]byte{body}
	}
	active := make([]json.RawMessage, 0, len(items))
	for _, item := range items {
		var state struct {
			IsExpired bool `json:"isExpired"`
		}
		if json.Unmarshal(item, &state) == nil && state.IsExpired {
			continue
		}
		active = append(active, item)
	}
	batches := make([][]byte, 0, (len(active)+batchSize-1)/batchSize)
	for start := 0; start < len(active); start += batchSize {
		end := start + batchSize
		if end > len(active) {
			end = len(active)
		}
		batch, err := json.Marshal(map[string]any{"items": active[start:end]})
		if err == nil {
			batches = append(batches, batch)
		}
	}
	return batches
}

type chatCompletion struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

func completionContent(raw []byte) (string, error) {
	var completion chatCompletion
	if err := json.Unmarshal(raw, &completion); err == nil {
		if len(completion.Choices) > 0 && completion.Choices[0].Message.Content != "" {
			return completion.Choices[0].Message.Content, nil
		}
		return "", fmt.Errorf("AI API returned no normalized jobs")
	}
	// Do not run the malformed-object recovery on SSE: its embedded JSON
	// strings also contain `"content":` and would be mistaken for a broken
	// non-streaming envelope.
	if !strings.HasPrefix(strings.TrimSpace(string(raw)), "data:") {
		if content, ok := malformedContentObject(raw); ok {
			return content, nil
		}
	}

	// Some OpenAI-compatible gateways return SSE even when stream=false. Join
	// OpenAI delta chunks rather than treating the leading "data:" as JSON.
	var content strings.Builder
	seenEvent := false
	for _, line := range strings.Split(string(raw), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data:") {
			continue
		}
		seenEvent = true
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "[DONE]" {
			break
		}
		var event chatCompletion
		if err := json.Unmarshal([]byte(data), &event); err != nil {
			return "", fmt.Errorf("invalid AI streaming event: %w", err)
		}
		if len(event.Choices) == 0 {
			continue
		}
		if event.Choices[0].Delta.Content != "" {
			content.WriteString(event.Choices[0].Delta.Content)
		} else if event.Choices[0].Message.Content != "" {
			content.WriteString(event.Choices[0].Message.Content)
		}
	}
	if !seenEvent {
		return "", fmt.Errorf("invalid AI API response: expected JSON or SSE data events")
	}
	if content.Len() == 0 {
		return "", fmt.Errorf("AI API returned no normalized jobs")
	}
	return content.String(), nil
}

// malformedContentObject handles a broken OpenAI-compatible response such as
// `"content":"{"jobs":[]}"`, where the gateway forgot to JSON-escape the
// model output. This should be fixed at the gateway; the fallback prevents one
// malformed completion from blocking the crawl queue in the meantime.
func malformedContentObject(raw []byte) (string, bool) {
	contentKey := []byte(`"content":`)
	keyIndex := bytes.Index(raw, contentKey)
	if keyIndex < 0 {
		return "", false
	}
	start := bytes.IndexByte(raw[keyIndex+len(contentKey):], '{')
	if start < 0 {
		return "", false
	}
	start += keyIndex + len(contentKey)

	depth := 0
	inString, escaped := false, false
	for index := start; index < len(raw); index++ {
		char := raw[index]
		if inString {
			if escaped {
				escaped = false
			} else if char == '\\' {
				escaped = true
			} else if char == '"' {
				inString = false
			}
			continue
		}
		switch char {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return string(raw[start : index+1]), true
			}
		}
	}
	return "", false
}

func decodeNormalizedJobs(content string) ([]crawler.NormalizedJob, error) {
	content = strings.TrimSpace(content)
	// Some OpenAI-compatible providers ignore response_format and prefix a valid
	// JSON object with prose or a Markdown fence. Keep only the JSON object.
	start, end := strings.Index(content, "{"), strings.LastIndex(content, "}")
	if start < 0 || end < start {
		return nil, fmt.Errorf("AI API returned normalized jobs without a JSON object")
	}
	var output struct {
		Jobs []crawler.NormalizedJob `json:"jobs"`
	}
	if err := json.Unmarshal([]byte(content[start:end+1]), &output); err != nil {
		return nil, fmt.Errorf("invalid normalized jobs: %w", err)
	}
	return output.Jobs, nil
}
