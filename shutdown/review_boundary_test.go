package shutdown

import (
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

type armedWatchParent struct {
	context.Context
	armed      atomic.Bool
	calls      atomic.Uint32
	failure    error
	valuePanic bool
}

func (p *armedWatchParent) Done() <-chan struct{} {
	p.calls.Add(1)
	if p.armed.Load() && !p.valuePanic {
		panic(p.failure)
	}
	return p.Context.Done()
}

// witness:waiver doctrine/code_form/return_interface -- implements the exact context.Context.Value method to test Go propagation and unlinking.
func (p *armedWatchParent) Value(key any) any {
	if p.armed.Load() && p.valuePanic {
		panic(p.failure)
	}
	return p.Context.Value(key)
}

func TestControllerEscalationNeverReentersParentDone(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		unknowns int
	}{
		{name: "second signal uses the captured parent channel"},
		{name: "ignored input cannot reenter the parent method", unknowns: 3},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				parent, cancel := context.WithCancel(t.Context())
				defer cancel()
				probe := &armedWatchParent{Context: parent, failure: errHostileCleanup}
				events := make(chan os.Signal, signalTransitionCapacity)
				releases := make(chan struct{}, 1)
				c := watchSourceRequestForTest(t, WatchRequest{Parent: probe, Set: SignalSetStandard, Policy: SignalPolicy{SecondSignal: SecondSignalEscalate, GraceExpiry: GraceExpiryDisabled}}, events, releases)
				events <- firstPlatformSignal()
				waitContext(c.Context(), t)
				synctest.Wait() // first-signal cancellation and Go unlinking have finished.
				before := probe.calls.Load()
				probe.armed.Store(true)
				for range tc.unknowns {
					events <- unknownSignal{}
					synctest.Wait()
				}
				events <- firstPlatformSignal()
				waitController(t, c)
				got, open := <-c.Escalated()
				if !open || got.Validate() != nil || got.Reason() != EscalationSecondSignal || got.FirstSignal() != SignalKindInterrupt || got.TriggerSignal() != SignalKindInterrupt {
					t.Fatalf("escalation = (%+v,%t), want exact second interrupt", got, open)
				}
				if calls := probe.calls.Load(); calls != before {
					t.Fatalf("parent Done calls = %d, want frozen at %d", calls, before)
				}
				if err := c.Close(); err != nil || len(releases) != 1 {
					t.Fatalf("Close = (%v,%d releases), want nil/1", err, len(releases))
				}
			})
		})
	}
}

func TestWatchConstructionPanicKeepsItsActualErrorIdentity(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name            string
		failure         error
		valuePanic      bool
		wantObservation bool
	}{
		{name: "Done panic preserves native failure", failure: errHostileCleanup},
		{name: "Value panic preserves native failure", failure: errHostileCleanup, valuePanic: true},
		{name: "observation identity only follows an observation panic", failure: core.ErrContextObservation, wantObservation: true},
		{name: "wrapped observation preserves both original identities", failure: errors.Join(core.ErrContextObservation, errHostileCleanup), wantObservation: true},
		{name: "native error formatter is never called", failure: &panicOnError{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			parent, cancel := context.WithCancel(t.Context())
			defer cancel()
			probe := &armedWatchParent{Context: parent, failure: tc.failure, valuePanic: tc.valuePanic}
			probe.armed.Store(true)
			events := make(chan os.Signal)
			c, err := watchSource(WatchRequest{Parent: probe, Set: SignalSetStandard, Policy: defaultSignalPolicy()}, signalSource{events: events, release: func() {}})
			if c != nil {
				if closeErr := c.Close(); closeErr != nil {
					t.Errorf("unexpected Close = %v, want nil", closeErr)
				}
			}
			if c != nil || !errors.Is(err, core.ErrShutdownContract) || !errors.Is(err, tc.failure) || errors.Is(err, core.ErrContextObservation) != tc.wantObservation {
				t.Fatalf("construction = (%v,%v,observation=%t), want nil/contract+native/observation=%t", c, err, errors.Is(err, core.ErrContextObservation), tc.wantObservation)
			}
		})
	}
}

