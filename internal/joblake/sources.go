package joblake

import (
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
)

var boardPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]{0,99}$`)

type Source struct {
	Company string `json:"company"`
	Board   string `json:"board"`
}

func validateSources(sources []Source) error {
	if len(sources) == 0 {
		return errors.New("at least one source is required")
	}
	seen := map[string]bool{}
	for _, source := range sources {
		board := strings.ToLower(source.Board)
		if !boardPattern.MatchString(source.Board) || strings.TrimSpace(source.Company) == "" || seen[board] {
			return fmt.Errorf("invalid or duplicate SmartRecruiters source %q", source.Board)
		}
		seen[board] = true
	}
	return nil
}

//go:embed sources.json
var builtInSources []byte

func LoadSources(path string) ([]Source, error) {
	content := builtInSources
	if path != "" {
		var err error
		content, err = os.ReadFile(path)
		if err != nil {
			return nil, err
		}
	}
	var sources []Source
	if err := json.Unmarshal(content, &sources); err != nil {
		return nil, err
	}
	return sources, validateSources(sources)
}
