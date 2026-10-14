package admission

import (
	"context"
	"errors"
	"math"
	"runtime"
	"sync/atomic"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
)

func admissionFixture(t testing.TB, maximum uint64) *Gate {
	t.Helper()
	limit, err := NewLimit(maximum)
	if err != nil {
		t.Fatalf("NewLimit(%d) = %v, want nil", maximum, err)
	}
	gate, err := NewGate(limit)
	if err != nil {
		t.Fatalf("NewGate = %v, want nil", err)
	}
	return gate
}

func TestAdmissionNativeRepresentationLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		maximum uint64
		wantErr error
	}{
		{name: "absent policy refused", wantErr: core.ErrAdmissionContract},
		{name: "minimum capacity", maximum: 1},
		{name: "ordinary capacity", maximum: 8},
		{name: "signed native maximum retained", maximum: math.MaxInt64},
		{name: "unsigned native maximum retained", maximum: math.MaxUint64},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			limit, err := NewLimit(tc.maximum)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("NewLimit = %v, want %v", err, tc.wantErr)
			}
			if err != nil {
				if limit != (Limit{}) {
					t.Fatalf("refused limit = %+v, want zero", limit)
				}
				return
			}
			gate, err := NewGate(limit)
			if err != nil || limit.Maximum() != tc.maximum {
				t.Fatalf("gate/maximum/error = %p/%d/%v, want present/%d/nil", gate, limit.Maximum(), err, tc.maximum)
			}
			calls := 0
			err = gate.WithSlot(t.Context(), func(_ context.Context, observed Observation) error {
				calls++
				if observed.Validate() != nil || observed.InUse() != 1 || observed.Limit() != limit {
					t.Fatalf("native reservation = %+v, want exact1/limit", observed)
				}
				return nil
			})
			if err != nil || calls != 1 || gate.used.Load() != 0 {
				t.Fatalf("slot/error/calls/retained = %v/%d/%d, want nil/1/0", err, calls, gate.used.Load())
			}
		})
	}
}

func TestAdmissionNativeTerminalAndCallbackCausesLayerTriad(t *testing.T) {
	t.Parallel()
	callbackCause := errors.New("owned callback failure")
	for _, tc := range []struct {
		name         string
		terminal     uint8
		nilOperation bool
		wantErr      error
		wantCalls    int
	}{
		{name: "callback cause returned exactly", wantErr: callbackCause, wantCalls: 1},
		{name: "cancelled scope performs no operation", terminal: 1, wantErr: context.Canceled},
		{name: "expired scope performs no operation", terminal: 2, wantErr: context.DeadlineExceeded},
		{name: "nil scope performs no operation", terminal: 3, wantErr: core.ErrNilContext},
		{name: "nil operation is a contract refusal", nilOperation: true, wantErr: core.ErrAdmissionContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gate := admissionFixture(t, 1)
			ctx := t.Context()
			switch tc.terminal {
			case 1:
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case 2:
				var cancel context.CancelFunc
				ctx, cancel = context.WithDeadline(ctx, time.Unix(0, 0))
				defer cancel()
			case 3:
				ctx = nil
			}
			calls := 0
			operation := func(context.Context, Observation) error { calls++; return callbackCause }
			if tc.nilOperation {
				operation = nil
			}
			err := gate.WithSlot(ctx, operation)
			if !errors.Is(err, tc.wantErr) || calls != tc.wantCalls || gate.used.Load() != 0 {
				t.Fatalf("terminal/error/calls/retained = %v/%d/%d, want %v/%d/0", err, calls, gate.used.Load(), tc.wantErr, tc.wantCalls)
			}
			if tc.wantCalls == 1 && err != callbackCause {
				t.Fatalf("callback error = %v, want identical cause", err)
			}
			if err := gate.WithSlot(t.Context(), func(context.Context, Observation) error { return nil }); err != nil {
				t.Fatalf("next operation after terminal = %v, want nil", err)
			}
		})
	}
	ctx, cancel := context.WithCancelCause(t.Context())
	reason := errors.New("owned shutdown reason")
	cancel(reason)
	gate := admissionFixture(t, 1)
	err := gate.WithSlot(ctx, func(context.Context, Observation) error { t.Fatal("cancelled scope executed callback"); return nil })
	if !errors.Is(err, context.Canceled) || !errors.Is(err, reason) || errors.Is(err, core.ErrAdmissionContract) {
		t.Fatalf("native context cause = %v, want cancellation+reason without admission identity", err)
	}
}

