package process_test

import (
	"bytes"
	"errors"
	"fmt"
	"math"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/process"
)

func TestExitStatusExhaustsPortableDomain(t *testing.T) {
	t.Parallel()
	for value := range core.ProcessExitStatusMaximum + 1 {
		if err := process.ExitStatus(value).Validate(); err != nil {
			t.Fatalf("normal exit status %d validation = %v, want nil", value, err)
		}
	}
	for _, value := range []int{math.MinInt, -1, core.ProcessExitStatusMaximum + 1, math.MaxInt} {
		status := process.ExitStatus(value)
		if err := status.Validate(); !errors.Is(err, core.ErrProcessContract) {
			t.Fatalf("unrepresentable status %d validation = %v, want %v", value, err, core.ErrProcessContract)
		}
	}
}

// The parent executes the actual self-termination leaf in a child. Its exact
// exit observation and absence of deferred output prove Go's terminal semantics.
func TestExitCurrentNativeLayerTriad(t *testing.T) {
	t.Parallel()
	for _, status := range []process.ExitStatus{-1, core.ProcessExitStatusMaximum + 1} {
		if err := process.ExitCurrent(status); !errors.Is(err, core.ErrProcessContract) {
			t.Fatalf("unrepresentable status %d self-exit = %v, want refusal without termination", status, err)
		}
	}
	for _, status := range []process.ExitStatus{process.ExitSuccess, process.ExitFailure, process.ExitUsage, 125, 254, core.ProcessExitStatusMaximum} {
		t.Run("normal-status-"+strconv.Itoa(int(status)), func(t *testing.T) {
			t.Parallel()
			var stdout, stderr bytes.Buffer
			request := processRequest(t, "self-exit:"+strconv.Itoa(int(status)), process.Streams{Stdin: bytes.NewReader(nil), Stdout: &stdout, Stderr: &stderr})
			result, err := process.Run(t.Context(), request)
			if err != nil {
				t.Fatalf("self-exit child execution = %v, want nil", err)
			}
			observed, observationErr := result.Observation()
			if observationErr != nil || observed.ExitCode != int64(status) || observed.TerminationSignal != nil || stdout.Len() != 0 || stderr.Len() != 0 {
				t.Fatalf("self-exit observation = (%+v, %v, stdout:%q, stderr:%q), want normal status %d and no deferred output", observed, observationErr, stdout.String(), stderr.String(), status)
			}
		})
	}
}

func helperExitCurrent(t *testing.T, text string) {
	t.Helper()
	value, err := strconv.Atoi(text)
	if err != nil {
		t.Fatalf("self-exit helper status = %v, want integer", err)
	}
	defer func() {
		streams, err := process.StandardStreams()
		if err != nil {
			t.Fatalf("deferred helper streams = %v, want nil", err)
		}
		if _, err := fmt.Fprintln(streams.Stdout, "deferred cleanup ran"); err != nil {
			t.Fatalf("deferred helper output = %v, want nil", err)
		}
	}()
	if err := process.ExitCurrent(process.ExitStatus(value)); err != nil {
		t.Fatalf("self-exit helper = %v, want terminal effect", err)
	}
}

func FuzzExitStatusValidation(f *testing.F) {
	for _, value := range []int{math.MinInt, -1, int(process.ExitSuccess), int(process.ExitFailure), int(process.ExitUsage), core.ProcessExitStatusMaximum, core.ProcessExitStatusMaximum + 1, math.MaxInt} {
		f.Add(value)
	}
	f.Fuzz(func(t *testing.T, value int) {
		status := process.ExitStatus(value)
		err := status.Validate()
		if value >= 0 && value <= core.ProcessExitStatusMaximum {
			if err != nil || int(status) != value {
				t.Fatalf("admitted normal status = (%d, %v), want (%d, nil)", status, err, value)
			}
			return // Native subprocess proof owns valid terminal execution.
		}
		if !errors.Is(err, core.ErrProcessContract) || !errors.Is(process.ExitCurrent(status), core.ErrProcessContract) {
			t.Fatalf("unrepresentable status %d refusal = %v, want %v and no termination", value, err, core.ErrProcessContract)
		}
	})
}
