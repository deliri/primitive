package upgradereport

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/chit"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlplanetest"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/submission"
	"github.com/deliri/primitive/v2026/submissionauth"
)

func evidenceFixture(t testing.TB) (EvidenceRequest, controlplanetest.Installation, controlplane.Authority) {
	t.Helper()
	r, installation, _, authority := reportFixture(t)
	payload := submission.RequestPayload{
		Build: installation.Build, Nonce: value[controlwire.RequestNonce](t)(controlwire.NewRequestNonce([32]byte{19})), Revision: controlwire.Revision2026V1,
		Declaration: submission.Declaration{ContentType: core.HTTPMediaTypeOctetStream(), Extent: r.Payload.Evidence.Payload.Body.Extent, SHA256: r.Payload.Evidence.Payload.Body.SHA256, CRC32C: r.Payload.Evidence.Payload.Body.CRC32C},
		Manifest: submission.ManifestIntent{
			Upload:     value[submission.UploadID](t)(submission.ParseUploadID("00000000-0006-7000-8000-000000000006")),
			Collection: value[chit.CollectionID](t)(chit.ParseCollectionID("00000000-0007-7000-8000-000000000007")),
			Partition:  value[chit.Partition](t)(chit.NewPartition(core.SHA256Of([]byte("fixture partition")))),
			Name:       value[chit.EntryName](t)(chit.ParseEntryName("fixture.bin")), Sequence: value[chit.EntrySequence](t)(chit.NewEntrySequence(1)), Objects: value[chit.ObjectCount](t)(chit.NewObjectCount(1)),
		},
	}
	inner := value[submission.RequestDocument](t)(submission.IssueRequest(submission.RequestIssuance{Payload: payload, Signer: installation.DevicePrivate}))
	credentialed := value[submissionauth.RequestDocument](t)(submissionauth.Assemble(submissionauth.RequestAssembly{Request: inner, Certificate: installation.Certificate}))
	return value[EvidenceRequest](t)(SignEvidence(r.Payload.Attempt, credentialed, installation.DevicePrivate)), installation, authority
}

func TestEvidenceLayerTriadAuthenticatesBothSignaturesAndExactAttempt(t *testing.T) {
	t.Parallel()
	r, installation, authority := evidenceFixture(t)
	if err := r.Verify(authority); err != nil {
		t.Fatalf("signed evidence = %v, want authenticated", err)
	}
	wire := value[[]byte](t)(r.MarshalJSON())
	var decoded EvidenceRequest
	if err := decoded.UnmarshalJSON(wire); err != nil || decoded != r {
		t.Fatalf("evidence decode=(%v,%v), want exact signed request", decoded, err)
	}
	route := value[controlwire.RouteContract](t)(r.ControlRoute())
	wantRoute := value[controlwire.RouteContract](t)(controlwire.NewRouteContract(installation.Build.Offering(), controlwire.RouteFamilyUpgradeEvidence))
	if route != wantRoute || r.ControlNonce() != r.Payload.Attempt || r.ControlRevision() != r.Submission.Request.Payload.Revision {
		t.Fatalf("route=%v nonce=%v revision=%v, want %v/%v/%v", route, r.ControlNonce(), r.ControlRevision(), wantRoute, r.Payload.Attempt, r.Submission.Request.Payload.Revision)
	}
	changedAttempt := value[controlwire.RequestNonce](t)(controlwire.NewRequestNonce([32]byte{8}))
	cases := []struct {
		wantErr error
		mutate  func(EvidenceRequest) EvidenceRequest
		name    string
	}{
		{name: "outer attempt changed without signature", mutate: func(got EvidenceRequest) EvidenceRequest {
			got.Payload.Attempt = changedAttempt
			return got
		}, wantErr: core.ErrReportAuthentication},
		{name: "outer signature replaced with valid foreign signature", mutate: func(got EvidenceRequest) EvidenceRequest {
			got.Attestation.Signature = got.Submission.Certificate.Attestation.Signature
			return got
		}, wantErr: core.ErrReportAuthentication},
		{name: "inner signature changed while outer remains genuine", mutate: func(got EvidenceRequest) EvidenceRequest {
			got.Submission.Request.Attestation.Signature = got.Submission.Certificate.Attestation.Signature
			return got
		}, wantErr: core.ErrReportAuthentication},
		{name: "certificate signature changed with both requests genuine", mutate: func(got EvidenceRequest) EvidenceRequest {
			got.Submission.Certificate.Attestation.Signature = got.Attestation.Signature
			return got
		}, wantErr: core.ErrReportAuthentication},
		{name: "declaration changed without recomputing commitment", mutate: func(got EvidenceRequest) EvidenceRequest {
			got.Submission.Request.Payload.Declaration.SHA256 = core.SHA256Of([]byte("different"))
			return got
		}, wantErr: core.ErrReportBinding},
		{name: "outer signer disagrees with nominated device", mutate: func(got EvidenceRequest) EvidenceRequest {
			got.Attestation.Signer = installation.AuthorityPublic
			return got
		}, wantErr: core.ErrReportBinding},
		{name: "wrong signed domain cannot cross endpoint", mutate: func(got EvidenceRequest) EvidenceRequest { got.Attestation.Domain = DomainObservation; return got }, wantErr: core.ErrReportBinding},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.mutate(r)
			if got == r {
				t.Fatalf("mutated request=%v, want a change from %v", got, r)
			}
			if err := got.Verify(authority); !errors.Is(err, tc.wantErr) {
				t.Fatalf("evidence verification=%v, want %v", err, tc.wantErr)
			}
		})
	}
	if err := decoded.UnmarshalJSON([]byte("null")); !errors.Is(err, core.ErrReportContract) || decoded != r {
		t.Fatalf("absent evidence=(%v,%v), want refusal and preserved receiver", decoded, err)
	}
	if got, err := SignEvidence(controlwire.RequestNonce{}, r.Submission, installation.DevicePrivate); !errors.Is(err, core.ErrReportAuthentication) || got != (EvidenceRequest{}) {
		t.Fatalf("absent attempt=(%v,%v), want zero and signing refusal", got, err)
	}
}

