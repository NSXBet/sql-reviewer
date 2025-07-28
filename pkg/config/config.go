package config

import (
	"encoding/json"
	"log/slog"
	"os"

	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
	"gopkg.in/yaml.v3"
)

// Config represents the configuration for SQL review
type Config struct {
	ID    string                 `yaml:"id" json:"id"`
	Rules []*types.SQLReviewRule `yaml:"rules" json:"rules"`
}

// LoadFromFile loads configuration from a file
func LoadFromFile(filename string) (*Config, error) {
	slog.Debug("Loading config from file", "filename", filename)
	data, err := os.ReadFile(filename)
	if err != nil {
		slog.Debug("Failed to read file", "error", err)
		return nil, err
	}

	slog.Debug("File content preview", "content", string(data[:min(200, len(data))]))

	var config Config

	// Try YAML first, then JSON
	slog.Debug("Attempting YAML unmarshal")
	if err := yaml.Unmarshal(data, &config); err != nil {
		slog.Debug("YAML unmarshal failed", "error", err)
		slog.Debug("Attempting JSON unmarshal")
		if err := json.Unmarshal(data, &config); err != nil {
			slog.Debug("JSON unmarshal failed", "error", err)
			return nil, err
		}
		slog.Debug("JSON unmarshal succeeded")
	} else {
		slog.Debug("YAML unmarshal succeeded")
	}

	// Normalize payloads to match expected format
	for _, rule := range config.Rules {
		rule.Payload = normalizePayload(rule.Type, rule.Payload)
	}

	slog.Debug("Loaded config", "rules_count", len(config.Rules))
	return &config, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// normalizePayload converts YAML config payload format to expected rule format
func normalizePayload(ruleType string, payload map[string]interface{}) map[string]interface{} {
	if payload == nil {
		// For rules that should have empty arrays as defaults
		switch ruleType {
		case "table.disallow-ddl", "table.disallow-dml":
			return map[string]interface{}{
				"list": []string{},
			}
		}
		return nil
	}

	// Convert based on rule type expectations
	switch ruleType {
	case "table.drop-naming-convention":
		// Convert format -> string
		if format, ok := payload["format"]; ok {
			return map[string]interface{}{
				"string": format,
			}
		}
	case "statement.query.minimum-plan-level":
		// YAML config seems to have wrong structure, fix it
		if _, hasRequired := payload["required"]; hasRequired {
			// This is misconfigured, use default INDEX level
			return map[string]interface{}{
				"string": "INDEX",
			}
		}
		if level, ok := payload["level"]; ok {
			return map[string]interface{}{
				"string": level,
			}
		}
	case "naming.table", "naming.column":
		// These have format + maxLength but we only need format for string type
		if format, ok := payload["format"]; ok {
			return map[string]interface{}{
				"string": format,
			}
		}
	case "naming.index.uk", "naming.index.idx", "naming.index.fk", "naming.column.auto-increment":
		// These also have format + maxLength but need string format
		if format, ok := payload["format"]; ok {
			return map[string]interface{}{
				"string": format,
			}
		}
	}

	// Default: return as-is for cases that already match expected format
	return payload
}

// DefaultConfig returns a default configuration
func DefaultConfig(id string) *Config {
	return &Config{
		ID:    id,
		Rules: []*types.SQLReviewRule{},
	}
}

// GetRulesForEngine returns rules applicable to the given engine
func (c *Config) GetRulesForEngine(engine types.Engine) []*types.SQLReviewRule {
	var rules []*types.SQLReviewRule
	for _, rule := range c.Rules {
		if rule.Engine == types.Engine_ENGINE_UNSPECIFIED || rule.Engine == engine {
			rules = append(rules, rule)
		}
	}
	return rules
}
