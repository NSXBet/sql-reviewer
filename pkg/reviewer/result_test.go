package reviewer

import (
	"testing"

	"github.com/nsxbet/sql-reviewer/pkg/types"
)

func TestReviewResult_HasErrors(t *testing.T) {
	tests := []struct {
		name     string
		result   *ReviewResult
		expected bool
	}{
		{
			name: "no errors",
			result: &ReviewResult{
				Summary: Summary{Errors: 0, Warnings: 2},
			},
			expected: false,
		},
		{
			name: "has errors",
			result: &ReviewResult{
				Summary: Summary{Errors: 1, Warnings: 0},
			},
			expected: true,
		},
		{
			name: "multiple errors",
			result: &ReviewResult{
				Summary: Summary{Errors: 5, Warnings: 3},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.result.HasErrors()
			if got != tt.expected {
				t.Errorf("HasErrors() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestReviewResult_HasWarnings(t *testing.T) {
	tests := []struct {
		name     string
		result   *ReviewResult
		expected bool
	}{
		{
			name: "no warnings",
			result: &ReviewResult{
				Summary: Summary{Errors: 1, Warnings: 0},
			},
			expected: false,
		},
		{
			name: "has warnings",
			result: &ReviewResult{
				Summary: Summary{Errors: 0, Warnings: 1},
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.result.HasWarnings()
			if got != tt.expected {
				t.Errorf("HasWarnings() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestReviewResult_IsClean(t *testing.T) {
	tests := []struct {
		name     string
		result   *ReviewResult
		expected bool
	}{
		{
			name: "clean",
			result: &ReviewResult{
				Summary: Summary{Errors: 0, Warnings: 0, Success: 5},
			},
			expected: true,
		},
		{
			name: "has errors",
			result: &ReviewResult{
				Summary: Summary{Errors: 1, Warnings: 0},
			},
			expected: false,
		},
		{
			name: "has warnings",
			result: &ReviewResult{
				Summary: Summary{Errors: 0, Warnings: 1},
			},
			expected: false,
		},
		{
			name: "has both",
			result: &ReviewResult{
				Summary: Summary{Errors: 1, Warnings: 1},
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.result.IsClean()
			if got != tt.expected {
				t.Errorf("IsClean() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestReviewResult_String(t *testing.T) {
	result := &ReviewResult{
		Summary: Summary{
			Total:    10,
			Errors:   3,
			Warnings: 5,
			Success:  2,
		},
	}

	str := result.String()
	expected := "Review Results: 10 total (3 errors, 5 warnings, 2 success)"
	if str != expected {
		t.Errorf("String() = %q, want %q", str, expected)
	}
}

func TestReviewResult_FilterByStatus(t *testing.T) {
	advices := []*types.Advice{
		{Status: types.Advice_ERROR, Title: "Error 1"},
		{Status: types.Advice_WARNING, Title: "Warning 1"},
		{Status: types.Advice_ERROR, Title: "Error 2"},
		{Status: types.Advice_SUCCESS, Title: "Success 1"},
		{Status: types.Advice_WARNING, Title: "Warning 2"},
	}

	result := &ReviewResult{Advices: advices}

	tests := []struct {
		name           string
		status         types.Advice_Status
		expectedCount  int
		expectedTitles []string
	}{
		{
			name:           "filter errors",
			status:         types.Advice_ERROR,
			expectedCount:  2,
			expectedTitles: []string{"Error 1", "Error 2"},
		},
		{
			name:           "filter warnings",
			status:         types.Advice_WARNING,
			expectedCount:  2,
			expectedTitles: []string{"Warning 1", "Warning 2"},
		},
		{
			name:           "filter success",
			status:         types.Advice_SUCCESS,
			expectedCount:  1,
			expectedTitles: []string{"Success 1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := result.FilterByStatus(tt.status)
			if len(filtered) != tt.expectedCount {
				t.Errorf("FilterByStatus() returned %d items, want %d", len(filtered), tt.expectedCount)
			}

			for i, advice := range filtered {
				if advice.Title != tt.expectedTitles[i] {
					t.Errorf("FilterByStatus()[%d].Title = %q, want %q", i, advice.Title, tt.expectedTitles[i])
				}
			}
		})
	}
}

func TestReviewResult_FilterByCode(t *testing.T) {
	advices := []*types.Advice{
		{Code: 100, Title: "Code 100 - 1"},
		{Code: 200, Title: "Code 200 - 1"},
		{Code: 100, Title: "Code 100 - 2"},
		{Code: 300, Title: "Code 300 - 1"},
	}

	result := &ReviewResult{Advices: advices}

	tests := []struct {
		name          string
		code          int32
		expectedCount int
	}{
		{
			name:          "code 100",
			code:          100,
			expectedCount: 2,
		},
		{
			name:          "code 200",
			code:          200,
			expectedCount: 1,
		},
		{
			name:          "code 999 (not found)",
			code:          999,
			expectedCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filtered := result.FilterByCode(tt.code)
			if len(filtered) != tt.expectedCount {
				t.Errorf("FilterByCode() returned %d items, want %d", len(filtered), tt.expectedCount)
			}

			// Verify all filtered items have the correct code
			for _, advice := range filtered {
				if advice.Code != tt.code {
					t.Errorf("FilterByCode() returned advice with code %d, want %d", advice.Code, tt.code)
				}
			}
		})
	}
}

func TestCalculateSummary(t *testing.T) {
	tests := []struct {
		name     string
		advices  []*types.Advice
		expected Summary
	}{
		{
			name:    "empty advices",
			advices: []*types.Advice{},
			expected: Summary{
				Total:    0,
				Errors:   0,
				Warnings: 0,
				Success:  0,
			},
		},
		{
			name: "mixed advices",
			advices: []*types.Advice{
				{Status: types.Advice_ERROR},
				{Status: types.Advice_ERROR},
				{Status: types.Advice_WARNING},
				{Status: types.Advice_SUCCESS},
			},
			expected: Summary{
				Total:    4,
				Errors:   2,
				Warnings: 1,
				Success:  1,
			},
		},
		{
			name: "only errors",
			advices: []*types.Advice{
				{Status: types.Advice_ERROR},
				{Status: types.Advice_ERROR},
				{Status: types.Advice_ERROR},
			},
			expected: Summary{
				Total:    3,
				Errors:   3,
				Warnings: 0,
				Success:  0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := calculateSummary(tt.advices)
			if got.Total != tt.expected.Total {
				t.Errorf("Total = %d, want %d", got.Total, tt.expected.Total)
			}
			if got.Errors != tt.expected.Errors {
				t.Errorf("Errors = %d, want %d", got.Errors, tt.expected.Errors)
			}
			if got.Warnings != tt.expected.Warnings {
				t.Errorf("Warnings = %d, want %d", got.Warnings, tt.expected.Warnings)
			}
			if got.Success != tt.expected.Success {
				t.Errorf("Success = %d, want %d", got.Success, tt.expected.Success)
			}
		})
	}
}

func TestReviewResult_EmptyResult(t *testing.T) {
	result := &ReviewResult{
		Advices: []*types.Advice{},
		Summary: Summary{},
	}

	if !result.IsClean() {
		t.Error("Empty result should be clean")
	}

	if result.HasErrors() {
		t.Error("Empty result should not have errors")
	}

	if result.HasWarnings() {
		t.Error("Empty result should not have warnings")
	}

	str := result.String()
	expected := "Review Results: 0 total (0 errors, 0 warnings, 0 success)"
	if str != expected {
		t.Errorf("String() = %q, want %q", str, expected)
	}
}

func TestReviewResult_FilterByStatus_EmptyResult(t *testing.T) {
	result := &ReviewResult{
		Advices: []*types.Advice{},
	}

	filtered := result.FilterByStatus(types.Advice_ERROR)
	if len(filtered) != 0 {
		t.Errorf("FilterByStatus on empty result should return empty slice, got %d items", len(filtered))
	}

	if filtered == nil {
		t.Error("FilterByStatus should return non-nil slice, even if empty")
	}
}

func TestReviewResult_FilterByCode_EmptyResult(t *testing.T) {
	result := &ReviewResult{
		Advices: []*types.Advice{},
	}

	filtered := result.FilterByCode(100)
	if len(filtered) != 0 {
		t.Errorf("FilterByCode on empty result should return empty slice, got %d items", len(filtered))
	}

	if filtered == nil {
		t.Error("FilterByCode should return non-nil slice, even if empty")
	}
}
