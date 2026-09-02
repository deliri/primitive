package reviewcontrol

import (
	"errors"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
)

const (
	ContractTitleMaximumBytes       = 160
	ProblemStatementMaximumBytes    = 16 << 10
	CompletionStatementMaximumBytes = 16 << 10
	FindingSummaryMaximumBytes      = 512
	FindingDetailMaximumBytes       = 16 << 10
	DecisionReasonMaximumBytes      = 4 << 10
)

type boundedText struct{ value string }
type ContractTitle struct{ boundedText }
type ProblemStatement struct{ boundedText }
type CompletionStatement struct{ boundedText }
type FindingSummary struct{ boundedText }
type FindingDetail struct{ boundedText }
type DecisionReason struct{ boundedText }

func newBoundedText(value string, maximum int) (boundedText, error) {
	candidate := boundedText{value: value}
	if err := candidate.validate(maximum); err != nil {
		return boundedText{}, err
	}
	return candidate, nil
}

func (t boundedText) validate(maximum int) error {
	if len(t.value) == 0 || len(t.value) > maximum || !utf8.ValidString(t.value) || strings.TrimSpace(t.value) != t.value {
		return contractError(errors.New("review control text is invalid"))
	}
	for _, current := range t.value {
		if unicode.IsControl(current) && current != '\n' && current != '\t' {
			return contractError(errors.New("review control text contains a control character"))
		}
	}
	return nil
}

func NewContractTitle(value string) (ContractTitle, error) {
	v, e := newBoundedText(value, ContractTitleMaximumBytes)
	return ContractTitle{v}, e
}
func NewProblemStatement(value string) (ProblemStatement, error) {
	v, e := newBoundedText(value, ProblemStatementMaximumBytes)
	return ProblemStatement{v}, e
}
func NewCompletionStatement(value string) (CompletionStatement, error) {
	v, e := newBoundedText(value, CompletionStatementMaximumBytes)
	return CompletionStatement{v}, e
}
func NewFindingSummary(value string) (FindingSummary, error) {
	v, e := newBoundedText(value, FindingSummaryMaximumBytes)
	return FindingSummary{v}, e
}
func NewFindingDetail(value string) (FindingDetail, error) {
	v, e := newBoundedText(value, FindingDetailMaximumBytes)
	return FindingDetail{v}, e
}
func NewDecisionReason(value string) (DecisionReason, error) {
	v, e := newBoundedText(value, DecisionReasonMaximumBytes)
	return DecisionReason{v}, e
}

func (t ContractTitle) Validate() error       { return t.validate(ContractTitleMaximumBytes) }
func (t ProblemStatement) Validate() error    { return t.validate(ProblemStatementMaximumBytes) }
func (t CompletionStatement) Validate() error { return t.validate(CompletionStatementMaximumBytes) }
func (t FindingSummary) Validate() error      { return t.validate(FindingSummaryMaximumBytes) }
func (t FindingDetail) Validate() error       { return t.validate(FindingDetailMaximumBytes) }
func (t DecisionReason) Validate() error      { return t.validate(DecisionReasonMaximumBytes) }

func marshalBoundedText(t boundedText, validate func() error) ([]byte, error) {
	if err := validate(); err != nil {
		return nil, jsonError(err)
	}
	return core.MarshalCanonicalJSONString(t.value)
}

func decodeBoundedText(data []byte, maximum int) (boundedText, error) {
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return boundedText{}, jsonError(err)
	}
	candidate, err := newBoundedText(value, maximum)
	if err != nil {
		return boundedText{}, jsonError(err)
	}
	return candidate, nil
}

func (t ContractTitle) MarshalJSON() ([]byte, error) {
	return marshalBoundedText(t.boundedText, t.Validate)
}
func (t ProblemStatement) MarshalJSON() ([]byte, error) {
	return marshalBoundedText(t.boundedText, t.Validate)
}
func (t CompletionStatement) MarshalJSON() ([]byte, error) {
	return marshalBoundedText(t.boundedText, t.Validate)
}
func (t FindingSummary) MarshalJSON() ([]byte, error) {
	return marshalBoundedText(t.boundedText, t.Validate)
}
func (t FindingDetail) MarshalJSON() ([]byte, error) {
	return marshalBoundedText(t.boundedText, t.Validate)
}
func (t DecisionReason) MarshalJSON() ([]byte, error) {
	return marshalBoundedText(t.boundedText, t.Validate)
}

func (t *ContractTitle) UnmarshalJSON(data []byte) error {
	if t == nil {
		return jsonError()
	}
	v, e := decodeBoundedText(data, ContractTitleMaximumBytes)
	if e == nil {
		t.boundedText = v
	}
	return e
}
func (t *ProblemStatement) UnmarshalJSON(data []byte) error {
	if t == nil {
		return jsonError()
	}
	v, e := decodeBoundedText(data, ProblemStatementMaximumBytes)
	if e == nil {
		t.boundedText = v
	}
	return e
}
func (t *CompletionStatement) UnmarshalJSON(data []byte) error {
	if t == nil {
		return jsonError()
	}
	v, e := decodeBoundedText(data, CompletionStatementMaximumBytes)
	if e == nil {
		t.boundedText = v
	}
	return e
}
func (t *FindingSummary) UnmarshalJSON(data []byte) error {
	if t == nil {
		return jsonError()
	}
	v, e := decodeBoundedText(data, FindingSummaryMaximumBytes)
	if e == nil {
		t.boundedText = v
	}
	return e
}
func (t *FindingDetail) UnmarshalJSON(data []byte) error {
	if t == nil {
		return jsonError()
	}
	v, e := decodeBoundedText(data, FindingDetailMaximumBytes)
	if e == nil {
		t.boundedText = v
	}
	return e
}
func (t *DecisionReason) UnmarshalJSON(data []byte) error {
	if t == nil {
		return jsonError()
	}
	v, e := decodeBoundedText(data, DecisionReasonMaximumBytes)
	if e == nil {
		t.boundedText = v
	}
	return e
}
