package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/nsxbet/sql-reviewer-cli/pkg/advisor"
	"github.com/nsxbet/sql-reviewer-cli/pkg/catalog"
	"github.com/nsxbet/sql-reviewer-cli/pkg/config"
	"github.com/nsxbet/sql-reviewer-cli/pkg/logger"
	_ "github.com/nsxbet/sql-reviewer-cli/pkg/rules/mysql"
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

var checkCmd = &cobra.Command{
	Use:   "check [flags] <sql-file>",
	Short: "Check SQL statements against review rules",
	Long: `Check SQL statements in a file against configured review rules.

The tool will analyze the SQL statements and report any issues found
according to the configured rules for the specified database engine.`,
	Args: cobra.ExactArgs(1),
	RunE: runCheck,
}

func init() {
	rootCmd.AddCommand(checkCmd)

	// Flags for check command
	checkCmd.Flags().StringP("engine", "e", "mysql", "database engine (mysql, postgres)")
	checkCmd.Flags().StringP("output", "o", "text", "output format (text, json, yaml)")
	checkCmd.Flags().StringP("rules", "r", "", "path to rules configuration file")
	checkCmd.Flags().String("schema", "", "path to database schema file (JSON)")
	checkCmd.Flags().Bool("fail-on-error", false, "exit with non-zero code if errors are found")
	checkCmd.Flags().Bool("fail-on-warning", false, "exit with non-zero code if warnings are found")

	// Bind flags to viper
	_ = viper.BindPFlag("engine", checkCmd.Flags().Lookup("engine"))
	_ = viper.BindPFlag("output", checkCmd.Flags().Lookup("output"))
	_ = viper.BindPFlag("rules", checkCmd.Flags().Lookup("rules"))
	_ = viper.BindPFlag("schema", checkCmd.Flags().Lookup("schema"))
	_ = viper.BindPFlag("fail-on-error", checkCmd.Flags().Lookup("fail-on-error"))
	_ = viper.BindPFlag("fail-on-warning", checkCmd.Flags().Lookup("fail-on-warning"))
}

func runCheck(cmd *cobra.Command, args []string) error {
	// Initialize logger
	logLevel := slog.LevelInfo
	if viper.GetBool("debug") {
		logLevel = slog.LevelDebug
	} else if viper.GetBool("verbose") {
		logLevel = slog.LevelInfo
	}
	_ = logger.NewWithLevel(logLevel)

	slog.Debug("Starting check command", "args", args)

	// Parse engine
	engineStr := viper.GetString("engine")
	slog.Debug("Parsing engine", "engine", engineStr)
	engine, err := parseEngine(engineStr)
	if err != nil {
		slog.Debug("Failed to parse engine", "error", err)
		return err
	}
	slog.Debug("Engine parsed successfully", "engine", engine)

	// Read SQL file
	sqlFile := args[0]
	slog.Debug("Reading SQL file", "file", sqlFile)
	sqlContent, err := os.ReadFile(sqlFile)
	if err != nil {
		slog.Debug("Failed to read SQL file", "error", err)
		return errors.Wrapf(err, "failed to read SQL file: %s", sqlFile)
	}
	slog.Debug("SQL file read successfully", "size", len(sqlContent))

	// Load configuration
	slog.Debug("Loading configuration", "engine", engine)
	cfg, err := loadConfiguration(engine)
	if err != nil {
		slog.Error("failed to load configuration", "error", err)
		return err
	}
	slog.Debug("Configuration loaded successfully")

	// Load schema if provided
	var dbSchema *types.DatabaseSchemaMetadata
	schemaPath := viper.GetString("schema")
	if schemaPath != "" {
		dbSchema, err = loadDatabaseSchema(schemaPath)
		if err != nil {
			return err
		}
	}

	// Create catalog finder
	var finder *catalog.Finder
	if dbSchema != nil {
		finder = catalog.NewFinder(dbSchema, &catalog.FinderContext{
			CheckIntegrity:      true,
			EngineType:          engine,
			IgnoreCaseSensitive: !isEngineCaseSensitive(engine),
		})
	}

	// Run SQL review
	ctx := context.Background()
	advices, err := runSQLReview(ctx, string(sqlContent), cfg.GetRulesForEngine(engine), advisor.Context{
		DBType:                engine,
		DBSchema:              dbSchema,
		Catalog:               &catalogWrapper{finder: finder},
		ChangeType:            types.PlanCheckRunConfig_DDL,
		EnablePriorBackup:     false,
		IsObjectCaseSensitive: isEngineCaseSensitive(engine),
	})
	if err != nil {
		return err
	}

	// Output results
	outputFormat := viper.GetString("output")
	if err := outputResults(advices, outputFormat); err != nil {
		return err
	}

	// Check exit codes
	hasErrors := false
	hasWarnings := false
	for _, advice := range advices {
		if advice.Status == types.Advice_ERROR {
			hasErrors = true
		} else if advice.Status == types.Advice_WARNING {
			hasWarnings = true
		}
	}

	if hasErrors && viper.GetBool("fail-on-error") {
		os.Exit(1)
	}
	if hasWarnings && viper.GetBool("fail-on-warning") {
		os.Exit(1)
	}

	return nil
}

func parseEngine(engineStr string) (types.Engine, error) {
	switch strings.ToLower(engineStr) {
	case "mysql":
		return types.Engine_MYSQL, nil
	case "postgres", "postgresql":
		return types.Engine_POSTGRES, nil
	default:
		return types.Engine_ENGINE_UNSPECIFIED, errors.Errorf("unsupported database engine: %s", engineStr)
	}
}

