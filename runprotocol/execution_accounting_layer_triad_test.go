package runprotocol_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runprotocol"
)

func TestExecutionAccountingLayerTriadPreservesAppendOnlyAttempts(t *testing.T) {
	t.Parallel()

	t.Run("positive failed first attempt remains before the passing retry", func(t *testing.T) {
		t.Parallel()
		got := runprotocol.ExecutionAccounting{Attempts: []runprotocol.ExecutionAttempt{
			{Sequence: 1, Planned: 2, Failed: 1, NotRun: 1, Cache: runprotocol.CacheDisabled},
			{Sequence: 2, Planned: 2, Passed: 2, Cache: runprotocol.CacheDisabled},
		}}
		gotErr := got.Validate()
		latest, present := got.Latest()
		if gotErr != nil || !present || len(got.Attempts) != 2 || got.Attempts[0].Failed != 1 || latest.Sequence != 2 || latest.Passed != 2 {
			t.Fatalf("ExecutionAccounting.Validate(retry) = (error %v, present %t, attempts %+v), want retained failed attempt then passing retry", gotErr, present, got.Attempts)
		}
	})

	t.Run("negative retry cannot change the planned denominator", func(t *testing.T) {
		t.Parallel()
		got := runprotocol.ExecutionAccounting{Attempts: []runprotocol.ExecutionAttempt{
			{Sequence: 1, Planned: 1, Failed: 1, Cache: runprotocol.CacheDisabled},
			{Sequence: 2, Planned: 2, Passed: 2, Cache: runprotocol.CacheDisabled},
		}}
		gotErr := got.Validate()
		if !errors.Is(gotErr, core.ErrRunProtocolConflict) {
			t.Fatalf("ExecutionAccounting.Validate(changed retry denominator) error = %v, want errors.Is(..., %v)", gotErr, core.ErrRunProtocolConflict)
		}
	})

	t.Run("neutral absence is not represented as a successful empty attempt list", func(t *testing.T) {
		t.Parallel()
		got := runprotocol.ExecutionAccounting{Attempts: []runprotocol.ExecutionAttempt{}}
		gotErr := got.Validate()
		latest, present := got.Latest()
		if !errors.Is(gotErr, core.ErrRunProtocolContract) || present || latest != (runprotocol.ExecutionAttempt{}) {
			t.Fatalf("ExecutionAccounting(empty) = (latest %+v, present %t, error %v), want zero, false, errors.Is(..., %v)", latest, present, gotErr, core.ErrRunProtocolContract)
		}
	})
}