// Direct internal execution-boundary ratchet. Public Run creates its own Go
// deadline root; these hostile roots attack the wider helper's error handling.
func TestSkippedStepPreservesObservedTerminalIdentity(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		deadline bool
		want     error
	}{
		{name: "canceled root never claims a deadline", want: context.Canceled},
		{name: "expired root keeps the deadline", deadline: true, want: context.DeadlineExceeded},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			synctest.Test(t, func(t *testing.T) {
				ctx, cancel := context.WithCancel(t.Context())
				cancel()
				if tc.deadline {
					var stop context.CancelFunc
					var err error
					ctx, stop, err = expiredRoot(t)
					if err != nil {
						t.Fatalf("deadline root = %v, want nil", err)
					}
					defer stop()
				}
				calls := 0
				step := validStep(t, 1, PhaseDrain, durationForTest(t, time.Second))
				step.Action = func(context.Context) error { calls++; return nil }
				got := runStep(ctx, step)
				if calls != 0 || got.Outcome() != StepOutcomeTotalBudgetExceeded || !errors.Is(got.Failure(), core.ErrShutdownTotalTimeout) || !errors.Is(got.Failure(), tc.want) || errors.Is(got.Failure(), context.DeadlineExceeded) != tc.deadline {
					t.Fatalf("skipped observation = (%+v,%d calls), want total/observed cause/no calls", got, calls)
				}
			})
		})
	}
}

func TestRejectedPlanRunDoesNotConsumeSingleUse(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name         string
		corruptCount bool
		want         diagnostic
	}{
		{name: "zero policy refuses consistently", want: diagnosticTotalBudgetInvalid},
		{name: "invalid retained count refuses consistently", corruptCount: true, want: diagnosticPlanCount},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			p := new(Plan)
			if tc.corruptCount {
				p.policy = PlanPolicy{TotalBudget: durationForTest(t, time.Hour)}
				p.count = MaximumSteps + 1
			}
			for attempt := range 2 {
				got, err := p.Run(t.Context())
				var detail diagnostic
				if !errors.Is(err, core.ErrShutdownContract) || !errors.As(err, &detail) || detail != tc.want || got.Count() != 0 {
					t.Fatalf("attempt %d = (%d,%v,diagnostic=%v), want zero/contract/%v", attempt, got.Count(), err, detail, tc.want)
				}
			}
		})
	}
}

func TestConcurrentPlanRunClaimsExactlyOneExecution(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		callers int
	}{
		{name: "two competing starts execute once", callers: 2},
		{name: "many competing starts execute once", callers: MaximumSteps},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			budget := durationForTest(t, time.Hour)
			p, err := NewPlan(PlanPolicy{TotalBudget: budget})
			if err != nil {
				t.Fatalf("NewPlan = %v, want nil", err)
			}
			var calls, successes atomic.Uint32
			step := validStep(t, 1, PhaseDrain, budget)
			step.Action = func(context.Context) error { calls.Add(1); return nil }
			if err := p.Register(step); err != nil {
				t.Fatalf("Register = %v, want nil", err)
			}
			var workers sync.WaitGroup
			for range tc.callers {
				workers.Go(func() {
					got, err := p.Run(t.Context())
					if err == nil {
						successes.Add(1)
						if got.Count() != 1 || got.Validate() != nil {
							t.Errorf("successful Run = (%d,%v), want one valid observation", got.Count(), got.Validate())
						}
						return
					}
					if !errors.Is(err, core.ErrShutdownContract) || got.Count() != 0 {
						t.Errorf("refused Run = (%d,%v), want zero/contract", got.Count(), err)
					}
				})
			}
			workers.Wait()
			if successes.Load() != 1 || calls.Load() != 1 {
				t.Fatalf("concurrent starts = (%d successes,%d calls), want 1/1", successes.Load(), calls.Load())
			}
		})
	}
}

func expiredRoot(t *testing.T) (context.Context, context.CancelFunc, error) {
	t.Helper()
	return temporal.WithTimeout(temporal.TimeoutRequest{Parent: t.Context(), Duration: durationForTest(t, 0)})
}

