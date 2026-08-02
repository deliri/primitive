package lease_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/lease"
)

func TestVerifyLayerTriad(t *testing.T) {
	t.Parallel()

	authority := fixtureAuthority(t, 7)
	subject := fixtureSubject(t, 11)
	decision := fixtureGrantDecision(t, subject, 1, 1_000, fixtureGrant())
	document, verified := fixtureVerified(t, authority, decision, subject)
	if err := verified.Validate(); err != nil {
		t.Fatalf("Verified.Validate() error = %v, want nil", err)
	}
	gotSubject, err := verified.Subject()
	if err != nil || gotSubject != subject {
		t.Fatalf("Verified.Subject() = (%v, %v), want (%v, nil)", gotSubject, err, subject)
	}
	gotEnvelope, err := verified.Envelope()
	if err != nil || gotEnvelope != document.Attestation {
		t.Fatalf("Verified.Envelope() differs from authentic envelope or returned error %v", err)
	}

	t.Run("positive authentic exact subject closes", func(t *testing.T) {
		t.Parallel()

		got, err := lease.Verify(lease.VerifyRequest{
			Document: document, TrustedKeys: authority.trusted,
			ExpectedSubject: subject,
		})
		if err != nil {
			t.Fatalf("lease.Verify() error = %v, want nil", err)
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("Verified.Validate() error = %v, want nil", err)
		}
	})

	t.Run("negative authentic wrong subject is typed", func(t *testing.T) {
		t.Parallel()

		other := fixtureSubject(t, 12)
		got, err := lease.Verify(lease.VerifyRequest{
			Document: document, TrustedKeys: authority.trusted,
			ExpectedSubject: other,
		})
		if !errors.Is(err, core.ErrLeaseVerification) ||
			!errors.Is(err, core.ErrLeaseScope) {
			t.Fatalf("lease.Verify() error = %v, want verification and scope identities", err)
		}
		var mismatch lease.ScopeMismatch
		if !errors.As(err, &mismatch) {
			t.Fatalf("lease.Verify() error = %v, want lease.ScopeMismatch", err)
		}
		expected, actual, factErr := mismatch.Subjects()
		if factErr != nil || expected != other || actual != subject {
			t.Fatalf("ScopeMismatch.Subjects() = (%v, %v, %v), want caller subject, signed subject, and nil", expected, actual, factErr)
		}
		if got != (lease.Verified{}) {
			t.Fatalf("lease.Verify() = %v, want zero", got)
		}
	})

	t.Run("neutral zero request reaches no trust claim", func(t *testing.T) {
		t.Parallel()

		got, err := lease.Verify(lease.VerifyRequest{})
		if !errors.Is(err, core.ErrLeaseContract) {
			t.Fatalf("lease.Verify() error = %v, want %v", err, core.ErrLeaseContract)
		}
		if got != (lease.Verified{}) {
			t.Fatalf("lease.Verify() = %v, want zero", got)
		}
	})
}

func TestVerifyAuthenticityPressure(t *testing.T) {
	t.Parallel()

	authority := fixtureAuthority(t, 21)
	otherAuthority := fixtureAuthority(t, 22)
	subject := fixtureSubject(t, 31)
	decision := fixtureGrantDecision(t, subject, 1, 1_000, fixtureGrant())
	document, _ := fixtureVerified(t, authority, decision, subject)
	changedDecision := fixtureGrantDecision(t, subject, 2, 1_001, fixtureGrant())
	changedDocument, _ := fixtureVerified(t, authority, changedDecision, subject)

	cases := []struct {
		wantErr   error
		wantCause error
		request   func(testing.TB) lease.VerifyRequest
		name      string
	}{
		{
			name: "trusted exact document",
			request: func(testing.TB) lease.VerifyRequest {
				return lease.VerifyRequest{
					Document: document, TrustedKeys: authority.trusted,
					ExpectedSubject: subject,
				}
			},
		},
		{
			name: "untrusted signer",
			request: func(testing.TB) lease.VerifyRequest {
				return lease.VerifyRequest{
					Document: document, TrustedKeys: otherAuthority.trusted,
					ExpectedSubject: subject,
				}
			},
			wantErr:   core.ErrLeaseVerification,
			wantCause: core.ErrAttestVerification,
		},
		{
			name: "wrong expected product entitlement and device",
			request: func(tb testing.TB) lease.VerifyRequest {
				return lease.VerifyRequest{
					Document: document, TrustedKeys: authority.trusted,
					ExpectedSubject: fixtureSubject(tb, 32),
				}
			},
			wantErr: core.ErrLeaseScope,
		},
		{
			name: "authentic envelope over another decision",
			request: func(testing.TB) lease.VerifyRequest {
				return lease.VerifyRequest{
					Document: lease.Document{
						Decision: decision, Attestation: changedDocument.Attestation,
					},
					TrustedKeys: authority.trusted, ExpectedSubject: subject,
				}
			},
			wantErr:   core.ErrLeaseVerification,
			wantCause: core.ErrAttestVerification,
		},
		{
			name: "changed decision under original envelope",
			request: func(testing.TB) lease.VerifyRequest {
				return lease.VerifyRequest{
					Document: lease.Document{
						Decision: changedDecision, Attestation: document.Attestation,
					},
					TrustedKeys: authority.trusted, ExpectedSubject: subject,
				}
			},
			wantErr:   core.ErrLeaseVerification,
			wantCause: core.ErrAttestVerification,
		},
		{
			name: "zero trusted keys",
			request: func(testing.TB) lease.VerifyRequest {
				return lease.VerifyRequest{
					Document: document, ExpectedSubject: subject,
				}
			},
			wantErr: core.ErrLeaseContract,
		},
		{
			name: "zero subject",
			request: func(testing.TB) lease.VerifyRequest {
				return lease.VerifyRequest{
					Document: document, TrustedKeys: authority.trusted,
				}
			},
			wantErr: core.ErrLeaseContract,
		},
		{
			name: "zero document",
			request: func(testing.TB) lease.VerifyRequest {
				return lease.VerifyRequest{
					TrustedKeys: authority.trusted, ExpectedSubject: subject,
				}
			},
			wantErr: core.ErrLeaseContract,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := lease.Verify(tc.request(t))
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("lease.Verify() error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantCause != nil && !errors.Is(err, tc.wantCause) {
				t.Fatalf("lease.Verify() error = %v, want nested %v", err, tc.wantCause)
			}
			if tc.wantErr != nil && got != (lease.Verified{}) {
				t.Fatalf("lease.Verify() = %v, want zero", got)
			}
		})
	}
}