func FuzzEvidenceAuthenticationAndCanonicalClosure(f *testing.F) {
	r, _, authority := evidenceFixture(f)
	wire := value[[]byte](f)(r.MarshalJSON())
	forged := r
	forged.Submission.Request.Attestation.Signature = forged.Attestation.Signature
	f.Add(wire)
	f.Add(value[[]byte](f)(forged.MarshalJSON()))
	f.Add([]byte("null"))
	f.Add(wire[:len(wire)-1])
	f.Add(append(bytes.Clone(wire), wire...))
	f.Fuzz(func(t *testing.T, data []byte) {
		got := r
		if err := got.UnmarshalJSON(data); err != nil {
			if (!errors.Is(err, core.ErrReportContract) && !errors.Is(err, core.ErrReportBinding)) || got != r {
				t.Fatalf("decode refusal=(%v,%v), want preserved receiver and typed refusal", got, err)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatal(err)
		}
		canonical := value[[]byte](t)(got.MarshalJSON())
		var again EvidenceRequest
		if err := again.UnmarshalJSON(canonical); err != nil || again != got {
			t.Fatalf("canonical decode=(%v,%v), want exact accepted request", again, err)
		}
		if second := value[[]byte](t)(again.MarshalJSON()); !bytes.Equal(second, canonical) {
			t.Fatalf("second canonical write=%q, want %q", second, canonical)
		}
		err := got.Verify(authority)
		if got == r {
			if err != nil {
				t.Fatalf("genuine evidence refused: %v", err)
			}
		} else if !errors.Is(err, core.ErrReportAuthentication) && !errors.Is(err, core.ErrReportBinding) {
			t.Fatalf("unsigned recombination verification=%v, want refusal", err)
		}
	})
}

func FuzzEvidenceResignedOuterCannotAuthorizeForgedInner(f *testing.F) {
	r, installation, authority := evidenceFixture(f)
	f.Add(uint8(0))
	f.Add(uint8(1))
	f.Fuzz(func(t *testing.T, selector uint8) {
		got := r
		if selector%2 == 0 {
			got.Submission.Request.Attestation.Signature = r.Attestation.Signature
		} else {
			got.Submission.Certificate.Attestation.Signature = r.Attestation.Signature
		}
		if got == r {
			t.Fatalf("mutated inner signature=%v, want a change from %v", got.Submission, r.Submission)
		}
		got.Attestation = value[attest.Envelope[Domain]](t)(attest.Sign(attest.SignRequest[Domain]{Body: got.Payload, Signer: installation.DevicePrivate}))
		if err := got.Validate(); err != nil {
			t.Fatalf("outer-valid forged-inner seed did not reach verifier: %v", err)
		}
		if err := got.Verify(authority); !errors.Is(err, core.ErrReportAuthentication) {
			t.Fatalf("genuine outer around forged inner=%v, want authentication refusal", err)
		}
	})
}
