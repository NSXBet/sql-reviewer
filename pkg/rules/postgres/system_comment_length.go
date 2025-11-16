package postgres

import (
	"context"
	"fmt"

	"github.com/antlr4-go/antlr/v4"
	parser "github.com/bytebase/parser/postgresql"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/pgparser"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

var _ advisor.Advisor = (*SystemCommentLengthAdvisor)(nil)

func init() {
	advisor.Register(types.Engine_POSTGRES, advisor.Type(advisor.SchemaRuleCommentLength), &SystemCommentLengthAdvisor{})
}

// SystemCommentLengthAdvisor is the advisor for system comment length.
type SystemCommentLengthAdvisor struct{}

// Check checks the system comment length.
func (*SystemCommentLengthAdvisor) Check(ctx context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
	tree, err := getANTLRTree(checkCtx)
	if err != nil {
		return ConvertSyntaxErrorToAdvice(err)
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(checkCtx.Rule.Level)
	if err != nil {
		return nil, err
	}

	payload, err := advisor.UnmarshalNumberTypeRulePayload(checkCtx.Rule.Payload)
	if err != nil {
		return nil, err
	}

	checker := &systemCommentLengthChecker{
		level:     level,
		title:     string(checkCtx.Rule.Type),
		maxLength: payload.Number,
	}

	antlr.ParseTreeWalkerDefault.Walk(checker, tree.Tree)

	return checker.adviceList, nil
}

type systemCommentLengthChecker struct {
	*parser.BasePostgreSQLParserListener

	level      types.Advice_Status
	title      string
	maxLength  int
	adviceList []*types.Advice
}

// EnterCommentstmt checks COMMENT statements
func (c *systemCommentLengthChecker) EnterCommentstmt(ctx *parser.CommentstmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	// Extract comment text
	comment := ""
	if ctx.Comment_text() != nil && ctx.Comment_text().Sconst() != nil {
		comment = extractStringConstant(ctx.Comment_text().Sconst())
	}

	// Check length
	if c.maxLength > 0 && len(comment) > c.maxLength {
		objectType := "UNKNOWN"
		objectName := ""

		// Determine object type using Object_type_any_name or Object_type_name
		if ctx.Object_type_any_name() != nil {
			if ctx.Object_type_any_name().TABLE() != nil {
				objectType = "TABLE"
			} else if ctx.Object_type_any_name().INDEX() != nil {
				objectType = "INDEX"
			}

			// Extract name from Any_name
			if ctx.Any_name() != nil {
				parts := pgparser.NormalizePostgreSQLAnyName(ctx.Any_name())
				if len(parts) > 0 {
					objectName = parts[len(parts)-1]
				}
			}
		} else if ctx.Object_type_name() != nil {
			if ctx.Object_type_name().DATABASE() != nil {
				objectType = "DATABASE"
			}
			// Note: SCHEMA type might not be available in this parser version
			// Most schema comments would go through the Any_name path

			// Extract name from name
			if ctx.Name() != nil {
				objectName = ctx.Name().GetText()
			}
		} else if ctx.COLUMN() != nil {
			// COMMENT ON COLUMN is special case
			objectType = "COLUMN"
			if ctx.Any_name() != nil {
				parts := pgparser.NormalizePostgreSQLAnyName(ctx.Any_name())
				if len(parts) > 0 {
					objectName = parts[len(parts)-1]
				}
			}
		}

		content := fmt.Sprintf("%s `%s` comment is too long. The length of comment should be within %d characters (current: %d)",
			objectType, objectName, c.maxLength, len(comment))

		c.adviceList = append(c.adviceList, &types.Advice{
			Status:  c.level,
			Code:    int32(types.SystemCommentTooLong),
			Title:   c.title,
			Content: content,
			StartPosition: &types.Position{
				Line: int32(ctx.GetStart().GetLine()),
			},
		})
	}
}
