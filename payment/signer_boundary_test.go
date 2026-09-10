package payment

import (
	"crypto"
	"crypto/ed25519"
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type paymentSigningFailure struct {
	private ed25519.PrivateKey
	cause   error
	calls   int
}

func (s *paymentSigningFailure) Public() crypto.PublicKey { return s.private.Public() }
func (s *paymentSigningFailure) Sign(r io.Reader, data []byte, opts crypto.SignerOpts) ([]byte, error) {
	s.calls++
	if s.cause != nil {
		return nil, s.cause
	}
	return s.private.Sign(r, data, opts)
}

type paymentIssuanceOutcome struct {
	err         error
	zero, exact bool
}

func TestPaymentSignerFailureLayerTriad(t *testing.T) {
	t.Parallel()
	f := paymentFixturesForFuzz(t)
	for _, door := range []struct {
		name    string
		private ed25519.PrivateKey
		issue   func(crypto.Signer, bool) paymentIssuanceOutcome
	}{
		{name: "receipt", private: f.payment.private, issue: func(s crypto.Signer, invalid bool) paymentIssuanceOutcome {
			payload := f.payload
			if invalid {
				payload = Payload{}
			}
			got, err := Issue(Issuance{Signer: s, Payload: payload})
			return paymentIssuanceOutcome{err: err, zero: got == (Document{}), exact: got == f.document}
		}},
		{name: "query", private: f.query.private, issue: func(s crypto.Signer, invalid bool) paymentIssuanceOutcome {
			payload := f.queryPayload
			if invalid {
				payload = QueryPayload{}
			}
			got, err := IssueQuery(QueryIssuance{Signer: s, Payload: payload})
			return paymentIssuanceOutcome{err: err, zero: got == (QueryDocument{}), exact: got == f.queryDocument}
		}},
		{name: "catalog", private: f.catalog.private, issue: func(s crypto.Signer, invalid bool) paymentIssuanceOutcome {
			payload := f.catalogPayload
			if invalid {
				payload = CatalogPayload{}
			}
			got, err := IssueCatalog(CatalogIssuance{Signer: s, Payload: payload})
			return paymentIssuanceOutcome{err: err, zero: samePaymentCatalogDocument(got, CatalogDocument{}), exact: samePaymentCatalogDocument(got, f.catalogDocument)}
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
				{name: "positive exact signed document", calls: 1},
				{name: "negative signer native failure", cause: io.ErrClosedPipe, want: io.ErrClosedPipe, calls: 1},
				{name: "negative typed nil signer", nilSigner: true, want: core.ErrPaymentContract},
				{name: "neutral invalid payload performs no signing", invalid: true, want: core.ErrPaymentContract},
			} {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					signer := &paymentSigningFailure{private: door.private, cause: tc.cause}
					var capability crypto.Signer = signer
					if tc.nilSigner {
						capability = (*ed25519.PrivateKey)(nil)
					}
					got := door.issue(capability, tc.invalid)
					if !errors.Is(got.err, tc.want) || signer.calls != tc.calls {
						t.Fatalf("issue error/calls = (%v,%d), want (%v,%d)", got.err, signer.calls, tc.want, tc.calls)
					}
					if tc.want != nil {
						if !got.zero || !errors.Is(got.err, core.ErrPaymentContract) {
							t.Fatalf("failed issue = %+v, want zero and typed payment refusal", got)
						}
					} else if !got.exact {
						t.Fatalf("issued result = %+v, want exact signed fixture", got)
					}
				})
			}
		})
	}
}
