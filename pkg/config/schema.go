package config

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/types"
	"gopkg.in/yaml.v3"
)

// SchemaRule represents a rule definition from schema.yaml
type SchemaRule struct {
	Type          string              `yaml:"type"`
	Category      string              `yaml:"category"`
	Engine        string              `yaml:"engine"`
	ComponentList []SchemaRulePayload `yaml:"componentList,omitempty"`
}

// SchemaRulePayload represents the payload configuration for a rule
type SchemaRulePayload struct {
	Key     string `yaml:"key"`
	Payload struct {
		Type         string      `yaml:"type"`
		Default      interface{} `yaml:"default"`
		TemplateList []string    `yaml:"templateList,omitempty"`
	} `yaml:"payload"`
}

// LoadSchema loads the schema.yaml file and returns schema rules
func LoadSchema(filename string) ([]SchemaRule, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema file %s: %w", filename, err)
	}

	var rules []SchemaRule
	if err := yaml.Unmarshal(data, &rules); err != nil {
		return nil, fmt.Errorf("failed to parse schema file %s: %w", filename, err)
	}

	return rules, nil
}

// ConvertSchemaRulesToConfig converts schema rules to SQL review rules with default payloads
func ConvertSchemaRulesToConfig(schemaRules []SchemaRule, engine types.Engine) (*Config, error) {
	var sqlRules []*types.SQLReviewRule

	for _, schemaRule := range schemaRules {
		// Convert engine string to types.Engine
		ruleEngine, err := parseEngineFromString(schemaRule.Engine)
		if err != nil {
			continue // Skip unknown engines
		}

		// Only include rules for the specified engine
		if ruleEngine != engine {
			continue
		}

		// Create the SQL review rule
		sqlRule := &types.SQLReviewRule{
			Type:   schemaRule.Type,
			Level:  types.SQLReviewRuleLevel_ERROR, // Default level
			Engine: ruleEngine,
		}

		// Build the payload from componentList
		if payload, err := buildPayloadFromComponents(schemaRule.ComponentList); err == nil && payload != nil {
			sqlRule.Payload = payload
		}

		sqlRules = append(sqlRules, sqlRule)
	}

	return &Config{
		ID:    "schema-default",
		Rules: sqlRules,
	}, nil
}

// buildPayloadFromComponents builds a payload map from schema rule components
func buildPayloadFromComponents(components []SchemaRulePayload) (map[string]interface{}, error) {
	if len(components) == 0 {
		return nil, nil
	}

	// Handle single component rules
	if len(components) == 1 {
		comp := components[0]
		return buildSinglePayload(comp)
	}

	// Handle multi-component rules - need special handling for naming rules
	// Naming rules expect only the "format" field as a "string" payload
	for _, comp := range components {
		if comp.Key == "format" && (comp.Payload.Type == "STRING" || comp.Payload.Type == "TEMPLATE") {
			// For naming rules, only use the format field as string payload
			payload := advisor.StringTypeRulePayload{
				String: fmt.Sprintf("%v", comp.Payload.Default),
			}
			return marshalToMap(payload)
		}
	}

	// Fallback to composite payload for other multi-component rules
	payload := make(map[string]interface{})
	for _, comp := range components {
		payload[comp.Key] = comp.Payload.Default
	}

	return payload, nil
}

// buildSinglePayload builds payload for single component rules following advisor patterns
func buildSinglePayload(comp SchemaRulePayload) (map[string]interface{}, error) {
	switch comp.Payload.Type {
	case "STRING":
		payload := advisor.StringTypeRulePayload{
			String: fmt.Sprintf("%v", comp.Payload.Default),
		}
		return marshalToMap(payload)

	case "NUMBER":
		var number int
		switch v := comp.Payload.Default.(type) {
		case int:
			number = v
		case float64:
			number = int(v)
		default:
			number = 0
		}
		payload := advisor.NumberTypeRulePayload{
			Number: number,
		}
		return marshalToMap(payload)

	case "STRING_ARRAY":
		var list []string
		if defaultList, ok := comp.Payload.Default.([]interface{}); ok {
			for _, item := range defaultList {
				if str, ok := item.(string); ok {
					list = append(list, str)
				}
			}
		}
		// Handle empty arrays explicitly - they're valid
		payload := advisor.StringArrayTypeRulePayload{
			List: list,
		}
		return marshalToMap(payload)

	case "TEMPLATE":
		payload := advisor.StringTypeRulePayload{
			String: fmt.Sprintf("%v", comp.Payload.Default),
		}
		return marshalToMap(payload)

	case "BOOLEAN":
		var boolVal bool
		if b, ok := comp.Payload.Default.(bool); ok {
			boolVal = b
		}
		payload := advisor.BooleanTypeRulePayload{
			Boolean: boolVal,
		}
		return marshalToMap(payload)

	default:
		return nil, fmt.Errorf("unsupported payload type: %s", comp.Payload.Type)
	}
}

// marshalToMap converts a struct to map[string]interface{}
func marshalToMap(v interface{}) (map[string]interface{}, error) {
	jsonBytes, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// parseEngineFromString converts engine string to types.Engine
func parseEngineFromString(engineStr string) (types.Engine, error) {
	switch engineStr {
	case "MYSQL":
		return types.Engine_MYSQL, nil
	case "POSTGRES":
		return types.Engine_POSTGRES, nil
	default:
		return types.Engine_ENGINE_UNSPECIFIED, fmt.Errorf("unknown engine: %s", engineStr)
	}
}
