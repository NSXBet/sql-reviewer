package mysql

import (
	"context"
	"fmt"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/bytebase/mysql-parser"
	"github.com/pkg/errors"

	"github.com/nsxbet/sql-reviewer-cli/pkg/advisor"
	"github.com/nsxbet/sql-reviewer-cli/pkg/catalog"
	"github.com/nsxbet/sql-reviewer-cli/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
)

// DatabaseDropEmptyDatabaseRule is the ANTLR-based implementation for checking database drop only if empty
type DatabaseDropEmptyDatabaseRule struct {
	BaseAntlrRule
	catalog *catalog.Finder
}

// NewDatabaseDropEmptyDatabaseRule creates a new ANTLR-based database drop empty database rule
func NewDatabaseDropEmptyDatabaseRule(level types.SQLReviewRuleLevel, title string, catalog *catalog.Finder) *DatabaseDropEmptyDatabaseRule {
	return &DatabaseDropEmptyDatabaseRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
		catalog: catalog,
	}
}

// Name returns the rule name
func (*DatabaseDropEmptyDatabaseRule) Name() string {
	return "DatabaseDropEmptyDatabaseRule"
}

// OnEnter is called when entering a parse tree node
func (r *DatabaseDropEmptyDatabaseRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	if nodeType == NodeTypeDropDatabase {
		r.checkDropDatabase(ctx.(*mysql.DropDatabaseContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*DatabaseDropEmptyDatabaseRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *DatabaseDropEmptyDatabaseRule) checkDropDatabase(ctx *mysql.DropDatabaseContext) {
	if ctx.SchemaRef() == nil {
		return
	}

	dbName := mysqlparser.NormalizeMySQLSchemaRef(ctx.SchemaRef())
	
	// If catalog is nil, we can't check the database state, so assume it's not empty to be safe
	if r.catalog == nil {
		r.AddAdvice(&types.Advice{
			Status:        types.Advice_Status(r.level),
			Code:          int32(types.DatabaseNotEmpty),
			Title:         r.title,
			Content:       fmt.Sprintf("Database `%s` is not allowed to drop if not empty", dbName),
			StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
		})
		return
	}

	if r.catalog.Origin.DatabaseName() != dbName {
		r.AddAdvice(&types.Advice{
			Status:        types.Advice_Status(r.level),
			Code:          int32(types.NotCurrentDatabase),
			Title:         r.title,
			Content:       fmt.Sprintf("Database `%s` that is trying to be deleted is not the current database `%s`", dbName, r.catalog.Origin.DatabaseName()),
			StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
		})
	} else if !r.catalog.Origin.HasNoTable() {
		r.AddAdvice(&types.Advice{
			Status:        types.Advice_Status(r.level),
			Code:          int32(types.DatabaseNotEmpty),
			Title:         r.title,
			Content:       fmt.Sprintf("Database `%s` is not allowed to drop if not empty", dbName),
			StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
		})
	}
}

// DatabaseDropEmptyDatabaseAdvisor is the advisor using ANTLR parser for database drop empty database checking
type DatabaseDropEmptyDatabaseAdvisor struct{}

// Check performs the ANTLR-based database drop empty database check
func (a *DatabaseDropEmptyDatabaseAdvisor) Check(ctx context.Context, statements string, rule *types.SQLReviewRule, checkContext advisor.SQLReviewCheckContext) ([]*types.Advice, error) {
	root, err := mysqlparser.ParseMySQL(statements)
	if err != nil {
		return nil, errors.Wrap(err, "failed to parse MySQL statement")
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(rule.Level)
	if err != nil {
		return nil, err
	}

	// Get catalog finder
	var catalogFinder *catalog.Finder
	if checkContext.Catalog != nil {
		catalogFinder = checkContext.Catalog.GetFinder()
	}

	// Create the rule with catalog
	dropEmptyRule := NewDatabaseDropEmptyDatabaseRule(types.SQLReviewRuleLevel(level), string(rule.Type), catalogFinder)

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{dropEmptyRule})

	for _, stmtNode := range root {
		dropEmptyRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}