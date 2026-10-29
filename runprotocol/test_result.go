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

// RunResult is an actual native result, not a name-based classification. A
// package has no synthetic test name; a benchmark row has no invented test
// outcome. The enclosing RunFact owns its occurrence ordinal. PackagePath is
// optional actual producer-resolved attribution, never module-prefix guessing.
type RunResult struct {
	Kind        RunResultKind      `json:"kind"`
	Suite       TestLabel          `json:"suite"`
	Name        *TestLabel         `json:"name,omitempty"`
	PackagePath *core.RelativePath `json:"package_path,omitempty"`
	Elapsed     *temporal.Duration `json:"elapsed,omitempty"`
	Outcome     *Outcome           `json:"outcome,omitempty"`
	Benchmark   *GoBenchmarkRow    `json:"benchmark,omitempty"`
}

func (r RunResult) Validate() error {
	if err := contractJoin(r.Kind.Validate(), r.Suite.Validate()); err != nil {
		return err
	}
	if r.PackagePath != nil {
		if err := r.PackagePath.Validate(); err != nil {
			return contractError(err)
		}
	}
	if r.Elapsed != nil {
		if err := r.Elapsed.Validate(); err != nil {
			return contractError(err)
		}
	}
	switch r.Kind {
	case RunResultPackage:
		if r.Name != nil || r.Benchmark != nil {
			return conflictError(nil)
		}
	case RunResultTest, RunResultFuzz:
		if r.Name == nil {
			return core.ErrRunProtocolContract
		}
		if err := r.Name.Validate(); err != nil {
			return err
		}
		if r.Kind == RunResultFuzz {
			if _, err := ParseGoFuzzTarget(r.Name.String()); err != nil {
				return contractError(err)
			}
		}
		if r.Benchmark != nil {
			return conflictError(nil)
		}
	case RunResultBenchmark:
		if r.Benchmark == nil || r.Outcome != nil {
			return conflictError(nil)
		}
		if err := r.Benchmark.Validate(); err != nil {
			return err
		}
		if r.Name != nil {
			if err := r.Name.Validate(); err != nil {
				return err
			}
		}
		if r.Benchmark.Measurement != nil && (r.Name == nil || r.Name.String() != r.Benchmark.Measurement.Name.String()) {
			return conflictError(nil)
		}
		return nil
	default:
		return core.ErrRunProtocolContract
	}
	if r.Outcome == nil {
		return core.ErrRunProtocolContract
	}
	switch *r.Outcome {
	case OutcomePassed, OutcomeFailed, OutcomeSkipped:
		return nil
	case OutcomeSetupFailed:
		if r.Kind == RunResultPackage {
			return nil
		}
		return conflictError(nil)
	default:
		return conflictError(nil)
	}
}

func (TestLabel) runProtocolFact()    {}
func (TestIdentity) runProtocolFact() {}
func (RunResult) runProtocolFact()    {}
