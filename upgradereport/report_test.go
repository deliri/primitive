package upgradereport

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlplanetest"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/receipt"
	"github.com/deliri/primitive/v2026/temporal"
)

func value[T any](t testing.TB) func(T, error) T {
	t.Helper()
	return func(v T, err error) T {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return v
	}
}

// The receipt is genuinely signed fixture material. These contract tests prove
// signature and scope mechanics; no provider bytes were uploaded by this fixture.
func reportFixture(t testing.TB) (Request, controlplanetest.Installation, attest.TrustedKeys, controlplane.Authority) {
	t.Helper()
	i := value[controlplanetest.Installation](t)(controlplanetest.IssueInstallation(controlplanetest.InstallationRequest{Offering: core.Offering{Token: "future-tool"}, AuthoritySeed: [32]byte{17}, DeviceSeed: [32]byte{23}}))
	t.Cleanup(func() { clear(i.AuthorityPrivate); clear(i.DevicePrivate) })
	keys := value[attest.TrustedKeys](t)(attest.NewTrustedKeys(attest.TrustedKeysRequest{Keys: []core.Ed25519PublicKey{i.AuthorityPublic}}))
	authority := value[controlplane.Authority](t)(controlplane.NewAuthority(controlplane.AuthorityConfiguration{TrustedAuthorityKeys: keys}))
	candidate := value[core.BuildIdentity](t)(core.NewBuildIdentity(core.BuildIdentityRequest{Offering: i.Build.Offering(), Version: core.NewReleaseVersion(2026, 1, 99), Commit: i.Build.Commit(), Platform: i.Build.Platform()}))
	e := value[receipt.EvidenceDocument](t)(receipt.IssueEvidence(receipt.IssueEvidenceRequest{Offering: i.Build.Offering(), Key: i.AuthorityPrivate, Principal: i.Certificate.Body.Account, OccurredAt: temporal.InstantFromNanoseconds(1800000000000000000), Identity: value[receipt.ReceiptID](t)(receipt.NewReceiptID([receipt.ReceiptIDBytes]byte{4})), Body: receipt.EvidenceBody{Submission: value[receipt.SubmissionIdentity](t)(receipt.NewSubmissionIdentity([16]byte{1})), Object: value[receipt.ObjectIdentity](t)(receipt.NewObjectIdentity([16]byte{2})), Extent: value[core.ByteLength](t)(core.NewByteLength(1)), SHA256: core.SHA256Of([]byte("x")), CRC32C: core.NewCRC32C(2839306131)}}))
	p := Payload{Attempt: value[controlwire.RequestNonce](t)(controlwire.NewRequestNonce([32]byte{7})), Installed: i.Build, Candidate: candidate, Evidence: e, ObservedAt: temporal.InstantFromNanoseconds(1800000000000000000), Revision: controlwire.Revision2026V1, Outcome: Failed, Stage: Trial}
	r := value[Request](t)(Sign(p, i.Certificate, i.DevicePrivate))
	return r, i, keys, authority
}

func TestReportLayerTriadAuthenticatesExactObservationAndAcknowledgment(t *testing.T) {
	t.Parallel()
	r, i, keys, authority := reportFixture(t)
	if err := r.Verify(authority, keys); err != nil {
		t.Fatalf("genuinely signed request = %v, want authenticated", err)
	}
	encoded := value[[]byte](t)(r.MarshalJSON())
	var decoded Request
	if err := decoded.UnmarshalJSON(encoded); err != nil || decoded != r {
		t.Fatalf("request round trip = (%v,%v), want exact signed request", decoded, err)
	}
	ack := value[Response](t)(SignAcknowledgment(Acknowledgment{Attempt: r.Payload.Attempt, RequestDigest: value[core.SHA256Digest](t)(r.Digest()), RecordedAt: r.Payload.ObservedAt}, i.AuthorityPrivate))
	if err := ack.Verify(keys, r); err != nil {
		t.Fatalf("acknowledgment = %v, want exact authenticated binding", err)
	}
	foreign := r
	foreign.Payload.Outcome = Cancelled
	if err := foreign.Verify(authority, keys); !errors.Is(err, core.ErrReportAuthentication) {
		t.Fatalf("changed outcome = %v, want ErrReportAuthentication", err)
	}
	if err := ack.Verify(keys, foreign); !errors.Is(err, core.ErrReportBinding) {
		t.Fatalf("ack for changed report = %v, want ErrReportBinding", err)
	}
	if err := decoded.UnmarshalJSON([]byte(`{}`)); !errors.Is(err, core.ErrReportContract) || decoded != r {
		t.Fatalf("empty request = (%v,%v), want refusal and preserved receiver", decoded, err)
	}
	if err := (Request{}).Verify(authority, keys); !errors.Is(err, core.ErrReportContract) {
		t.Fatalf("absent observation = %v, want no authenticated fact", err)
	}
}

