package mysqlparser

import (
	"fmt"

	"github.com/antlr4-go/antlr/v4"
	"github.com/nsxbet/sql-reviewer-cli/pkg/types"
)

// SyntaxError is a syntax error.
type SyntaxError struct {
	Position   *types.Position
	Message    string
	RawMessage string
}

// Error returns the error message.
func (e *SyntaxError) Error() string {
	return e.Message
}

// ParseErrorListener is a custom error listener for ANTLR parsing.
type ParseErrorListener struct {
	*antlr.DefaultErrorListener
	BaseLine  int
	Err       *SyntaxError
	Statement string
}

// SyntaxError returns the errors.
func (l *ParseErrorListener) SyntaxError(
	_ antlr.Recognizer,
	token any,
	line, column int,
	message string,
	_ antlr.RecognitionException,
) {
	if l.Err != nil {
		return
	}

	errMessage := ""
	if token, ok := token.(*antlr.CommonToken); ok {
		stream := token.GetInputStream()
		start := token.GetStart() - 40
		if start < 0 {
			start = 0
		}
		stop := token.GetStop()
		if stop >= stream.Size() {
			stop = stream.Size() - 1
		}
		errMessage = fmt.Sprintf("related text: %s", stream.GetTextFromInterval(antlr.NewInterval(start, stop)))
	}

	l.Err = &SyntaxError{
		Position: &types.Position{
			Line:   int32(line + l.BaseLine),
			Column: int32(column),
		},
		RawMessage: message,
		Message:    fmt.Sprintf("Syntax error at line %d:%d \n%s", line+l.BaseLine, column, errMessage),
	}
}

// ReportAmbiguity reports an ambiguity.
func (*ParseErrorListener) ReportAmbiguity(
	recognizer antlr.Parser,
	dfa *antlr.DFA,
	startIndex, stopIndex int,
	exact bool,
	ambigAlts *antlr.BitSet,
	configs *antlr.ATNConfigSet,
) {
	antlr.ConsoleErrorListenerINSTANCE.ReportAmbiguity(recognizer, dfa, startIndex, stopIndex, exact, ambigAlts, configs)
}

// ReportAttemptingFullContext reports an attempting full context.
func (*ParseErrorListener) ReportAttemptingFullContext(
	recognizer antlr.Parser,
	dfa *antlr.DFA,
	startIndex, stopIndex int,
	conflictingAlts *antlr.BitSet,
	configs *antlr.ATNConfigSet,
) {
	antlr.ConsoleErrorListenerINSTANCE.ReportAttemptingFullContext(
		recognizer,
		dfa,
		startIndex,
		stopIndex,
		conflictingAlts,
		configs,
	)
}

// ReportContextSensitivity reports a context sensitivity.
func (*ParseErrorListener) ReportContextSensitivity(
	recognizer antlr.Parser,
	dfa *antlr.DFA,
	startIndex, stopIndex, prediction int,
	configs *antlr.ATNConfigSet,
) {
	antlr.ConsoleErrorListenerINSTANCE.ReportContextSensitivity(recognizer, dfa, startIndex, stopIndex, prediction, configs)
}
