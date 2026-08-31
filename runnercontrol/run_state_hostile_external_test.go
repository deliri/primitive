package runnercontrol_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	primitiveid "github.com/deliri/primitive/v2026/id"
	"github.com/deliri/primitive/v2026/projectstandards"
	"github.com/deliri/primitive/v2026/runnercontrol"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestRunControlStateExhaustsClosedDomainAndObservationBoundary(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		state     runnercontrol.RunControlState
		wantLabel string
		wantValid bool
	}{
		{name: "unknown state is outside the protocol", state: runnercontrol.RunControlStateUnknown},
		{name: "queued state is admitted without final evidence", state: runnercontrol.RunControlQueued, wantLabel: "queued", wantValid: true},
		{name: "starting state is admitted without final evidence", state: runnercontrol.RunControlStarting, wantLabel: "starting", wantValid: true},
		{name: "ready state is admitted without final evidence", state: runnercontrol.RunControlReady, wantLabel: "ready", wantValid: true},
		{name: "executing state is admitted without final evidence", state: runnercontrol.RunControlExecuting, wantLabel: "executing", wantValid: true},
		{name: "cancellation requested state is admitted without final evidence", state: runnercontrol.RunControlCancellationRequested, wantLabel: "cancellation-requested", wantValid: true},
		{name: "cleaning state is admitted without final evidence", state: runnercontrol.RunControlCleaning, wantLabel: "cleaning", wantValid: true},
		{name: "delivering state is admitted without final evidence", state: runnercontrol.RunControlDelivering, wantLabel: "delivering", wantValid: true},
		{name: "delivered state refuses a missing immutable observation", state: runnercontrol.RunControlDelivered, wantLabel: "delivered", wantValid: true},
		{name: "infrastructure failure is a terminal admitted state", state: runnercontrol.RunControlInfrastructureFailed, wantLabel: "infrastructure-failed", wantValid: true},
		{name: "first future state is outside the closed protocol", state: runnercontrol.RunControlState(10)},
		{name: "maximum numeric state is outside the closed protocol", state: runnercontrol.RunControlState(255)},
	}

	run := runStateFixtureID(t)
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			stateErr := tc.state.Validate()
			if tc.wantValid {
				if stateErr != nil || tc.state.String() != tc.wantLabel {
					t.Fatalf("RunControlState(%d) = (label %q, error %v), want (%q, nil)", tc.state, tc.state.String(), stateErr, tc.wantLabel)
				}
			} else if !errors.Is(stateErr, core.ErrPrimitiveContract) || tc.state.String() != "" {
				t.Fatalf("RunControlState(%d) = (label %q, error %v), want empty label and errors.Is(..., %v)", tc.state, tc.state.String(), stateErr, core.ErrPrimitiveContract)
			}

			response := runnercontrol.RunStateResponse{SchemaVersion: runnercontrol.SchemaVersion, Run: run, State: tc.state, UpdatedAt: temporal.InstantFromNanoseconds(2)}
			responseErr := response.Validate()
			wantResponseValid := tc.wantValid && tc.state != runnercontrol.RunControlDelivered
			if wantResponseValid && responseErr != nil {
				t.Fatalf("RunStateResponse.Validate(%s without observation) error = %v, want nil", tc.name, responseErr)
			}
			if !wantResponseValid && !errors.Is(responseErr, core.ErrPrimitiveContract) {
				t.Fatalf("RunStateResponse.Validate(%s without observation) error = %v, want errors.Is(..., %v)", tc.name, responseErr, core.ErrPrimitiveContract)
			}
		})
	}
}