func TestStageOutcomeExhaustiveProductObservationDomain(t *testing.T) {
	t.Parallel()
	r, i, keys, authority := reportFixture(t)
	// Exhaust every pair, including both unset values and the adjacent unknown
	// values. Primitive transports the declared stage and outcome independently;
	// only the product knows which observations complete its upgrade.
	for outcome := OutcomeUnknown; outcome <= Interrupted+1; outcome++ {
		for stage := StageUnknown; stage <= Complete+1; stage++ {
			p := r.Payload
			p.Outcome = outcome
			p.Stage = stage
			admitted := outcome >= Succeeded && outcome <= Interrupted && stage >= Bootstrap && stage <= Complete
			got, err := Sign(p, i.Certificate, i.DevicePrivate)
			if !admitted {
				wantErr := core.ErrReportContract
				if !errors.Is(err, wantErr) || got != (Request{}) {
					t.Fatalf("outcome=%d stage=%d = (%v,%v), want zero signed request and %v", outcome, stage, got, err, wantErr)
				}
				continue
			}
			if err != nil || got.Payload != p {
				t.Fatalf("outcome=%d stage=%d = (%v,%v), want exact admitted observation", outcome, stage, got, err)
			}
			if err := got.Verify(authority, keys); err != nil {
				t.Fatalf("outcome=%d stage=%d verification = %v, want nil", outcome, stage, err)
			}
		}
	}
}

func FuzzRequestAuthenticationAndCanonicalClosure(f *testing.F) {
	r, _, keys, authority := reportFixture(f)
	b := value[[]byte](f)(r.MarshalJSON())
	forged := r
	forged.Payload.Outcome = Cancelled
	f.Add(b)
	f.Add(value[[]byte](f)(forged.MarshalJSON()))
	f.Add([]byte{})
	f.Add([]byte(`{}`))
	f.Add(b[:len(b)-1])
	f.Add(append(bytes.Clone(b), b...))
	f.Fuzz(func(t *testing.T, data []byte) {
		got := r
		if err := got.UnmarshalJSON(data); err != nil {
			if (!errors.Is(err, core.ErrReportContract) && !errors.Is(err, core.ErrReportBinding)) || got != r {
				t.Fatalf("rejected request = (%v,%v), want stable refusal and preserved receiver", got, err)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("admitted request validation = %v, want nil", err)
		}
		encoded := value[[]byte](t)(got.MarshalJSON())
		var again Request
		if err := again.UnmarshalJSON(encoded); err != nil || again != got {
			t.Fatalf("canonical closure = (%v,%v), want admitted request", again, err)
		}
		if second := value[[]byte](t)(again.MarshalJSON()); !bytes.Equal(second, encoded) {
			t.Fatalf("second canonical projection=%q, want %q", second, encoded)
		}
		err := got.Verify(authority, keys)
		if got == r {
			if err != nil {
				t.Fatalf("genuinely signed seed = %v, want authenticated", err)
			}
		} else if !errors.Is(err, core.ErrReportAuthentication) && !errors.Is(err, core.ErrReportBinding) {
			t.Fatalf("unsigned recombination = %v, want authentication/binding refusal", err)
		}
	})
}
