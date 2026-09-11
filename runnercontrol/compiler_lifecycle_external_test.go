package runnercontrol_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runnercontrol"
)

func TestGoObservationCompilerClosedAndZeroStates(t *testing.T) {
	t.Parallel()
	data := events(event("pass", "selected", "", ""))(t)[0]
	var zero runnercontrol.GoTestObservationCompiler
	if n, err := zero.Write(data); n != 0 || !errors.Is(err, core.ErrPrimitiveContract) {
		t.Fatalf("zero.Write() = %d/%v, want 0/typed refusal", n, err)
	}
	compiler, err := runnercontrol.NewGoTestObservationCompiler(policy(1, false))
	if err != nil {
		t.Fatalf("NewGoTestObservationCompiler() error = %v, want nil", err)
	}
	if _, err := compiler.Write(data); err != nil {
		t.Fatalf("Write() error = %v, want nil", err)
	}
	if _, err := compiler.Seal(nil); err != nil {
		t.Fatalf("Seal() error = %v, want nil", err)
	}
	if n, err := compiler.Write(data); n != 0 || !errors.Is(err, core.ErrPrimitiveContract) {
		t.Fatalf("sealed.Write() = %d/%v, want 0/typed refusal", n, err)
	}
	if got, err := compiler.Seal(nil); !errors.Is(err, core.ErrPrimitiveContract) || len(got.Accounting.Attempts) != 0 {
		t.Fatalf("sealed.Seal() = %+v/%v, want zero/typed refusal", got, err)
	}
}

func TestGoCoverageCompilerCannotChangeAfterSeal(t *testing.T) {
	t.Parallel()
	compiler := runnercontrol.NewGoCoverageCompiler()
	if _, err := compiler.Write([]byte("mode: set\na.go:1.1,1.2 1 1\n")); err != nil {
		t.Fatalf("Write() error = %v, want nil", err)
	}
	got, err := compiler.Seal()
	if err != nil || got.Statements != 1 {
		t.Fatalf("Seal() = %+v/%v, want one statement/nil", got, err)
	}
	if n, err := compiler.Write([]byte("a.go:1.1,1.2 1 1\n")); n != 0 || !errors.Is(err, core.ErrPrimitiveContract) {
		t.Fatalf("sealed.Write() = %d/%v, want 0/typed refusal", n, err)
	}
	if got, err := compiler.Seal(); !errors.Is(err, core.ErrPrimitiveContract) || got != (runnercontrol.GoCoverageObservation{}) {
		t.Fatalf("sealed.Seal() = %+v/%v, want zero/typed refusal", got, err)
	}
}