func FuzzRunStateRequestJSONSemanticClosure(f *testing.F) {
	seed := runnercontrol.RunStateRequest{SchemaVersion: runnercontrol.SchemaVersion, Run: runStateFixtureID(f), RequestedAt: temporal.InstantFromNanoseconds(1)}
	canonical, err := seed.MarshalJSON()
	if err != nil {
		f.Fatalf("RunStateRequest.MarshalJSON(seed) error = %v, want nil", err)
	}
	f.Add(canonical)
	f.Add([]byte{})
	f.Add([]byte(`{}`))
	f.Add([]byte(`{"schema_version":1,"run_id":"01890f2e-7b00-7000-8000-000000000001","requested_at":1,"unknown":true}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		got := seed
		gotErr := got.UnmarshalJSON(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrJSONContract) || got != seed {
				t.Fatalf("RunStateRequest.UnmarshalJSON(rejected) = (%+v, %v), want preserved %+v and errors.Is(..., %v)", got, gotErr, seed, core.ErrJSONContract)
			}
			return
		}
		proveRunStateRequestCanonical(t, got)
	})
}

func FuzzCancellationRequestJSONSemanticClosure(f *testing.F) {
	seed := runnercontrol.CancellationRequest{
		SchemaVersion: runnercontrol.SchemaVersion,
		Identity:      runnercontrol.CancellationIdentity{Digest: core.SHA256Of([]byte("cancel exact run"))},
		Run:           runStateFixtureID(f),
		RequestedAt:   temporal.InstantFromNanoseconds(1),
	}
	canonical, err := seed.MarshalJSON()
	if err != nil {
		f.Fatalf("CancellationRequest.MarshalJSON(seed) error = %v, want nil", err)
	}
	f.Add(canonical)
	f.Add([]byte{})
	f.Add([]byte(`{}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		got := seed
		gotErr := got.UnmarshalJSON(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrJSONContract) || got != seed {
				t.Fatalf("CancellationRequest.UnmarshalJSON(rejected) = (%+v, %v), want preserved %+v and errors.Is(..., %v)", got, gotErr, seed, core.ErrJSONContract)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("CancellationRequest.UnmarshalJSON(accepted).Validate() error = %v, want nil", err)
		}
		encoded, err := got.MarshalJSON()
		if err != nil {
			t.Fatalf("CancellationRequest.MarshalJSON(accepted) error = %v, want nil", err)
		}
		var roundTrip runnercontrol.CancellationRequest
		if err := roundTrip.UnmarshalJSON(encoded); err != nil || roundTrip != got {
			t.Fatalf("CancellationRequest canonical round trip = (%+v, %v), want (%+v, nil)", roundTrip, err, got)
		}
		second, err := roundTrip.MarshalJSON()
		if err != nil || !bytes.Equal(second, encoded) {
			t.Fatalf("CancellationRequest second canonical projection = (%q, %v), want (%q, nil)", second, err, encoded)
		}
	})
}

func proveRunStateRequestCanonical(t testing.TB, got runnercontrol.RunStateRequest) {
	t.Helper()
	if err := got.Validate(); err != nil {
		t.Fatalf("RunStateRequest.UnmarshalJSON(accepted).Validate() error = %v, want nil", err)
	}
	encoded, err := got.MarshalJSON()
	if err != nil {
		t.Fatalf("RunStateRequest.MarshalJSON(accepted) error = %v, want nil", err)
	}
	var roundTrip runnercontrol.RunStateRequest
	if err := roundTrip.UnmarshalJSON(encoded); err != nil || roundTrip != got {
		t.Fatalf("RunStateRequest canonical round trip = (%+v, %v), want (%+v, nil)", roundTrip, err, got)
	}
	second, err := roundTrip.MarshalJSON()
	if err != nil || !bytes.Equal(second, encoded) {
		t.Fatalf("RunStateRequest second canonical projection = (%q, %v), want (%q, nil)", second, err, encoded)
	}
}

func runStateFixtureID(t testing.TB) projectstandards.RunID {
	t.Helper()
	uuid, uuidErr := primitiveid.ParseUUIDv7("01890f2e-7b00-7000-8000-000000000001")
	run, runErr := projectstandards.NewRunID(uuid)
	if err := errors.Join(uuidErr, runErr); err != nil {
		t.Fatalf("run state RunID fixture error = %v, want nil", err)
	}
	return run
}