// Actual native goroutines reserve8 scopes. Channels establish that each
// callback holds its slot; a ninth call refuses before executing any callback.
func TestAdmissionNativeConcurrentCapacityAndCancellationLayerTriad(t *testing.T) {
	t.Parallel()
	gate := admissionFixture(t, 8)
	ctx, cancel := context.WithCancel(t.Context())
	entered := make(chan Observation, 8)
	done := make(chan error, 8)
	for range 8 {
		go func() {
			done <- gate.WithSlot(ctx, func(ctx context.Context, o Observation) error { entered <- o; <-ctx.Done(); return ctx.Err() })
		}()
	}
	joined := 0
	defer func() {
		cancel()
		for joined < 8 {
			select {
			case <-done:
				joined++
			case <-time.After(10 * time.Second):
				t.Error("owned native scope did not join")
				return
			}
		}
	}()
	for range 8 {
		select {
		case o := <-entered:
			if o.Validate() != nil || o.InUse() > 8 {
				t.Fatalf("reservation = %+v, want admitted<=8", o)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("native callbacks failed to enter")
		}
	}
	if gate.used.Load() != 8 {
		t.Fatalf("held native count = %d, want8", gate.used.Load())
	}
	var refusedCalls atomic.Int64
	refused := make(chan error, 1)
	refusalJoined := false
	defer func() {
		if refusalJoined {
			return
		}
		cancel()
		select {
		case <-refused:
		case <-time.After(10 * time.Second):
			t.Error("refused native scope did not join")
		}
	}()
	go func() {
		refused <- gate.WithSlot(ctx, func(context.Context, Observation) error { refusedCalls.Add(1); return nil })
	}()
	select {
	case err := <-refused:
		refusalJoined = true
		if !errors.Is(err, core.ErrAdmissionCapacity) || errors.Is(err, core.ErrAdmissionContract) || refusedCalls.Load() != 0 {
			t.Fatalf("full admission/error/calls = %v/%d, want capacity-only/0", err, refusedCalls.Load())
		}
	case <-time.After(10 * time.Second):
		t.Fatal("full admission queued instead of refusing")
	}
	cancel()
	for joined < 8 {
		select {
		case err := <-done:
			joined++
			if !errors.Is(err, context.Canceled) {
				t.Errorf("cancelled callback error = %v, want native cancellation", err)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("cancelled scope did not join")
		}
	}
	if gate.used.Load() != 0 {
		t.Fatalf("joined native count = %d, want0", gate.used.Load())
	}
	if err := gate.WithSlot(t.Context(), func(_ context.Context, o Observation) error {
		if o.InUse() != 1 {
			t.Errorf("reused count = %d, want1", o.InUse())
		}
		return nil
	}); err != nil {
		t.Fatalf("reuse after cancellation = %v, want nil", err)
	}
}

func TestAdmissionNativePanicAndGoexitRelease(t *testing.T) {
	t.Parallel()
	gate := admissionFixture(t, 1)
	cause := errors.New("owned native panic")
	func() {
		defer func() {
			if got := recover(); got != cause {
				t.Errorf("panic = %v, want identical cause", got)
			}
		}()
		_ = gate.WithSlot(t.Context(), func(context.Context, Observation) error { panic(cause) })
	}()
	if gate.used.Load() != 0 {
		t.Fatalf("panic retained count = %d, want0", gate.used.Load())
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		_ = gate.WithSlot(ctx, func(context.Context, Observation) error { runtime.Goexit(); return nil })
	}()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("Goexit scope did not join")
	}
	if err := gate.WithSlot(t.Context(), func(context.Context, Observation) error { return nil }); err != nil || gate.used.Load() != 0 {
		t.Fatalf("reuse after Goexit/error/count = %v/%d, want nil/0", err, gate.used.Load())
	}
}

func TestAdmissionZeroCapabilitiesNeverExecute(t *testing.T) {
	t.Parallel()
	for _, gate := range []*Gate{nil, {}} {
		calls := 0
		err := gate.WithSlot(t.Context(), func(context.Context, Observation) error { calls++; return nil })
		if !errors.Is(err, core.ErrAdmissionContract) || calls != 0 {
			t.Fatalf("zero gate/error/calls = %v/%d, want contract/0", err, calls)
		}
	}
	if err := (Observation{}).Validate(); !errors.Is(err, core.ErrAdmissionContract) {
		t.Fatalf("zero observation = %v, want contract", err)
	}
}

func FuzzAdmissionLimitNativeClosure(f *testing.F) {
	for _, maximum := range []uint64{0, 1, 8, math.MaxInt64, math.MaxUint64} {
		f.Add(maximum)
	}
	f.Fuzz(func(t *testing.T, maximum uint64) {
		limit, err := NewLimit(maximum)
		if err != nil {
			if maximum != 0 || !errors.Is(err, core.ErrAdmissionContract) || limit != (Limit{}) {
				t.Fatalf("limit refusal = %+v/%v, want zero-only typed refusal", limit, err)
			}
			return
		}
		gate, err := NewGate(limit)
		if err != nil || limit.Maximum() != maximum {
			t.Fatalf("admitted native limit/gate = %+v/%v, want exact maximum/nil", limit, err)
		}
		err = gate.WithSlot(t.Context(), func(_ context.Context, o Observation) error {
			if o.Validate() != nil || o.Limit() != limit || o.InUse() != 1 {
				t.Fatalf("native reservation = %+v, want exact agreement", o)
			}
			return nil
		})
		if err != nil || gate.used.Load() != 0 {
			t.Fatalf("native closure = %v/%d, want nil/0", err, gate.used.Load())
		}
	})
}
