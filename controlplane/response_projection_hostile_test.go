package controlplane_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/core"
)

func TestExistingAuthenticatedResponseProjectionKeepsOneEnvelope(t *testing.T) {
	t.Parallel()

	cases := []struct {
		run  func(testing.TB)
		name string
	}{
		{name: "registration response closes its existing envelope", run: func(t testing.TB) {
			proveBidirectionalResponseProjection(t, issueTestRegistration(t).document)
		}},
		{name: "check in response closes its existing envelope", run: func(t testing.TB) {
			proveBidirectionalResponseProjection(t, issueTestCheckInResponse(t).document)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tc.run(t)
		})
	}

	var zeroRegistration controlplane.RegistrationDocument
	if got, err := core.EncodeValidatedJSON(zeroRegistration, core.DefaultStrictJSONLimits()); got != nil ||
		!errors.Is(err, core.ErrControlPlaneRegistration) {
		t.Fatalf("EncodeValidatedJSON(zero registration response) = (%d bytes, %v), want nil and %v", len(got), err, core.ErrControlPlaneRegistration)
	}
	var zeroCheckIn controlplane.CheckInResponseDocument
	if got, err := core.EncodeValidatedJSON(zeroCheckIn, core.DefaultStrictJSONLimits()); got != nil ||
		!errors.Is(err, core.ErrControlPlaneCheckInResponse) {
		t.Fatalf("EncodeValidatedJSON(zero check-in response) = (%d bytes, %v), want nil and %v", len(got), err, core.ErrControlPlaneCheckInResponse)
	}
}

func proveBidirectionalResponseProjection[T core.ValidatedJSONProjection](t testing.TB, response T) {
	t.Helper()

	want, err := response.MarshalJSON()
	if err != nil {
		t.Fatalf("response MarshalJSON() error = %v, want nil", err)
	}
	got, err := core.EncodeValidatedJSON(response, core.DefaultStrictJSONLimits())
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("EncodeValidatedJSON(response) = (%d bytes, %v), want exact %d bytes", len(got), err, len(want))
	}
	maximum, err := core.NewByteCount(uint64(len(want) - 1))
	if err != nil {
		t.Fatalf("NewByteCount(one below response) error = %v, want nil", err)
	}
	limits := core.DefaultStrictJSONLimits()
	limits.DocumentMaximumBytes = maximum
	if got, err := core.EncodeValidatedJSON(response, limits); got != nil || !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("EncodeValidatedJSON(one-byte-short limit) = (%d bytes, %v), want nil and %v", len(got), err, core.ErrJSONContract)
	}
}
