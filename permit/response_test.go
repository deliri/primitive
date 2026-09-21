package permit

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
)

func TestPermitResponseBindingLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		wantErr error
		mutate  func(*Terms) error
		name    string
	}{
		{name: "same response facts survive both public decoders", mutate: func(*Terms) error { return nil }, wantErr: nil},
		{name: "empty skill selection remains an empty permission", mutate: func(p *Terms) error { p.Actions = Actions{}; return nil }, wantErr: nil},
		{name: "foreign response nonce cannot replace the expected answer", mutate: func(p *Terms) error {
			var err error
			p.RequestNonce, err = controlwire.NewRequestNonce([32]byte{77})
			return err
		}, wantErr: core.ErrPermitBinding},
		{name: "foreign generation cannot borrow the response identity", mutate: func(p *Terms) error { var err error; p.Generation, err = lease.NewGeneration(2); return err }, wantErr: core.ErrPermitBinding},
		{name: "shifted activation cannot borrow the provider timestamp", mutate: func(p *Terms) error {
			var startErr, endErr error
			p.NotBefore, startErr = p.NotBefore.Add(p.RetryAfter)
			p.ExpiresAt, endErr = p.ExpiresAt.Add(p.RetryAfter)
			return errors.Join(startErr, endErr)
		}, wantErr: core.ErrPermitBinding},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			registration, checkIn, _ := signedResponseFixtures(t)
			beforeRegistration, beforeCheckIn := registration, checkIn
			if err := errors.Join(tc.mutate(&registration.Permission.Terms), tc.mutate(&checkIn.Permission.Terms)); err != nil {
				t.Fatalf("construct mutation = %v, want nil", err)
			}
			if tc.wantErr != nil && registration.Permission.Terms == beforeRegistration.Permission.Terms {
				t.Fatal("binding mutation changed = false, want true")
			}
			if err := registration.Validate(); !errors.Is(err, tc.wantErr) {
				t.Fatalf("registration Validate = %v, want %v", err, tc.wantErr)
			}
			if err := checkIn.Validate(); !errors.Is(err, tc.wantErr) {
				t.Fatalf("check-in Validate = %v, want %v", err, tc.wantErr)
			}
			// An explicit malformed-wire fixture bypasses only the outer validator.
			// The nested typed production marshalers still own their representations.
			type registrationWire RegistrationResponse
			type checkInWire CheckInResponse
			registrationBytes, err := core.MarshalCanonicalJSONDocument(registrationWire(registration))
			if err != nil {
				t.Fatalf("marshal registration wire = %v, want nil", err)
			}
			checkInBytes, err := core.MarshalCanonicalJSONDocument(checkInWire(checkIn))
			if err != nil {
				t.Fatalf("marshal check-in wire = %v, want nil", err)
			}
			gotRegistration, gotCheckIn := beforeRegistration, beforeCheckIn
			if err := gotRegistration.UnmarshalJSON(registrationBytes); !errors.Is(err, tc.wantErr) {
				t.Fatalf("registration decode = %v, want %v", err, tc.wantErr)
			}
			if err := gotCheckIn.UnmarshalJSON(checkInBytes); !errors.Is(err, tc.wantErr) {
				t.Fatalf("check-in decode = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if gotRegistration != beforeRegistration || gotCheckIn != beforeCheckIn {
					t.Fatal("refused receiver changed = true, want false")
				}
			} else if gotRegistration.Permission != registration.Permission || gotCheckIn != checkIn {
				t.Fatalf("decoded permissions = %+v/%+v, want %+v/%+v", gotRegistration.Permission, gotCheckIn.Permission, registration.Permission, checkIn.Permission)
			}
		})
	}
}

func TestPermitAuthenticResponseMustAnswerExpectedRequest(t *testing.T) {
	t.Parallel()
	r, _ := permitFixture(t)
	baseline, err := Verify(r)
	if err != nil || baseline.Allows(permitAction(t, "operation-a"), r.EffectiveAt) != nil {
		t.Fatalf("baseline Verify = %v, want executable permission", err)
	}
	foreign, err := controlwire.NewRequestNonce([32]byte{99})
	if err != nil {
		t.Fatalf("NewRequestNonce = %v, want nil", err)
	}
	if foreign == r.RequestNonce {
		t.Fatal("nonce mutation changed = false, want true")
	}
	r.RequestNonce = foreign
	got, err := Verify(r)
	if !errors.Is(err, core.ErrPermitBinding) || got != (Verified{}) {
		t.Fatalf("foreign request Verify = %v/%v, want zero/core.ErrPermitBinding", got, err)
	}
	r.RequestNonce = controlwire.RequestNonce{}
	got, err = Verify(r)
	if !errors.Is(err, core.ErrPermitContract) || got != (Verified{}) {
		t.Fatalf("missing request Verify = %v/%v, want zero/core.ErrPermitContract", got, err)
	}
}

func TestPermitMissingResponseNonceCannotBeSignedOrWritten(t *testing.T) {
	t.Parallel()
	r, signer := permitFixture(t)
	r.Document.Terms.RequestNonce = controlwire.RequestNonce{}
	if err := r.Document.Terms.Validate(); !errors.Is(err, core.ErrPermitContract) || !errors.Is(err, core.ErrControlWireNonce) {
		t.Fatalf("Terms.Validate missing nonce = %v, want core.ErrPermitContract and ErrControlWireNonce", err)
	}
	got, err := Sign(r.Document.Terms, signer)
	if !errors.Is(err, core.ErrControlWireNonce) || got != (Document{}) {
		t.Fatalf("Sign missing nonce = %v/%v, want zero/ErrControlWireNonce", got, err)
	}
	var sink permissionWriterObservation
	err = r.Document.Write(&sink)
	if !errors.Is(err, core.ErrControlWireNonce) || sink.calls != 0 || len(sink.data) != 0 {
		t.Fatalf("Write missing nonce = %v/%d/%d, want ErrControlWireNonce/no calls/no bytes", err, sink.calls, len(sink.data))
	}
}