func loadConfiguration(engine types.Engine) (*config.Config, error) {
	rulesPath := viper.GetString("rules")
	if rulesPath != "" {
		return config.LoadFromFile(rulesPath)
	}

	// Load default configuration from schema.yaml
	return loadDefaultConfigFromSchema(engine)
}

func loadDefaultConfigFromSchema(engine types.Engine) (*config.Config, error) {
	// Try to find schema.yaml in the working directory first
	schemaPath := "schema.yaml"
	if _, err := os.Stat(schemaPath); os.IsNotExist(err) {
		// If not found in working directory, try in the project root
		schemaPath = "/Users/nsx/workspace/bytebase/sql-reviewer-cli/schema.yaml"
		if _, err := os.Stat(schemaPath); os.IsNotExist(err) {
			slog.Warn("schema.yaml not found, using empty default config")
			return config.DefaultConfig("default"), nil
		}
	}

	slog.Debug("Loading default configuration from schema", "schema_path", schemaPath, "engine", engine)

	// Load schema rules
	schemaRules, err := config.LoadSchema(schemaPath)
	if err != nil {
		slog.Warn("Failed to load schema.yaml, using empty default config", "error", err)
		return config.DefaultConfig("default"), nil
	}

	// Convert schema rules to configuration with default payloads
	cfg, err := config.ConvertSchemaRulesToConfig(schemaRules, engine)
	if err != nil {
		slog.Warn("Failed to convert schema rules to config, using empty default config", "error", err)
		return config.DefaultConfig("default"), nil
	}

	slog.Debug("Loaded default configuration from schema", "rules_count", len(cfg.Rules))
	return cfg, nil
}

func loadDatabaseSchema(schemaPath string) (*types.DatabaseSchemaMetadata, error) {
	data, err := os.ReadFile(schemaPath)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to read schema file: %s", schemaPath)
	}

	var schema types.DatabaseSchemaMetadata
	if err := json.Unmarshal(data, &schema); err != nil {
		return nil, errors.Wrapf(err, "failed to parse schema file: %s", schemaPath)
	}

	return &schema, nil
}

func isEngineCaseSensitive(engine types.Engine) bool {
	switch engine {
	case types.Engine_MYSQL, types.Engine_MARIADB, types.Engine_TIDB:
		return false
	case types.Engine_POSTGRES:
		return true
	case types.Engine_ORACLE:
		return false
	case types.Engine_MSSQL:
		return false
	case types.Engine_SNOWFLAKE:
		return false
	default:
		return true
	}
}

func outputResults(advices []*types.Advice, format string) error {
	switch format {
	case "json":
		return outputJSON(advices)
	case "yaml":
		return outputYAML(advices)
	case "text":
		return outputText(advices)
	default:
		return errors.Errorf("unsupported output format: %s", format)
	}
}

func outputJSON(advices []*types.Advice) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(map[string]interface{}{
		"advices": advices,
	})
}

func outputYAML(advices []*types.Advice) error {
	encoder := yaml.NewEncoder(os.Stdout)
	defer encoder.Close()
	return encoder.Encode(map[string]interface{}{
		"advices": advices,
	})
}

func outputText(advices []*types.Advice) error {
	if len(advices) == 0 {
		fmt.Println("No issues found.")
		return nil
	}

	errorCount := 0
	warningCount := 0

	for _, advice := range advices {
		var prefix string
		switch advice.Status {
		case types.Advice_ERROR:
			prefix = "ERROR"
			errorCount++
		case types.Advice_WARNING:
			prefix = "WARNING"
			warningCount++
		default:
			prefix = "INFO"
		}

		position := ""
		if advice.StartPosition != nil {
			position = fmt.Sprintf(" at line %d, column %d", advice.StartPosition.Line, advice.StartPosition.Column)
		}

		fmt.Printf("[%s] %s%s\n", prefix, advice.Title, position)
		if advice.Content != "" {
			fmt.Printf("  %s\n", advice.Content)
		}
		fmt.Println()
	}

	fmt.Printf("Summary: %d error(s), %d warning(s)\n", errorCount, warningCount)
	return nil
}

// catalogWrapper wraps the catalog finder to implement the interface expected by advisor
type catalogWrapper struct {
	finder *catalog.Finder
}

func (c *catalogWrapper) GetFinder() *catalog.Finder {
	return c.finder
}

// runSQLReview runs SQL review using the advisor system
func runSQLReview(
	ctx context.Context,
	statements string,
	reviewRules []*types.SQLReviewRule,
	checkContext advisor.Context,
) ([]*types.Advice, error) {
	var allAdvices []*types.Advice

	// Process each rule and check against the statements
	for _, rule := range reviewRules {
		// Skip rules for different engines
		if rule.Engine != types.Engine_ENGINE_UNSPECIFIED && rule.Engine != checkContext.DBType {
			continue
		}

		// Skip disabled rules
		if rule.Level == types.SQLReviewRuleLevel_DISABLED {
			continue
		}

		// Set up the context for this rule check
		ruleCheckContext := checkContext
		ruleCheckContext.Rule = rule
		ruleCheckContext.Statements = statements

		// Use the advisor system to run the check
		advices, err := advisor.Check(ctx, checkContext.DBType, advisor.Type(rule.Type), ruleCheckContext)
		if err != nil {
			slog.Warn("Rule check failed", "rule_type", rule.Type, "error", err)
			continue
		}

		// Add advices to the result
		allAdvices = append(allAdvices, advices...)
	}

	return allAdvices, nil
}
