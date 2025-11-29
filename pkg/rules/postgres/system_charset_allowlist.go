package postgres

import (
	"context"
	"fmt"
	"strings"

	"github.com/antlr4-go/antlr/v4"
	parser "github.com/gedhean/parser/postgresql"
	"github.com/nsxbet/sql-reviewer/pkg/advisor"
	"github.com/nsxbet/sql-reviewer/pkg/types"
)

var _ advisor.Advisor = (*EncodingAllowlistAdvisor)(nil)

func init() {
	advisor.Register(types.Engine_POSTGRES, advisor.Type(advisor.SchemaRuleCharsetAllowlist), &EncodingAllowlistAdvisor{})
}

// EncodingAllowlistAdvisor is the advisor checking for encoding allowlist.
type EncodingAllowlistAdvisor struct{}

// Check checks for encoding allowlist.
func (*EncodingAllowlistAdvisor) Check(_ context.Context, checkCtx advisor.Context) ([]*types.Advice, error) {
	tree, err := getANTLRTree(checkCtx)
	if err != nil {
		return ConvertSyntaxErrorToAdvice(err)
	}

	level, err := advisor.NewStatusBySQLReviewRuleLevel(checkCtx.Rule.Level)
	if err != nil {
		return nil, err
	}

	payload, err := advisor.UnmarshalStringArrayTypeRulePayload(checkCtx.Rule.Payload)
	if err != nil {
		return nil, err
	}

	// Convert allowlist to lowercase for case-insensitive comparison
	allowlist := make(map[string]bool)
	for _, encoding := range payload.List {
		allowlist[strings.ToLower(encoding)] = true
	}

	checker := &encodingAllowlistChecker{
		BasePostgreSQLParserListener: &parser.BasePostgreSQLParserListener{},
		level:                        level,
		title:                        string(checkCtx.Rule.Type),
		allowlist:                    allowlist,
	}

	antlr.ParseTreeWalkerDefault.Walk(checker, tree.Tree)

	return checker.adviceList, nil
}

type encodingAllowlistChecker struct {
	*parser.BasePostgreSQLParserListener

	adviceList []*types.Advice
	level      types.Advice_Status
	title      string
	allowlist  map[string]bool
}

func (c *encodingAllowlistChecker) EnterCreatedbstmt(ctx *parser.CreatedbstmtContext) {
	if !isTopLevel(ctx.GetParent()) {
		return
	}

	// Extract encoding from createdb_opt_list
	// Check both with and without WITH keyword
	var encoding string
	if ctx.Createdb_opt_list() != nil {
		encoding = c.extractEncoding(ctx.Createdb_opt_list())
	}

	if encoding != "" {
		// Check if encoding is in allowlist
		if !c.allowlist[strings.ToLower(encoding)] {
			c.adviceList = append(c.adviceList, &types.Advice{
				Status:  c.level,
				Code:    int32(types.DisabledCharset),
				Title:   c.title,
				Content: fmt.Sprintf("\"\" used disabled encoding '%s'", strings.ToLower(encoding)),
				StartPosition: &types.Position{
					Line: int32(ctx.GetStart().GetLine()),
				},
			})
		}
	}
}

func (*encodingAllowlistChecker) extractEncoding(optList parser.ICreatedb_opt_listContext) string {
	if optList == nil {
		return ""
	}

	// Iterate through all createdb_opt_items
	if optList.Createdb_opt_items() == nil {
		return ""
	}

	for _, item := range optList.Createdb_opt_items().AllCreatedb_opt_item() {
		if item == nil {
			continue
		}

		// Check if this is an ENCODING option
		if item.Createdb_opt_name() != nil && item.Createdb_opt_name().ENCODING() != nil {
			// Get the encoding value from Opt_boolean_or_string
			if item.Opt_boolean_or_string() != nil {
				// Could be a string constant or identifier
				text := item.Opt_boolean_or_string().GetText()
				// Remove quotes if present
				if len(text) >= 2 && text[0] == '\'' && text[len(text)-1] == '\'' {
					return text[1 : len(text)-1]
				}
				return text
			}
		}
	}

	return ""
}
