package mysqlparser

import (
	"github.com/antlr4-go/antlr/v4"
	parser "github.com/gedhean/mysql-parser"
)

// isEmptyTokenSequence checks if a token sequence is empty (only comments and semicolons)
//
//nolint:unused
func isEmptyTokenSequence(tokens []antlr.Token, semicolonType int) bool {
	for _, token := range tokens {
		if token.GetChannel() == antlr.TokenDefaultChannel &&
			token.GetTokenType() != semicolonType &&
			token.GetTokenType() != parser.MySQLParserEOF {
			return false
		}
	}
	return true
}
