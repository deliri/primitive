package runprotocol

import (
	"errors"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// TestLabel is one exact nonempty UTF-8 producer-observed label, otherwise
// uninterpreted. Native spaces, punctuation and control characters remain data.
// This whole value owns its allocation; there is no acceptance-length quota.
type TestLabel struct{ value string }

func NewTestLabel(value string) (TestLabel, error) {
	candidate := TestLabel{value: value}
	if err := candidate.Validate(); err != nil {
		return TestLabel{}, err
	}
	return candidate, nil
}

func (l TestLabel) Validate() error {
	if l.value == "" || !utf8.ValidString(l.value) {
		return core.ErrRunProtocolContract
	}
	return nil
}

func (l TestLabel) String() string               { return l.value }
func (l TestLabel) MarshalJSON() ([]byte, error) { return marshalScalar(l.value, l.Validate) }
func (l *TestLabel) UnmarshalJSON(data []byte) error {
	if l == nil {
		return jsonError(errors.New("nil run protocol test label receiver"))
	}
	return decodeScalar(data, func(raw string) error {
		candidate, err := NewTestLabel(raw)
		if err == nil {
			*l = candidate
		}
		return err
	})
}

// TestIdentity names exactly what the producer observed. Suite is the native
// package/file/suite identity; Name includes any observed subtest suffix.
// Labels are data, not selectors or paths to execute. No source-file ownership
// or intermediate parent is inferred from a slash in a dynamic name.
// Streaming callers retain only the current result, never a population of
// identities. Go's JSON parser and the current label own scalar allocation;
// constant sequence memory is not a bound on the largest individual label.
type TestIdentity struct {
	Suite TestLabel `json:"suite"`
	Name  TestLabel `json:"name"`
}

func (i TestIdentity) Validate() error {
	return errors.Join(i.Suite.Validate(), i.Name.Validate())
}

// TestResult is one producer-observed test/subtest result in a run's ordered
// result stream. Ordinal identifies the occurrence, so repeated names remain
// separate facts. It does not claim an inferred retry or parent attempt.
// Elapsed is absent when no duration was observed; a present zero is a real
// observation. A result alone never proves complete selection or acceptance.
// Producers use the existing Outcome domain, including explicit non-pass facts.
type TestResult struct {
	Test    TestIdentity       `json:"test"`
	Elapsed *temporal.Duration `json:"elapsed,omitempty"`
	Ordinal uint64             `json:"ordinal"`
	Outcome Outcome            `json:"outcome"`
}

func (r TestResult) Validate() error {
	if err := errors.Join(r.Test.Validate(), r.Outcome.Validate()); err != nil {
		return errors.Join(core.ErrRunProtocolContract, err)
	}
	if r.Ordinal == 0 {
		return core.ErrRunProtocolContract
	}
	if r.Elapsed != nil {
		if err := r.Elapsed.Validate(); err != nil {
			return errors.Join(core.ErrRunProtocolContract, err)
		}
	}
	return nil
}

func (TestLabel) runProtocolFact()    {}
func (TestIdentity) runProtocolFact() {}
func (TestResult) runProtocolFact()   {}
