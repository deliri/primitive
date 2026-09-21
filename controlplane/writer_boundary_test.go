package controlplane_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/core"
)

type canonicalDocument interface {
	core.ValidatedJSONMarshaler
	WriteCanonical(io.Writer) error
}

type canonicalWriteObservation struct {
	err     error
	offered []byte
	count   int
	calls   int
}

func (w *canonicalWriteObservation) Write(data []byte) (int, error) {
	w.calls++
	w.offered = bytes.Clone(data)
	return w.count, w.err
}

func TestCanonicalWriterLayerTriadPreservesExactEffects(t *testing.T) {
	t.Parallel()
	registration := issueTestRegistration(t)
	checkIn := issueTestCheckIn(t, controlplaneOffering(t, 3), testCheckInWindow())
	response := issueTestCheckInResponse(t)
	bodyBytes, err := registration.document.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON(fixture) error = %v, want nil", err)
	}
	length, err := core.NewByteLength(uint64(len(bodyBytes)))
	if err != nil {
		t.Fatalf("NewByteLength() error = %v, want nil", err)
	}
	commitment := controlplane.ResponseCommitment{Header: registration.document.Payload.Header, BodyLength: length, BodySHA256: core.SHA256Of(bodyBytes)}
	documents := []struct {
		valid    canonicalDocument
		zero     canonicalDocument
		name     string
		identity core.ErrorIdentity
	}{
		{name: "certificate body", valid: checkIn.certificate.Body, zero: controlplane.InstallationCertificateBody{}, identity: core.ErrControlPlaneRegistration},
		{name: "registration payload", valid: registration.document.Payload, zero: controlplane.RegistrationPayload{}, identity: core.ErrControlPlaneRegistration},
		{name: "check-in payload", valid: checkIn.request.Payload, zero: controlplane.CheckInPayload{}, identity: core.ErrControlPlaneCheckIn},
		{name: "check-in response", valid: response.document.Payload, zero: controlplane.CheckInResponsePayload{}, identity: core.ErrControlPlaneCheckInResponse},
		{name: "response commitment", valid: commitment, zero: controlplane.ResponseCommitment{}, identity: core.ErrControlPlaneResponseDocument},
	}
	for _, document := range documents {
		t.Run(document.name, func(t *testing.T) {
			t.Parallel()
			encoded, err := document.valid.MarshalJSON()
			if err != nil || len(encoded) == 0 {
				t.Fatalf("canonical fixture = (%d bytes, %v), want nonempty and nil", len(encoded), err)
			}
			for _, tc := range []struct {
				cause     error
				wantErr   error
				name      string
				count     int
				wantCalls int
				nilWriter bool
				zeroBody  bool
			}{
				{name: "exact full write preserves every signed byte", count: len(encoded), wantCalls: 1},
				{name: "one-byte truncation cannot report completion", count: len(encoded) - 1, wantErr: io.ErrShortWrite, wantCalls: 1},
				{name: "zero progress cannot report completion", count: 0, wantErr: io.ErrShortWrite, wantCalls: 1},
				{name: "negative writer count cannot report completion", count: -1, wantErr: io.ErrShortWrite, wantCalls: 1},
				{name: "oversized writer count cannot report completion", count: len(encoded) + 1, wantErr: io.ErrShortWrite, wantCalls: 1},
				{name: "partial canceled write preserves cancellation", count: 1, cause: context.Canceled, wantErr: context.Canceled, wantCalls: 1},
				{name: "full count cannot erase write failure", count: len(encoded), cause: io.ErrClosedPipe, wantErr: io.ErrClosedPipe, wantCalls: 1},
				{name: "nil destination refuses without panic", nilWriter: true, wantErr: core.ErrControlPlaneContract},
				{name: "zero document emits no bytes", zeroBody: true, wantErr: core.ErrControlPlaneContract},
			} {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					defer func() {
						if got := recover(); got != nil {
							t.Fatalf("WriteCanonical() panic = %v, want typed refusal", got)
						}
					}()
					observed := &canonicalWriteObservation{count: tc.count, err: tc.cause}
					var writer io.Writer = observed
					if tc.nilWriter {
						writer = nil
					}
					body := document.valid
					if tc.zeroBody {
						body = document.zero
					}
					gotErr := body.WriteCanonical(writer)
					if !errors.Is(gotErr, tc.wantErr) {
						t.Errorf("WriteCanonical() error = %v, want %v", gotErr, tc.wantErr)
					}
					if tc.wantErr != nil && !errors.Is(gotErr, core.ErrControlPlaneContract) {
						t.Errorf("WriteCanonical() identity = %v, want %v", gotErr, core.ErrControlPlaneContract)
					}
					if tc.wantErr != nil && !tc.zeroBody {
						for _, identity := range []core.ErrorIdentity{core.ErrControlPlaneRegistration, core.ErrControlPlaneCheckIn, core.ErrControlPlaneCheckInResponse, core.ErrControlPlaneResponseDocument} {
							want := identity == document.identity
							if got := errors.Is(gotErr, identity); got != want {
								t.Errorf("WriteCanonical() identity %v = %t, want %t; error = %v", identity, got, want, gotErr)
							}
						}
					}
					if observed.calls != tc.wantCalls {
						t.Fatalf("writer calls = %d, want %d", observed.calls, tc.wantCalls)
					}
					if tc.wantCalls == 0 && len(observed.offered) != 0 {
						t.Fatalf("refused write offered %d bytes, want zero", len(observed.offered))
					}
					if tc.wantCalls == 1 && !bytes.Equal(observed.offered, encoded) {
						t.Fatalf("offered bytes = %d, want exact %d canonical bytes", len(observed.offered), len(encoded))
					}
				})
			}
		})
	}
}