func TestDocumentStrictJSONPressure(t *testing.T) {
	t.Parallel()

	authority := fixtureAuthority(t, 41)
	subject := fixtureSubject(t, 42)
	decision := fixtureGrantDecision(t, subject, 1, 1_000, fixtureGrant())
	document, _ := fixtureVerified(t, authority, decision, subject)
	canonical, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v, want nil", err)
	}

	cases := []struct {
		wantErr error
		name    string
		data    []byte
	}{
		{name: "canonical document", data: canonical},
		{name: "leading and trailing whitespace accepted", data: append(append([]byte(" \n"), canonical...), '\t')},
		{name: "empty document", data: nil, wantErr: core.ErrJSONContract},
		{name: "null document", data: []byte("null"), wantErr: core.ErrJSONContract},
		{name: "array document", data: []byte("[]"), wantErr: core.ErrJSONContract},
		{name: "truncated document", data: canonical[:len(canonical)-1], wantErr: core.ErrJSONContract},
		{name: "trailing value", data: append(append([]byte(nil), canonical...), []byte("true")...), wantErr: core.ErrJSONContract},
		{name: "unknown outer field", data: injectObjectField(canonical, `"future":1`), wantErr: core.ErrJSONContract},
		{name: "duplicate decision field", data: duplicateFirstField(canonical), wantErr: core.ErrJSONContract},
		{name: "case variant decision field", data: injectObjectField(canonical, `"Decision":{}`), wantErr: core.ErrJSONContract},
		{name: "over maximum whitespace", data: append(make([]byte, lease.DocumentJSONMaximumBytes+1), canonical...), wantErr: core.ErrJSONContract},
		{name: "invalid utf8", data: []byte{0xff}, wantErr: core.ErrJSONContract},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			sentinel := document
			err := sentinel.UnmarshalJSON(tc.data)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("json.Unmarshal() error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil && sentinel != document {
				t.Fatalf("rejected Document mutated receiver")
			}
			if tc.wantErr == nil {
				got, marshalErr := json.Marshal(sentinel)
				if marshalErr != nil {
					t.Fatalf("json.Marshal() error = %v, want nil", marshalErr)
				}
				if string(got) != string(canonical) {
					t.Fatalf("canonical document changed after decode")
				}
			}
		})
	}
}

func injectObjectField(document []byte, field string) []byte {
	result := make([]byte, 0, len(document)+len(field)+1)
	result = append(result, document[:len(document)-1]...)
	result = append(result, ',')
	result = append(result, field...)
	result = append(result, '}')
	return result
}

func duplicateFirstField(document []byte) []byte {
	const prefix = `{"decision":`
	if len(document) <= len(prefix) || string(document[:len(prefix)]) != prefix {
		return nil
	}
	return injectObjectField(document, `"decision":{}`)
}

func TestDocumentRejectsMismatchedDomainStructurally(t *testing.T) {
	t.Parallel()

	subject := fixtureSubject(t, 51)
	decision := fixtureGrantDecision(t, subject, 1, 1_000, fixtureGrant())
	document := lease.Document{
		Decision:    decision,
		Attestation: attest.Envelope[lease.Domain]{},
	}
	if !errors.Is(document.Validate(), core.ErrLeaseContract) {
		t.Fatalf("Document.Validate() error = %v, want %v", document.Validate(), core.ErrLeaseContract)
	}
}
