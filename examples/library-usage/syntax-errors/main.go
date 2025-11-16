package main

import (
	"context"
	"fmt"

	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/types"

	// Import PostgreSQL rules for registration
	_ "github.com/nsxbet/sql-reviewer/pkg/rules/postgres"
)

func main() {
	fmt.Println("PostgreSQL Syntax Error to Advice Conversion Demo")
	fmt.Println("==================================================")
	fmt.Println("")

	// Example 1: Missing table name in CREATE TABLE
	fmt.Println("Example 1: Missing table name")
	fmt.Println("SQL: CREATE TABLE;")
	demonstrateSyntaxError("CREATE TABLE;", advisor.SchemaRuleTableNaming)

	// Example 2: Invalid INSERT statement
	fmt.Println("\nExample 2: Invalid INSERT statement")
	fmt.Println("SQL: INSERT users VALUES (1, 'John');")
	demonstrateSyntaxError("INSERT users VALUES (1, 'John');", advisor.SchemaRuleStatementInsertMustSpecifyColumn)

	// Example 3: Missing table in SELECT FROM
	fmt.Println("\nExample 3: Missing table in SELECT FROM")
	fmt.Println("SQL: SELECT * FROM WHERE id = 1;")
	demonstrateSyntaxError("SELECT * FROM WHERE id = 1;", advisor.SchemaRuleStatementNoSelectAll)

	// Example 4: Incomplete ALTER TABLE
	fmt.Println("\nExample 4: Incomplete ALTER TABLE")
	fmt.Println("SQL: ALTER TABLE ADD;")
	demonstrateSyntaxError("ALTER TABLE ADD;", advisor.SchemaRuleColumnNaming)

	// Example 5: Incomplete CREATE INDEX
	fmt.Println("\nExample 5: Incomplete CREATE INDEX")
	fmt.Println("SQL: CREATE INDEX;")
	demonstrateSyntaxError("CREATE INDEX;", advisor.SchemaRuleIDXNaming)

	// Example 6: Valid SQL for comparison
	fmt.Println("\nExample 6: Valid SQL (no syntax errors)")
	fmt.Println("SQL: CREATE TABLE users (id SERIAL PRIMARY KEY, name VARCHAR(100));")
	checkCtx := advisor.Context{
		DBType:     types.Engine_POSTGRES,
		Statements: "CREATE TABLE users (id SERIAL PRIMARY KEY, name VARCHAR(100));",
		Rule: &types.SQLReviewRule{
			Type:    string(advisor.SchemaRuleTableNaming),
			Level:   types.SQLReviewRuleLevel_ERROR,
			Payload: map[string]interface{}{"format": "^[a-z]+(_[a-z]+)*$", "maxLength": 64},
		},
		ChangeType: types.PlanCheckRunConfig_DDL,
	}

	advices, err := advisor.Check(context.Background(), types.Engine_POSTGRES, advisor.Type(advisor.SchemaRuleTableNaming), checkCtx)
	if err != nil {
		fmt.Printf("  ❌ Unexpected error: %v\n", err)
		return
	}

	syntaxErrors := filterByCode(advices, 201)
	if len(syntaxErrors) > 0 {
		fmt.Println("  ⚠️  Unexpected syntax error in valid SQL")
	} else {
		fmt.Println("  ✅ No syntax errors - SQL is valid")
	}

	fmt.Println("\n==================================================")
	fmt.Println("Key Takeaways:")
	fmt.Println("- Syntax errors are converted to Advice objects (code 201)")
	fmt.Println("- Position information (line/column) is preserved")
	fmt.Println("- Errors are NOT silently discarded - users see them")
	fmt.Println("- Consistent behavior across MySQL and PostgreSQL")
}

func demonstrateSyntaxError(sql string, ruleType advisor.SQLReviewRuleType) {
	// Create check context with the rule
	checkCtx := advisor.Context{
		DBType:     types.Engine_POSTGRES,
		Statements: sql,
		Rule: &types.SQLReviewRule{
			Type:    string(ruleType),
			Level:   types.SQLReviewRuleLevel_ERROR,
			Payload: getDefaultPayload(ruleType),
		},
		ChangeType: types.PlanCheckRunConfig_DDL,
	}

	// Call advisor.Check which will trigger rule execution
	advices, err := advisor.Check(context.Background(), types.Engine_POSTGRES, advisor.Type(ruleType), checkCtx)
	if err != nil {
		fmt.Printf("  ❌ Unexpected error during check: %v\n", err)
		return
	}

	// Filter for syntax errors (code 201)
	syntaxErrors := filterByCode(advices, 201)
	if len(syntaxErrors) > 0 {
		fmt.Printf("  ✅ Syntax error converted to advice (Code 201):\n")
		for _, advice := range syntaxErrors {
			fmt.Printf("     Title: %s\n", advice.Title)
			fmt.Printf("     Content: %s\n", advice.Content)
			if advice.StartPosition != nil {
				fmt.Printf("     Position: Line %d, Column %d\n",
					advice.StartPosition.Line+1, // Convert 0-indexed to 1-indexed for display
					advice.StartPosition.Column)
			}
			fmt.Printf("     Status: %s\n", advice.Status)
		}
	} else {
		fmt.Println("  ⚠️  No syntax error advice found")
		if len(advices) > 0 {
			fmt.Printf("     (Found %d other advice items)\n", len(advices))
		}
	}
}

func filterByCode(advices []*types.Advice, code int32) []*types.Advice {
	var result []*types.Advice
	for _, advice := range advices {
		if advice.Code == code {
			result = append(result, advice)
		}
	}
	return result
}

func getDefaultPayload(ruleType advisor.SQLReviewRuleType) map[string]interface{} {
	switch ruleType {
	case advisor.SchemaRuleTableNaming, advisor.SchemaRuleColumnNaming:
		return map[string]interface{}{
			"format":    "^[a-z]+(_[a-z]+)*$",
			"maxLength": 64,
		}
	case advisor.SchemaRuleIDXNaming:
		return map[string]interface{}{
			"format":       "^$|^idx_{{table}}_{{column_list}}$",
			"maxLength":    64,
			"templateList": []string{"table", "column_list"},
		}
	default:
		return map[string]interface{}{}
	}
}
