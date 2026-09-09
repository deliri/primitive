package distribution_test

import (
	"bytes"
	"crypto"
	"crypto/ed25519"
	"errors"
	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/deploy"
	"github.com/deliri/primitive/v2026/distribution"
	"github.com/deliri/primitive/v2026/release"
	"io"
	"testing"
)

type refusingDistributionSigner struct {
	key   ed25519.PrivateKey
	calls int
}

func (s *refusingDistributionSigner) Public() crypto.PublicKey { return s.key.Public() }
func (s *refusingDistributionSigner) Sign(io.Reader, []byte, crypto.SignerOpts) ([]byte, error) {
	s.calls++
	return nil, io.ErrClosedPipe
}

func TestIssuanceRefusalsPreserveIdentityAndZeroOutput(t *testing.T) {
	t.Parallel()
	p := newPublicationExchangeFixture(t)
	u := newUpdateExchangeFixture(t)
	g := newUpgradeExchangeFixture(t)
	var sources [release.PublicationObjectCount]distribution.PublicationSource
	for i, payload := range p.release.payloads {
		sources[i] = distribution.PublicationSource{Reader: bytes.NewReader(payload)}
	}
	plan, err := distribution.PreparePublicationPlan(distribution.PublicationPlanRequest{Grant: p.verifiedGrant, Manifest: p.release.manifest, Sources: sources, Policy: objectstorePolicy(t)})
	if err != nil {
		t.Fatalf("PreparePublicationPlan()=%v, want nil", err)
	}
	receipts, err := deploy.ReleaseGCS(t.Context(), objectstoreClient(t, &publicationTransport{failAt: -1}), plan)
	if err != nil {
		t.Fatalf("ReleaseGCS()=%v, want nil", err)
	}
	issuers := []struct {
		name  string
		issue func(crypto.Signer) (bool, error)
	}{
		{"publication request", func(s crypto.Signer) (bool, error) {
			d, e := distribution.IssuePublicationRequest(distribution.PublicationRequestIssuance{Signer: s, Payload: p.request})
			return d == (distribution.PublicationRequestDocument{}), e
		}},
		{"update request", func(s crypto.Signer) (bool, error) {
			d, e := distribution.IssueUpdateRequest(distribution.UpdateRequestIssuance{Signer: s, Payload: u.request})
			return d == (distribution.UpdateRequestDocument{}), e
		}},
		{"upgrade request", func(s crypto.Signer) (bool, error) {
			d, e := distribution.IssueUpgradeRequest(distribution.UpgradeRequestIssuance{Signer: s, Payload: g.request})
			return d == (distribution.UpgradeRequestDocument{}), e
		}},
		{"update response", func(s crypto.Signer) (bool, error) {
			d, e := distribution.IssueUpdateResponse(distribution.UpdateResponseIssuance{Signer: s, Payload: u.responseDoc.Payload})
			return d == (distribution.UpdateResponseDocument{}), e
		}},
		{"publication completion", func(s crypto.Signer) (bool, error) {
			d, e := distribution.IssuePublicationCompletion(distribution.PublicationCompletionIssuance{Signer: s, Request: p.verifiedRequest, Grant: p.verifiedGrant, Receipts: receipts})
			return d == (distribution.PublicationCompletionProjection{}), e
		}},
		{"publication grant", func(s crypto.Signer) (bool, error) {
			d, e := distribution.IssuePublicationGrant(distribution.PublicationGrantIssuance{Signer: s, Payload: p.grantPayload, Capabilities: p.grantProjection.Capabilities})
			zero := d.Payload == (distribution.PublicationGrantPayload{}) && d.Attestation == (attest.Envelope[distribution.SigningDomain]{})
			for _, c := range d.Capabilities {
				zero = zero && c.IsZero()
			}
			return zero, e
		}},
		{"upgrade grant", func(s crypto.Signer) (bool, error) {
			d, e := distribution.IssueUpgradeGrant(distribution.UpgradeGrantIssuance{Signer: s, Payload: g.grantDoc.Payload, Capability: g.grantProjection.Capability})
			return d.Payload == (distribution.UpgradeGrantPayload{}) && d.Attestation == (attest.Envelope[distribution.SigningDomain]{}) && d.Capability.IsZero(), e
		}},
	}
	for _, issuer := range issuers {
		t.Run(issuer.name, func(t *testing.T) {
			t.Parallel()
			cases := []struct {
				name      string
				absent    bool
				wantCalls int
				wantCause error
			}{
				{name: "absent signer emits no document", absent: true, wantCause: core.ErrAttestContract},
				{name: "signing provider refusal emits no document", wantCalls: 1, wantCause: io.ErrClosedPipe},
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					provider := &refusingDistributionSigner{key: signingKey(71)}
					var signer crypto.Signer = provider
					if tc.absent {
						signer = nil
					}
					zero, err := issuer.issue(signer)
					if !zero || !errors.Is(err, core.ErrDistributionContract) || !errors.Is(err, tc.wantCause) || provider.calls != tc.wantCalls {
						t.Fatalf("issuance=(zero %t,%v,%d calls), want (true,%v,%d)", zero, err, provider.calls, tc.wantCause, tc.wantCalls)
					}
				})
			}
		})
	}
}

