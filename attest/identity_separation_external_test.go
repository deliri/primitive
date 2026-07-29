package attest_test

import (
	"crypto/ed25519"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
)

type signingSeparationMutation uint8

const (
	signingSeparationNone signingSeparationMutation = iota
	signingSeparationBody
	signingSeparationDomain
	signingSeparationKey
)

func TestSignPublicDeterminismAndSeparationMatrix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		mutation  signingSeparationMutation
		wantEqual bool
	}{
		{name: "same key domain and body are deterministic", mutation: signingSeparationNone, wantEqual: true},
		{name: "one changed body byte separates", mutation: signingSeparationBody},
		{name: "changed domain separates", mutation: signingSeparationDomain},
		{name: "changed standard private key separates", mutation: signingSeparationKey},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			firstKey := deterministicPrivateKey(t, "determinism-key")
			firstBody := literalBody{domain: testDomainPrimary, value: []byte("deterministic body")}
			secondKey := append(ed25519.PrivateKey(nil), firstKey...)
			secondBody := copyLiteralBody(firstBody)
			switch tc.mutation {
			case signingSeparationNone:
			case signingSeparationBody:
				secondBody.value[0] ^= 1
			case signingSeparationDomain:
				secondBody.domain = testDomainAlternate
			case signingSeparationKey:
				secondKey = deterministicPrivateKey(t, "separated-key")
			default:
				t.Fatalf("signing separation mutation = %d, want admitted mutation", tc.mutation)
			}
			gotFirst := mustEnvelope(t, firstBody, firstKey)
			gotSecond := mustEnvelope(t, secondBody, secondKey)
			gotEqual := gotFirst == gotSecond
			if gotEqual != tc.wantEqual {
				t.Fatalf("signed envelopes equal = %t, want %t", gotEqual, tc.wantEqual)
			}
		})
	}
}

func TestAttestStableErrorIdentityHierarchy(t *testing.T) {
	t.Parallel()

	cases := []struct {
		run              func(testing.TB) error
		wantNative       error
		name             string
		wantContract     bool
		wantVerification bool
		wantJSON         bool
	}{
		{
			name:         "sign contract failure has only contract identity",
			run:          signContractErrorFixture,
			wantContract: true,
		},
		{
			name:             "verification failure has verification and parent contract identities",
			run:              verificationErrorFixture,
			wantContract:     true,
			wantVerification: true,
		},
		{
			name:         "envelope JSON failure has JSON and contract identities",
			run:          jsonContractErrorFixture,
			wantContract: true,
			wantJSON:     true,
		},
		{
			name:         "native callback error remains under contract identity",
			run:          nativeCallbackErrorFixture,
			wantContract: true,
			wantNative:   fixtureErrorWrite,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := tc.run(t)
			if gotErr == nil {
				t.Fatal("operation error = nil, want typed failure")
			}
			if got := errors.Is(gotErr, core.ErrAttestContract); got != tc.wantContract {
				t.Fatalf("errors.Is(error, ErrAttestContract) = %t, want %t", got, tc.wantContract)
			}
			if got := errors.Is(gotErr, core.ErrAttestVerification); got != tc.wantVerification {
				t.Fatalf("errors.Is(error, ErrAttestVerification) = %t, want %t", got, tc.wantVerification)
			}
			if got := errors.Is(gotErr, core.ErrJSONContract); got != tc.wantJSON {
				t.Fatalf("errors.Is(error, ErrJSONContract) = %t, want %t", got, tc.wantJSON)
			}
			if tc.wantNative != nil && !errors.Is(gotErr, tc.wantNative) {
				t.Fatalf("errors.Is(error, native) = false, want true; error = %v", gotErr)
			}
		})
	}
}

func signContractErrorFixture(testing.TB) error {
	_, err := attest.Sign(attest.SignRequest[testDomain]{})
	return err
}

func verificationErrorFixture(t testing.TB) error {
	t.Helper()
	privateKey := deterministicPrivateKey(t, "error-hierarchy")
	body := literalBody{domain: testDomainPrimary, value: []byte("identity")}
	envelope := mustEnvelope(t, body, privateKey)
	envelope.Signature = mutateSignature(t, envelope.Signature)
	_, err := attest.Verify(attest.VerifyRequest[testDomain]{
		Body:        body,
		Envelope:    envelope,
		TrustedKeys: mustTrustedKeys(t, mustPublicKey(t, privateKey)),
	})
	return err
}

func jsonContractErrorFixture(testing.TB) error {
	var receiver attest.Envelope[testDomain]
	return receiver.UnmarshalJSON([]byte("{"))
}

func nativeCallbackErrorFixture(t testing.TB) error {
	t.Helper()
	_, err := attest.Sign(attest.SignRequest[testDomain]{
		Body: hostileBody{mode: hostileBodyWriteError},
		Key:  deterministicPrivateKey(t, "native-callback-error"),
	})
	return err
}

func TestVerifiedZeroValueCannotClaimProof(t *testing.T) {
	t.Parallel()

	var proof attest.Verified[testDomain]
	gotValidateErr := proof.Validate()
	if !errors.Is(gotValidateErr, core.ErrAttestVerification) {
		t.Fatalf("Verified.Validate() error = %v, want %v", gotValidateErr, core.ErrAttestVerification)
	}
	gotEnvelope, gotEnvelopeErr := proof.Envelope()
	if !errors.Is(gotEnvelopeErr, core.ErrAttestVerification) {
		t.Fatalf("Verified.Envelope() error = %v, want %v", gotEnvelopeErr, core.ErrAttestVerification)
	}
	if gotEnvelope != (attest.Envelope[testDomain]{}) {
		t.Fatalf("Verified.Envelope() = %+v, want zero", gotEnvelope)
	}
}