func TestControllerOwnsContextUnlinkFailureAndStillJoins(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		signal     bool
		valuePanic bool
	}{
		{name: "Close contains parent Done panic"},
		{name: "Close contains parent Value panic", valuePanic: true},
		{name: "first signal contains parent Done panic", signal: true},
		{name: "first signal contains parent Value panic", signal: true, valuePanic: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			parent, cancel := context.WithCancel(t.Context())
			defer cancel()
			probe := &armedWatchParent{Context: parent, failure: errHostileCleanup, valuePanic: tc.valuePanic}
			events := make(chan os.Signal, signalTransitionCapacity)
			var releases atomic.Uint32
			c, err := watchSource(WatchRequest{Parent: probe, Policy: defaultSignalPolicy(), Set: SignalSetStandard}, signalSource{events: events, release: func() { releases.Add(1) }})
			if err != nil {
				t.Fatalf("Watch setup = %v, want nil", err)
			}
			t.Cleanup(func() {
				probe.armed.Store(false)
				if err := c.Close(); !errors.Is(err, errHostileCleanup) {
					t.Errorf("cleanup Close = %v, want retained native failure", err)
				}
			})
			probe.armed.Store(true)
			if tc.signal {
				events <- firstPlatformSignal()
				waitController(t, c)
			}
			err = c.Close()
			var panicErr ContextPanicError
			if !errors.Is(err, errHostileCleanup) || !errors.Is(err, core.ErrShutdownContract) || errors.Is(err, core.ErrContextObservation) || !errors.As(err, &panicErr) || panicErr.Validate() != nil {
				t.Fatalf("Close = (%v,%+v), want valid context panic with native identity only", err, panicErr)
			}
			select {
			case <-c.Done():
			default:
				t.Fatal("Close joined = false, want true")
			}
			if releases.Load() != 1 {
				t.Fatalf("release count = %d, want one", releases.Load())
			}
			wantCause := error(context.Canceled)
			if tc.signal {
				wantCause = core.ErrShutdownSignalReceived
			}
			if !errors.Is(context.Cause(c.Context()), wantCause) {
				t.Fatalf("owned cause = %v, want %v", context.Cause(c.Context()), wantCause)
			}
			if again := c.Close(); !errors.Is(again, errHostileCleanup) || releases.Load() != 1 {
				t.Fatalf("repeated Close = (%v,%d releases), want same native identity/one", again, releases.Load())
			}
		})
	}
}

type textPanicParent struct {
	context.Context
	input string
	bytes bool
}

func (p textPanicParent) Done() <-chan struct{} {
	if p.bytes {
		panic([]byte(p.input))
	}
	panic(p.input)
}

func FuzzWatchParentPanicIngress(f *testing.F) {
	for _, seed := range []string{"", "context failure", "\xff", strings.Repeat("雪", panicDiagnosticMaximumRunes-1), strings.Repeat("雪", panicDiagnosticMaximumRunes), strings.Repeat("雪", panicDiagnosticMaximumRunes+1)} {
		f.Add(seed, false, true)
		f.Add(seed, true, true)
	}
	f.Add("", false, false)
	f.Fuzz(func(t *testing.T, input string, asBytes, panics bool) {
		input = input[:min(len(input), 4096)]
		request := WatchRequest{Parent: textPanicParent{Context: t.Context(), input: input, bytes: asBytes}, Policy: defaultSignalPolicy(), Set: SignalSetStandard}
		if !panics {
			request.Parent = t.Context()
		}
		events := make(chan os.Signal)
		close(events)
		c, err := watchSource(request, signalSource{events: events, release: func() {}})
		if !panics {
			if err != nil || c == nil {
				t.Fatalf("unmutated parent = (%v,%v), want owned/nil", c, err)
			}
			waitController(t, c)
			if err := c.Close(); err != nil || !errors.Is(context.Cause(c.Context()), core.ErrShutdownSignalSource) {
				t.Fatalf("unmutated parent Close = (%v,%v), want nil/source closure", err, context.Cause(c.Context()))
			}
			return
		}
		var got ContextPanicError
		if c != nil {
			if e := c.Close(); e != nil {
				t.Errorf("unexpected Close = %v, want nil", e)
			}
		}
		if c != nil || !errors.Is(err, core.ErrShutdownContract) || errors.Is(err, core.ErrContextObservation) || !errors.As(err, &got) {
			t.Fatalf("panic ingress = (%v,%v), want nil/context panic without fabricated observation", c, err)
		}
		runes := []rune(input)
		runes = runes[:min(len(runes), panicDiagnosticMaximumRunes)]
		want := string(runes)
		if len(runes) == 0 {
			want = emptyPanicDiagnostic
		}
		if got.Diagnostic() != want || got.Validate() != nil || !utf8.ValidString(got.Error()) {
			t.Fatalf("panic diagnostic = (%q,%v), want %q/valid", got.Diagnostic(), got.Validate(), want)
		}
	})
}

func TestContextPanicErrorSchemaLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		value   ContextPanicError
		wantErr error
	}{
		{name: "bounded native diagnostic keeps its typed cause", value: ContextPanicError{cause: errors.Join(core.ErrShutdownContract, errHostileCleanup), diagnostic: "retained"}},
		{name: "unset panic is no evidence", wantErr: core.ErrShutdownContract},
		{name: "foreign error cannot claim shutdown panic", value: ContextPanicError{cause: errHostileCleanup, diagnostic: "retained"}, wantErr: core.ErrShutdownContract},
		{name: "empty diagnostic is refused", value: ContextPanicError{cause: core.ErrShutdownContract}, wantErr: core.ErrShutdownContract},
		{name: "invalid UTF8 is refused", value: ContextPanicError{cause: core.ErrShutdownContract, diagnostic: "\xff"}, wantErr: core.ErrShutdownContract},
		{name: "one above the diagnostic ceiling is refused", value: ContextPanicError{cause: core.ErrShutdownContract, diagnostic: strings.Repeat("x", panicDiagnosticMaximumRunes+1)}, wantErr: core.ErrShutdownContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if err := tc.value.Validate(); !errors.Is(err, tc.wantErr) {
				t.Fatalf("panic Validate = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr == nil && !errors.Is(tc.value, errHostileCleanup) {
				t.Fatalf("panic cause = %v, want retained native error", tc.value)
			}
		})
	}
}

type terminalParentProbe struct {
	context.Context
	terminal   error
	panicErr   bool
	panicValue bool
}

func (p terminalParentProbe) Err() error {
	if p.panicErr {
		panic(errHostileCleanup)
	}
	return p.terminal
}

// witness:waiver doctrine/code_form/return_interface -- implements the exact context.Context.Value method to attack cancellation-cause observation.
func (p terminalParentProbe) Value(key any) any {
	if p.panicValue {
		panic(errHostileCleanup)
	}
	return p.Context.Value(key)
}

func TestTerminalContextObservationPreservesRefusalAndCustomCause(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		terminal   error
		panicErr   bool
		panicValue bool
		custom     bool
		want       error
		wantNative bool
	}{
		{name: "active context supplies no terminal fact"},
		{name: "nonstandard Err is an observation refusal", terminal: errHostileCleanup, want: core.ErrContextObservation},
		{name: "panicking Err is an observation refusal", panicErr: true, want: core.ErrContextObservation},
		{name: "cancellation keeps a custom cause", custom: true, want: context.Canceled, wantNative: true},
		{name: "cause lookup panic keeps cancellation and actual panic", terminal: context.Canceled, panicValue: true, want: context.Canceled, wantNative: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var ctx context.Context = terminalParentProbe{Context: t.Context(), terminal: tc.terminal, panicErr: tc.panicErr, panicValue: tc.panicValue}
			if tc.custom {
				var cancel context.CancelCauseFunc
				ctx, cancel = context.WithCancelCause(t.Context())
				cancel(errHostileCleanup)
			}
			got := contextTerminalError(ctx)
			if !errors.Is(got, tc.want) || errors.Is(got, context.DeadlineExceeded) || errors.Is(got, errHostileCleanup) != tc.wantNative {
				t.Fatalf("terminal = %v, want %v/native=%t/no deadline", got, tc.want, tc.wantNative)
			}
			if tc.panicValue && !errors.Is(got, core.ErrShutdownContract) {
				t.Fatalf("cause observation failure = %v, want shutdown contract", got)
			}
		})
	}
}
