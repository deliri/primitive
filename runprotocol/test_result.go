package runprotocol

import (
	"errors"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// TestIdentity names exactly what the producer observed. Suite is the native
// package/file/suite identity; Name includes any observed subtest suffix.
// Labels are data, not selectors or paths to execute. No source-file ownership
// or intermediate parent is inferred from a slash in a dynamic name.
// A whole value owns its label allocation; streaming callers retain only the
// current result, never a population of identities.
type TestIdentity struct {
	Suite string `json:"suite"`
	Name  string `json:"name"`
}

func (i TestIdentity) Validate() error {
	if i.Suite == "" || i.Name == "" || !utf8.ValidString(i.Suite) || !utf8.ValidString(i.Name) {
		return core.ErrRunProtocolContract
	}
	return nil
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

func (TestIdentity) runProtocolFact() {}
func (TestResult) runProtocolFact()   {}
