package mysqlparser

import (
	"github.com/antlr4-go/antlr/v4"
	parser "github.com/bytebase/mysql-parser"
)

// isEmptyTokenSequence checks if a token sequence is empty (only comments and semicolons)
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