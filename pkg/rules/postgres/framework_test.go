package postgres

import (
	"errors"
	"testing"

	"github.com/nsxbet/sql-reviewer/pkg/pgparser"
	"github.com/nsxbet/sql-reviewer/pkg/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertSyntaxErrorToAdvice(t *testing.T) {
	tests := []struct {
		name           string
		err            error
		wantAdviceCode int32
		wantTitle      string
		wantStatus     types.Advice_Status
		wantContent    string
		checkPosition  bool
		wantLine       int32
		wantColumn     int32
	}{
		{
			name: "pgparser.SyntaxError with position",
			err: &pgparser.SyntaxError{
				Message: `syntax error at line 5, column 10: syntax error at or near "INVALID"`,
				Position: &types.Position{
					Line:   5,
					Column: 10,
				},
			},
			wantAdviceCode: int32(types.StatementSyntaxError),
			wantTitle:      "Syntax error",
			wantStatus:     types.Advice_ERROR,
			wantContent:    `syntax error at line 5, column 10: syntax error at or near "INVALID"`,
			checkPosition:  true,
			wantLine:       5,
			wantColumn:     10,
		},
		{
			name: "pgparser.SyntaxError without position",
			err: &pgparser.SyntaxError{
				Message: "failed to parse SQL statement",
			},
			wantAdviceCode: int32(types.StatementSyntaxError),
			wantTitle:      "Syntax error",
			wantStatus:     types.Advice_ERROR,
			wantContent:    "failed to parse SQL statement",
			checkPosition:  false,
		},
		{
			name:           "generic error",
			err:            errors.New("unexpected parse failure"),
			wantAdviceCode: int32(types.Internal),
			wantTitle:      "Parse error",
			wantStatus:     types.Advice_ERROR,
			wantContent:    "unexpected parse failure",
			checkPosition:  false,
		},
		{
			name: "pgparser.SyntaxError with zero position",
			err: &pgparser.SyntaxError{
				Message: "syntax error at beginning",
				Position: &types.Position{
					Line:   0,
					Column: 0,
				},
			},
			wantAdviceCode: int32(types.StatementSyntaxError),
			wantTitle:      "Syntax error",
			wantStatus:     types.Advice_ERROR,
			wantContent:    "syntax error at beginning",
			checkPosition:  true,
			wantLine:       0,
			wantColumn:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			advices, err := ConvertSyntaxErrorToAdvice(tt.err)

			// Should never return an error
			assert.NoError(t, err, "ConvertSyntaxErrorToAdvice should never return an error")

			// Should always return exactly one advice
			require.Len(t, advices, 1, "Should return exactly one advice")

			advice := advices[0]

			// Verify advice properties
			assert.Equal(t, tt.wantStatus, advice.Status, "Status should match")
			assert.Equal(t, tt.wantAdviceCode, advice.Code, "Code should match")
			assert.Equal(t, tt.wantTitle, advice.Title, "Title should match")
			assert.Equal(t, tt.wantContent, advice.Content, "Content should match")

			// Verify position information if expected
			if tt.checkPosition {
				require.NotNil(t, advice.StartPosition, "StartPosition should not be nil")
				assert.Equal(t, tt.wantLine, advice.StartPosition.Line, "Line should match")
				assert.Equal(t, tt.wantColumn, advice.StartPosition.Column, "Column should match")
			} else {
				// For generic errors, position may be nil
				if advice.StartPosition != nil {
					t.Logf("Note: StartPosition is %+v (not required but present)", advice.StartPosition)
				}
			}
		})
	}
}

func TestConvertSyntaxErrorToAdvice_ErrorCodes(t *testing.T) {
	t.Run("syntax error code is 201", func(t *testing.T) {
		err := &pgparser.SyntaxError{
			Message: "syntax error",
		}
		advices, _ := ConvertSyntaxErrorToAdvice(err)
		require.Len(t, advices, 1)
		assert.Equal(t, int32(201), advices[0].Code, "Syntax error code should be 201")
	})

	t.Run("internal error code is 1", func(t *testing.T) {
		err := errors.New("internal error")
		advices, _ := ConvertSyntaxErrorToAdvice(err)
		require.Len(t, advices, 1)
		assert.Equal(t, int32(1), advices[0].Code, "Internal error code should be 1")
	})
}

func TestConvertSyntaxErrorToAdvice_AlwaysReturnsNilError(t *testing.T) {
	testCases := []error{
		&pgparser.SyntaxError{Message: "test"},
		errors.New("generic error"),
		nil, // Even nil error should be handled gracefully
	}

	for i, testErr := range testCases {
		t.Run(string(rune('A'+i)), func(t *testing.T) {
			_, err := ConvertSyntaxErrorToAdvice(testErr)
			assert.NoError(t, err, "Should never return an error")
		})
	}
}

func TestConvertSyntaxErrorToAdvice_MatchesMySQLPattern(t *testing.T) {
	// This test verifies that the PostgreSQL implementation matches the MySQL pattern
	t.Run("follows MySQL conversion pattern", func(t *testing.T) {
		syntaxErr := &pgparser.SyntaxError{
			Message: "test syntax error",
			Position: &types.Position{
				Line:   10,
				Column: 20,
			},
		}

		advices, err := ConvertSyntaxErrorToAdvice(syntaxErr)

		// Verify MySQL-compatible behavior
		assert.NoError(t, err, "Should return nil error like MySQL")
		require.Len(t, advices, 1, "Should return exactly one advice like MySQL")

		advice := advices[0]
		assert.Equal(t, types.Advice_ERROR, advice.Status, "Should use ERROR status like MySQL")
		assert.Equal(t, int32(types.StatementSyntaxError), advice.Code, "Should use code 201 like MySQL")
		assert.NotEmpty(t, advice.Title, "Should have a title like MySQL")
		assert.NotEmpty(t, advice.Content, "Should have content like MySQL")
		assert.NotNil(t, advice.StartPosition, "Should preserve position like MySQL")
	})
}
