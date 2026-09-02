package reviewcontrol

import (
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type textContractCase struct {
	name      string
	maximum   int
	construct func(string) error
}

func reviewTextContracts() []textContractCase {
	return []textContractCase{
		{name: "contract title", maximum: ContractTitleMaximumBytes, construct: func(value string) error { _, err := NewContractTitle(value); return err }},
		{name: "problem statement", maximum: ProblemStatementMaximumBytes, construct: func(value string) error { _, err := NewProblemStatement(value); return err }},
		{name: "completion statement", maximum: CompletionStatementMaximumBytes, construct: func(value string) error { _, err := NewCompletionStatement(value); return err }},
		{name: "finding summary", maximum: FindingSummaryMaximumBytes, construct: func(value string) error { _, err := NewFindingSummary(value); return err }},
		{name: "finding detail", maximum: FindingDetailMaximumBytes, construct: func(value string) error { _, err := NewFindingDetail(value); return err }},
		{name: "decision reason", maximum: DecisionReasonMaximumBytes, construct: func(value string) error { _, err := NewDecisionReason(value); return err }},
	}
}

func TestReviewControlTextAdmitsTenDistinctValidFormsAndRefusesTenInvalidForms(t *testing.T) {
	t.Parallel()
	valid := []struct {
		name  string
		value string
	}{
		{name: "single ASCII byte", value: "x"},
		{name: "ordinary sentence", value: "Exact source remains bound."},
		{name: "internal spaces", value: "one  two"},
		{name: "internal tab", value: "one\ttwo"},
		{name: "internal newline", value: "one\ntwo"},
		{name: "Unicode scalar", value: "evidence ✓"},
		{name: "combining sequence", value: "Cafe\u0301"},
		{name: "punctuation", value: "commit:path@sha256"},
		{name: "numeric facts", value: "sequence 1 of 2"},
		{name: "non-Latin text", value: "証拠"},
	}
	invalid := []struct {
		name  string
		value string
	}{
		{name: "empty", value: ""},
		{name: "space only", value: " "},
		{name: "leading space", value: " leading"},
		{name: "trailing space", value: "trailing "},
		{name: "newline only", value: "\n"},
		{name: "tab only", value: "\t"},
		{name: "NUL control", value: "a\x00b"},
		{name: "carriage return control", value: "a\rb"},
		{name: "delete control", value: "a\x7fb"},
		{name: "invalid UTF-8", value: string([]byte{0xff})},
	}
	for _, contract := range reviewTextContracts() {
		t.Run(contract.name+" valid forms", func(t *testing.T) {
			t.Parallel()
			for _, tc := range valid {
				if gotErr := contract.construct(tc.value); gotErr != nil {
					t.Fatalf("%s constructor(%s) error = %v, want nil", contract.name, tc.name, gotErr)
				}
			}
		})
		t.Run(contract.name+" invalid forms", func(t *testing.T) {
			t.Parallel()
			for _, tc := range invalid {
				if gotErr := contract.construct(tc.value); !errors.Is(gotErr, core.ErrReviewControlContract) {
					t.Fatalf("%s constructor(%s) error = %v, want %v", contract.name, tc.name, gotErr, core.ErrReviewControlContract)
				}
			}
		})
	}
}

func TestReviewControlTextPressuresEveryOwnedByteCeiling(t *testing.T) {
	t.Parallel()
	for _, contract := range reviewTextContracts() {
		cases := []struct {
			name      string
			value     string
			wantValid bool
		}{
			{name: "one below maximum", value: strings.Repeat("x", contract.maximum-1), wantValid: true},
			{name: "exact maximum", value: strings.Repeat("x", contract.maximum), wantValid: true},
			{name: "one above maximum", value: strings.Repeat("x", contract.maximum+1), wantValid: false},
			{name: "twice maximum", value: strings.Repeat("x", contract.maximum*2), wantValid: false},
		}
		for _, tc := range cases {
			t.Run(contract.name+" "+tc.name, func(t *testing.T) {
				t.Parallel()
				gotErr := contract.construct(tc.value)
				if (gotErr == nil) != tc.wantValid {
					t.Fatalf("%s constructor(%s) error = %v, want valid=%t", contract.name, tc.name, gotErr, tc.wantValid)
				}
			})
		}
	}
}
