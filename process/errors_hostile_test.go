package process

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func failureCommandFixture(t *testing.T) core.AbsolutePath {
	t.Helper()
	command, err := core.ParseAbsolutePath(filepath.Join(string(filepath.Separator), "command"))
	if err != nil {
		t.Fatalf("core.ParseAbsolutePath() error = %v, want nil", err)
	}
	return command
}

func failureLimitFixture(t *testing.T) core.ByteCount {
	t.Helper()
	limit, err := core.NewByteCount(8)
	if err != nil {
		t.Fatalf("core.NewByteCount() error = %v, want nil", err)
	}
	return limit
}

// TestSealedFailureConstructorsRejectUnprovedIdentities proves no invalid
// Process report can carry a stable phase, stream, or output-limit identity.
func TestSealedFailureConstructorsRejectUnprovedIdentities(t *testing.T) {
	t.Parallel()

	command := failureCommandFixture(t)
	limit := failureLimitFixture(t)
	native := errors.New("native process failure")
	cases := []struct {
		construct func() error
		identity  error
		name      string
	}{
		{name: "unknown process phase", construct: func() error { return newFailure(FailureKindUnknown, command, native) }, identity: core.ErrProcessWait},
		{name: "unset process command", construct: func() error { return newFailure(FailureKindStart, core.AbsolutePath{}, native) }, identity: core.ErrProcessStart},
		{name: "nil process cause", construct: func() error { return newFailure(FailureKindWait, command, nil) }, identity: core.ErrProcessWait},
		{name: "unknown stream", construct: func() error { return newStreamFailure(StreamUnknown, native) }, identity: core.ErrProcessStream},
		{name: "nil stream cause", construct: func() error { return newStreamFailure(StreamStdout, nil) }, identity: core.ErrProcessStream},
		{name: "stdin output limit", construct: func() error { return newOutputLimitExceeded(StreamStdin, limit) }, identity: core.ErrProcessOutputLimit},
		{name: "unset output limit", construct: func() error { return newOutputLimitExceeded(StreamStdout, core.ByteCount{}) }, identity: core.ErrProcessOutputLimit},
	}
	for _, tc := range cases {
		rejection := tc.construct()
		if !errors.Is(rejection, core.ErrProcessContract) || errors.Is(rejection, tc.identity) {
			t.Errorf("%s constructor error = %v, want contract without %v", tc.name, rejection, tc.identity)
		}
	}
	failureSpecialized := errors.Is(failure{}, core.ErrProcessWait)
	streamSpecialized := errors.Is(streamFailure{}, core.ErrProcessStream)
	limitSpecialized := errors.Is(
		outputLimitExceeded{},
		core.ErrProcessOutputLimit,
	)
	if failureSpecialized || streamSpecialized || limitSpecialized {
		t.Fatalf(
			"zero internal Process specialized identities = (failure=%t, stream=%t, limit=%t), want all false",
			failureSpecialized,
			streamSpecialized,
			limitSpecialized,
		)
	}
}

// TestSealedFailureFactsAndNativeCausesSurviveConstruction proves each valid
// Process-built report remains recoverable by shape and preserves every fact a
// caller needs for a typed decision.
func TestSealedFailureFactsAndNativeCausesSurviveConstruction(t *testing.T) {
	t.Parallel()

	command := failureCommandFixture(t)
	limit := failureLimitFixture(t)
	native := errors.New("native process failure")
	processErr := newFailure(FailureKindStart, command, native)
	var processFailure Failure
	if !errors.Is(processErr, core.ErrProcessStart) ||
		!errors.Is(processErr, core.ErrProcessContract) ||
		!errors.Is(processErr, native) ||
		!errors.As(processErr, &processFailure) {
		t.Fatalf("newFailure() error = %v, want typed start, contract, and native identities", processErr)
	}
	if processFailure.Kind() != FailureKindStart ||
		processFailure.Command() != command ||
		processFailure.Cause() != native {
		t.Fatalf("Failure facts = (%v, %v, %v), want exact constructor facts", processFailure.Kind(), processFailure.Command(), processFailure.Cause())
	}

	limitErr := newOutputLimitExceeded(StreamStderr, limit)
	streamErr := newStreamFailure(StreamStderr, limitErr)
	var outputFailure OutputLimitExceeded
	var callerFailure StreamFailure
	if !errors.Is(streamErr, core.ErrProcessStream) ||
		!errors.Is(streamErr, core.ErrProcessOutputLimit) ||
		!errors.As(streamErr, &callerFailure) ||
		!errors.As(streamErr, &outputFailure) {
		t.Fatalf("nested stream error = %v, want stream and output-limit reports", streamErr)
	}
	if callerFailure.Stream() != StreamStderr || callerFailure.Cause() != limitErr {
		t.Fatalf("StreamFailure facts = (%v, %v), want stderr and exact cause", callerFailure.Stream(), callerFailure.Cause())
	}
	if outputFailure.Stream() != StreamStderr || outputFailure.Limit() != limit {
		t.Fatalf("OutputLimitExceeded facts = (%v, %v), want stderr and exact limit", outputFailure.Stream(), outputFailure.Limit())
	}
}
