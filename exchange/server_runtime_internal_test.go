package exchange

import (
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"testing"
)

// Input order is meaningful: the latest acquisition supersedes an unconsumed
// earlier acquisition. This table is the local one-slot publication layer.
func TestServerRuntimeReadinessLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		input       []error
		wantPresent bool
		wantErr     error
	}{
		{name: "neutral absence cannot create readiness"},
		{name: "completed acquisition publishes nil exactly once", input: []error{nil}, wantPresent: true},
		{name: "acquisition refusal remains typed", input: []error{core.ErrExchangeTransport}, wantPresent: true, wantErr: core.ErrExchangeTransport},
		{name: "successful retry replaces stale refusal", input: []error{core.ErrExchangeTransport, nil}, wantPresent: true},
		{name: "new refusal cannot hide behind stale success", input: []error{nil, core.ErrExchangeContract}, wantPresent: true, wantErr: core.ErrExchangeContract},
		{name: "distinct refusal order retains latest identity", input: []error{core.ErrExchangeContract, core.ErrExchangeTransport}, wantPresent: true, wantErr: core.ErrExchangeTransport},
		{name: "duplicate acquisition does not multiply evidence", input: []error{nil, nil}, wantPresent: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			runtime := &ServerRuntime{ready: make(chan error, 1)}
			for _, input := range tc.input {
				runtime.publishReady(input)
			}
			var gotErr error
			var gotPresent bool
			select {
			case gotErr = <-runtime.Ready():
				gotPresent = true
			default:
			}
			if gotPresent != tc.wantPresent || !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("readiness = (%t,%v), want (%t,%v)", gotPresent, gotErr, tc.wantPresent, tc.wantErr)
			}
			select {
			case extra := <-runtime.Ready():
				t.Fatalf("extra acquisition = %v, want none", extra)
			default:
			}
		})
	}
}