type preflightReader struct{ calls int }

func (r *preflightReader) Read([]byte) (int, error) { r.calls++; return 0, io.ErrNoProgress }
func TestPublicationPlanPreflightLayerTriad(t *testing.T) {
	t.Parallel()
	f := newPublicationExchangeFixture(t)
	cases := []struct {
		name      string
		missing   int
		zeroGrant bool
		wantErr   error
	}{
		{name: "exact plan preserves unread sources", missing: -1},
		{name: "absent grant emits no plan", missing: -1, zeroGrant: true, wantErr: core.ErrDistributionContract},
	}
	for i := range release.PublicationObjectCount {
		cases = append(cases, struct {
			name      string
			missing   int
			zeroGrant bool
			wantErr   error
		}{name: "absent source for " + release.PublicationRole(i+1).String(), missing: i, wantErr: core.ErrDistributionContract})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			probes := [release.PublicationObjectCount]preflightReader{}
			sources := [release.PublicationObjectCount]distribution.PublicationSource{}
			for i := range sources {
				sources[i].Reader = &probes[i]
			}
			if tc.missing >= 0 {
				sources[tc.missing].Reader = nil
			}
			grant := f.verifiedGrant
			if tc.zeroGrant {
				grant = distribution.VerifiedPublicationGrant{}
			}
			plan, err := distribution.PreparePublicationPlan(distribution.PublicationPlanRequest{Grant: grant, Manifest: f.release.manifest, Sources: sources, Policy: objectstorePolicy(t)})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("PreparePublicationPlan()=%v, want %v", err, tc.wantErr)
			}
			for i, p := range probes {
				if p.calls != 0 {
					t.Fatalf("source %d reads=%d, want zero", i, p.calls)
				}
			}
			if tc.wantErr == nil {
				if err := plan.Validate(); err != nil {
					t.Fatalf("plan.Validate()=%v, want nil", err)
				}
			} else if err := plan.Validate(); !errors.Is(err, core.ErrDeployContract) {
				t.Fatalf("refused plan.Validate()=%v, want %v", err, core.ErrDeployContract)
			}
		})
	}
}

func TestPublicationPlanForeignManifestHasNoSourceEffect(t *testing.T) {
	t.Parallel()
	f := newPublicationExchangeFixture(t)
	other := newReleaseFixture(t, core.NewReleaseVersion(2026, 0, 56), 3)
	cases := []struct {
		name     string
		manifest release.VerifiedManifest
		wantErr  error
	}{
		{name: "exact manifest", manifest: f.release.manifest},
		{name: "another authenticated manifest", manifest: other.manifest, wantErr: core.ErrDistributionBinding},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var probes [release.PublicationObjectCount]preflightReader
			var sources [release.PublicationObjectCount]distribution.PublicationSource
			for i := range sources {
				sources[i].Reader = &probes[i]
			}
			got, err := distribution.PreparePublicationPlan(distribution.PublicationPlanRequest{Grant: f.verifiedGrant, Manifest: tc.manifest, Sources: sources, Policy: objectstorePolicy(t)})
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("PreparePublicationPlan()=%v, want %v", err, tc.wantErr)
			}
			for i, p := range probes {
				if p.calls != 0 {
					t.Fatalf("source %d reads=%d, want zero", i, p.calls)
				}
			}
			if tc.wantErr != nil && got.Validate() == nil {
				t.Fatalf("refused plan validation=%v, want refusal", got.Validate())
			}
		})
	}
}
