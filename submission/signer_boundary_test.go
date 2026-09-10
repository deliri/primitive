package submission

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type submissionSigningProbe struct {
	private crypto.Signer
	cause   error
	calls   int
}

func (s *submissionSigningProbe) Public() crypto.PublicKey { return s.private.Public() }
func (s *submissionSigningProbe) Sign(reader io.Reader, data []byte, opts crypto.SignerOpts) ([]byte, error) {
	s.calls++
	if s.cause != nil {
		return nil, s.cause
	}
	return s.private.Sign(reader, data, opts)
}

type submissionIssuanceOutcome struct {
	err         error
	zero, exact bool
}

func TestSubmissionSignerFailureLayerTriad(t *testing.T) {
	t.Parallel()
	fixture := newCompletionFixture(t, submissionOffering(t, 2), []byte("signer failure"), 0x10)
	requestDocument, err := IssueRequest(RequestIssuance{Signer: fixture.deviceSigner, Payload: fixture.request})
	if err != nil {
		t.Fatal(err)
	}
	completion, err := IssueCompletion(CompletionIssuance{Signer: fixture.deviceSigner, Transfer: fixture.transfer, Request: fixture.request, Grant: fixture.grant, Nonce: fixture.nonce})
	if err != nil {
		t.Fatal(err)
	}
	grant := newGrantFixture(t, grantFixtureRequest{})
	_, authority := testSigningKey(t, 0x41)
	for _, door := range []struct {
		name  string
		key   crypto.Signer
		issue func(crypto.Signer, bool) submissionIssuanceOutcome
	}{
		{"request", fixture.deviceSigner, func(s crypto.Signer, invalid bool) submissionIssuanceOutcome {
			payload := fixture.request
			if invalid {
				payload = RequestPayload{}
			}
			got, err := IssueRequest(RequestIssuance{Signer: s, Payload: payload})
			return submissionIssuanceOutcome{err: err, zero: got == (RequestDocument{}), exact: got == requestDocument}
		}},
		{"grant", authority, func(s crypto.Signer, invalid bool) submissionIssuanceOutcome {
			payload := grant.payload
			if invalid {
				payload = GrantPayload{}
			}
			got, err := IssueGrant(GrantIssuance{Signer: s, Payload: payload, Capability: grant.projection.Capability})
			return submissionIssuanceOutcome{err: err, zero: grantProjectionIsZero(got), exact: got.Payload == grant.payload && got.Attestation == grant.document.Attestation && !got.Capability.IsZero()}
		}},
		{"completion", fixture.deviceSigner, func(s crypto.Signer, invalid bool) submissionIssuanceOutcome {
			request := fixture.request
			if invalid {
				request = RequestPayload{}
			}
			got, err := IssueCompletion(CompletionIssuance{Signer: s, Transfer: fixture.transfer, Request: request, Grant: fixture.grant, Nonce: fixture.nonce})
			return submissionIssuanceOutcome{err: err, zero: got == (CompletionProjection{}), exact: got == completion}
		}},
	} {
		t.Run(door.name, func(t *testing.T) {
			t.Parallel()
			for _, tc := range []struct {
				name               string
				cause              error
				nilSigner, invalid bool
				want               error
				calls              int
			}{
				{name: "exact_once", calls: 1},
				{name: "native_signer_failure", cause: io.ErrClosedPipe, want: io.ErrClosedPipe, calls: 1},
				{name: "signer_cancellation", cause: context.Canceled, want: context.Canceled, calls: 1},
				{name: "typed_nil_signer", nilSigner: true, want: core.ErrControlPlaneContract},
				{name: "neutral_invalid_request_no_signing", invalid: true, want: core.ErrControlPlaneContract},
			} {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					signer := &submissionSigningProbe{private: door.key, cause: tc.cause}
					var capability crypto.Signer = signer
					if tc.nilSigner {
						capability = (*ed25519.PrivateKey)(nil)
					}
					got := door.issue(capability, tc.invalid)
					if !errors.Is(got.err, tc.want) || signer.calls != tc.calls {
						t.Fatalf("error/calls=(%v,%d), want (%v,%d)", got.err, signer.calls, tc.want, tc.calls)
					}
					if tc.want != nil {
						if !got.zero || !errors.Is(got.err, core.ErrControlPlaneContract) {
							t.Fatalf("refused issuance retained output: %+v", got)
						}
						return
					}
					if !got.exact {
						t.Fatalf("got exact signed facts=%t, want true", got.exact)
					}
				})
			}
		})
	}
}
