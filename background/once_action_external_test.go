package background_test

import (
	"errors"
	"io"
	"runtime"
	"sync"
	"testing"
	"testing/synctest"

	"github.com/deliri/primitive/v2026/background"
	"github.com/deliri/primitive/v2026/core"
)

func TestOnceActionCopiesPreserveFirstNativeObservation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		cause       error
		panicAction bool
	}{
		{name: "successful action"},
		{name: "native source refusal", cause: io.ErrClosedPipe},
		{name: "joined native causes", cause: errors.Join(io.ErrClosedPipe, io.ErrUnexpectedEOF)},
		{name: "native panic remains a refusal", cause: io.ErrNoProgress, panicAction: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runs := 0
			action, err := background.NewOnceAction(background.OnceActionRequest{Run: func() error {
				runs++
				if tc.panicAction {
					panic(tc.cause)
				}
				return tc.cause
			}})
			if err != nil {
				t.Fatal(err)
			}
			copyOfAction := action
			first := action.Run()
			if !errors.Is(first, tc.cause) || errors.Is(first, core.ErrBackgroundCallbackPanic) != tc.panicAction {
				t.Fatalf("first native observation=%v cause=%v panic=%t", first, tc.cause, tc.panicAction)
			}
			if second := copyOfAction.Run(); second != first || runs != 1 {
				t.Fatalf("copied action changed first result=%v/%v executions=%d", first, second, runs)
			}
			if later := action.Run(); later != first || runs != 1 {
				t.Fatalf("later action replaced first observation=%v/%v executions=%d", first, later, runs)
			}
		})
	}
}

func TestOnceActionConcurrentCallersJoinBeforeReceivingFirstFailure(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		entered, release := make(chan struct{}), make(chan struct{})
		cleaned, runs := false, 0
		action, err := background.NewOnceAction(background.OnceActionRequest{Run: func() error {
			runs++
			defer func() { cleaned = true }()
			close(entered)
			<-release
			return io.ErrUnexpectedEOF
		}})
		if err != nil {
			t.Fatal(err)
		}
		results := make(chan error, 17)
		var joined sync.WaitGroup
		joined.Go(func() { copyOfAction := action; results <- copyOfAction.Run() })
		<-entered
		synctest.Wait()
		select {
		case result := <-results:
			t.Fatalf("action reported before cleanup=%v", result)
		default:
		}
		// Mutex waits are not durably blocked in a synctest bubble. Observe
		// the active action first, then contend and release through channels.
		for range 16 {
			joined.Go(func() { copyOfAction := action; results <- copyOfAction.Run() })
		}
		close(release)
		joined.Wait()
		close(results)
		for result := range results {
			if !errors.Is(result, io.ErrUnexpectedEOF) {
				t.Fatalf("concurrent caller lost first failure=%v", result)
			}
		}
		if !cleaned || runs != 1 {
			t.Fatalf("native cleanup=%t executions=%d", cleaned, runs)
		}
	})
}

func TestOnceActionGoexitReleasesWaitersWithoutClaimingSuccess(t *testing.T) {
	t.Parallel()
	cleaned := false
	runs := 0
	action, err := background.NewOnceAction(background.OnceActionRequest{Run: func() error { runs++; defer func() { cleaned = true }(); runtime.Goexit(); return nil }})
	if err != nil {
		t.Fatal(err)
	}
	joined := make(chan struct{})
	go func() { defer close(joined); _ = action.Run() }()
	<-joined
	copyOfAction := action
	if err := copyOfAction.Run(); !errors.Is(err, core.ErrBackgroundStopped) || !cleaned || runs != 1 {
		t.Fatalf("Goexit native observation=%v cleanup=%t executions=%d", err, cleaned, runs)
	}
}

func TestOnceActionRefusesUnsetCustodyWithoutExecuting(t *testing.T) {
	t.Parallel()
	if value, err := background.NewOnceAction(background.OnceActionRequest{}); !errors.Is(err, core.ErrBackgroundContract) || value != (background.OnceAction{}) {
		t.Fatalf("nil action=%+v/%v", value, err)
	}
	var empty background.OnceAction
	if err := empty.Run(); !errors.Is(err, core.ErrBackgroundContract) {
		t.Fatalf("unset action run=%v", err)
	}
}

func FuzzOnceActionCopiesKeepFirstNativeResult(f *testing.F) {
	f.Add(uint8(0), false)
	f.Add(uint8(255), true)
	f.Fuzz(func(t *testing.T, repeats uint8, refuse bool) {
		runs := 0
		var cause error
		if refuse {
			cause = io.ErrClosedPipe
		}
		action, err := background.NewOnceAction(background.OnceActionRequest{Run: func() error { runs++; return cause }})
		if err != nil {
			t.Fatal(err)
		}
		first := action.Run()
		for range int(repeats) {
			copyOfAction := action
			if err := copyOfAction.Run(); err != first {
				t.Fatalf("copied native result=%v want=%v", err, first)
			}
		}
		if runs != 1 || !errors.Is(first, cause) {
			t.Fatalf("first-result conservation=%v cause=%v executions=%d", first, cause, runs)
		}
	})
}
