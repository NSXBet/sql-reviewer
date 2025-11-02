package mysql

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/antlr4-go/antlr/v4"
	mysql "github.com/gedhean/mysql-parser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/advisor"
	"github.com/nsxbet/sql-reviewer-cli/pkg/mysqlparser"
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
)

// TableTextFieldsTotalLengthRule is the ANTLR-based implementation for checking table text fields total length
type TableTextFieldsTotalLengthRule struct {
	BaseAntlrRule
	maximum int
}

// NewTableTextFieldsTotalLengthRule creates a new ANTLR-based table text fields total length rule
func NewTableTextFieldsTotalLengthRule(
	level types.SQLReviewRuleLevel,
	title string,
	maximum int,
) *TableTextFieldsTotalLengthRule {
	return &TableTextFieldsTotalLengthRule{
		BaseAntlrRule: BaseAntlrRule{
			level: level,
			title: title,
		},
		maximum: maximum,
	}
}

// Name returns the rule name
func (*TableTextFieldsTotalLengthRule) Name() string {
	return "TableTextFieldsTotalLengthRule"
}

// OnEnter is called when entering a parse tree node
func (r *TableTextFieldsTotalLengthRule) OnEnter(ctx antlr.ParserRuleContext, nodeType string) error {
	switch nodeType {
	case NodeTypeCreateTable:
		r.checkCreateTable(ctx.(*mysql.CreateTableContext))
	}
	return nil
}

// OnExit is called when exiting a parse tree node
func (*TableTextFieldsTotalLengthRule) OnExit(_ antlr.ParserRuleContext, _ string) error {
	return nil
}

func (r *TableTextFieldsTotalLengthRule) checkCreateTable(ctx *mysql.CreateTableContext) {
	if ctx.TableElementList() == nil || ctx.TableName() == nil {
		return
	}

	_, tableName := mysqlparser.NormalizeMySQLTableName(ctx.TableName())
	if tableName == "" {
		return
	}

	var totalLength int64
	for _, element := range ctx.TableElementList().AllTableElement() {
		if element.ColumnDefinition() == nil {
			continue
		}

		columnDef := element.ColumnDefinition()
		if columnDef.FieldDefinition() == nil || columnDef.FieldDefinition().DataType() == nil {
			continue
		}

		dataType := columnDef.FieldDefinition().DataType()
		textLength := r.getTextLength(dataType.GetText())
		totalLength += textLength
	}

	if totalLength > int64(r.maximum) {
		r.AddAdvice(&types.Advice{
			Status: types.Advice_Status(r.level),
			Code:   int32(types.TotalTextLengthExceedsLimit),
			Title:  r.title,
			Content: fmt.Sprintf(
				"Table %q total text column length (%d) exceeds the limit (%d).",
				tableName,
				totalLength,
				r.maximum,
			),
			StartPosition: ConvertANTLRLineToPosition(r.baseLine + ctx.GetStart().GetLine()),
		})
	}
}

func (r *TableTextFieldsTotalLengthRule) getTextLength(dataTypeText string) int64 {
	s := strings.ToLower(dataTypeText)
	switch {
	case strings.HasPrefix(s, "char"):
		if strings.Contains(s, "(") {
			return r.extractLength(s, 1) // Default char length is 1
		}
		return 1
	case strings.HasPrefix(s, "varchar"):
		return r.extractLength(s, 255) // Default varchar length is 255
	case strings.HasPrefix(s, "binary"):
		if strings.Contains(s, "(") {
			return r.extractLength(s, 1)
		}
		return 1
	case strings.HasPrefix(s, "varbinary"):
		return r.extractLength(s, 255)
	case strings.HasPrefix(s, "tinytext") || strings.HasPrefix(s, "tinyblob"):
		return 255
	case strings.HasPrefix(s, "text") || strings.HasPrefix(s, "blob"):
		return 65535
	case strings.HasPrefix(s, "mediumtext") || strings.HasPrefix(s, "mediumblob"):
		return 16777215
	case strings.HasPrefix(s, "longtext") || strings.HasPrefix(s, "longblob"):
		return 4294967295
	default:
		return 0 // Non-text types
	}
}

func (r *TableTextFieldsTotalLengthRule) extractLength(dataType string, defaultLength int64) int64 {
	re := regexp.MustCompile(`\((\d+)\)`)
	match := re.FindStringSubmatch(dataType)
	if len(match) >= 2 {
		if length, err := strconv.ParseInt(match[1], 10, 64); err == nil {
			return length
		}
	}
	return defaultLength
}

// TableTextFieldsTotalLengthAdvisor is the advisor using ANTLR parser for table text fields total length checking
type TableTextFieldsTotalLengthAdvisor struct{}

// Check performs the ANTLR-based table text fields total length check
func (a *TableTextFieldsTotalLengthAdvisor) Check(
	ctx context.Context,
	statements string,
	rule *types.SQLReviewRule,
	checkContext advisor.SQLReviewCheckContext,
) ([]*types.Advice, error) {
	root, err := mysqlparser.ParseMySQL(statements)
	if err != nil {
		return ConvertSyntaxErrorToAdvice(err)
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(rule.Level)
	if err != nil {
		return nil, err
	}

	payload, err := advisor.UnmarshalNumberTypeRulePayload(rule.Payload)
	if err != nil {
		return nil, err
	}

	// Create the rule
	tableTextFieldsTotalLengthRule := NewTableTextFieldsTotalLengthRule(
		types.SQLReviewRuleLevel(level),
		string(rule.Type),
		payload.Number,
	)

	// Create the generic checker with the rule
	checker := NewGenericAntlrChecker([]AntlrRule{tableTextFieldsTotalLengthRule})

	for _, stmtNode := range root {
		tableTextFieldsTotalLengthRule.SetBaseLine(stmtNode.BaseLine)
		checker.SetBaseLine(stmtNode.BaseLine)
		antlr.ParseTreeWalkerDefault.Walk(checker, stmtNode.Tree)
	}

	return checker.GetAdviceList(), nil
}
