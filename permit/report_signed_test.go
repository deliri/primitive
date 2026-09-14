package permit

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/id"
	"github.com/deliri/primitive/v2026/temporal"
)

func reportScopeFixture(t testing.TB) ReportScope {
	t.Helper()
	r, _ := permitFixture(t)
	company, err := id.NewULIDFromBytes([16]byte{1})
	if err != nil {
		t.Fatalf("company = %v, want nil", err)
	}
	epoch, err := id.NewULIDFromBytes([16]byte{2})
	if err != nil {
		t.Fatalf("epoch = %v, want nil", err)
	}
	project, err := ParseReportProjectID("parser")
	if err != nil {
		t.Fatalf("project = %v, want nil", err)
	}
	return ReportScope{Company: company, Offering: r.Subject.Offering, Installation: r.Subject.DeviceID, Project: project, Epoch: epoch}
}
func TestReportSignedLayerTriad(t *testing.T) {
	t.Parallel()
	r, key := permitFixture(t)
	scope := reportScopeFixture(t)
	schedule := reportSchedule(t)
	p := ProjectPermission{Scope: scope, IssuedAt: temporal.InstantFromNanoseconds(0), NotBefore: temporal.InstantFromNanoseconds(0), ExpiresAt: temporal.InstantFromNanoseconds(1000), Schedule: schedule}
	signed, err := SignProjectPermission(p, key)
	if err != nil {
		t.Fatalf("SignProjectPermission() = %v, want nil", err)
	}
	if err := signed.Verify(r.TrustedKeys, scope); err != nil {
		t.Fatalf("Verify() = %v, want nil", err)
	}
	altered := signed
	altered.Payload.Schedule.NextReportAt = temporal.InstantFromNanoseconds(101)
	if err := altered.Verify(r.TrustedKeys, scope); !errors.Is(err, core.ErrReportAuthentication) {
		t.Fatalf("mutated schedule Verify() = %v, want %v", err, core.ErrReportAuthentication)
	}
	foreign := scope
	foreign.Epoch, _ = id.NewULIDFromBytes([16]byte{3})
	if err := signed.Verify(r.TrustedKeys, foreign); !errors.Is(err, core.ErrReportBinding) {
		t.Fatalf("foreign epoch Verify() = %v, want %v", err, core.ErrReportBinding)
	}
	if got, err := SignProjectPermission(ProjectPermission{}, key); err == nil || got != (SignedProjectPermission{}) {
		t.Fatalf("zero permission = %+v/%v, want zero/refusal", got, err)
	}
}

// Every mutated input reaches a real signed decoder and the real verifier.
// Re-encoding must equal the independently signed seed whenever verification succeeds.
func FuzzReportSignedSemanticClosure(f *testing.F) {
	r, key := permitFixture(f)
	scope := reportScopeFixture(f)
	schedule := reportSchedule(f)
	initial, err := InitialReportDigest(scope)
	if err != nil {
		f.Fatalf("InitialReportDigest() = %v, want nil", err)
	}
	report, err := SignReportPayload(ReportPayload{Scope: scope, Sequence: 1, Previous: initial, Window: controlplane.UsageWindow{Bounds: temporal.IntervalBounds{Start: temporal.InstantFromNanoseconds(0), End: temporal.InstantFromNanoseconds(50)}, Freshness: temporal.InstantFromNanoseconds(50), Units: []controlplane.UsageCount{{Class: 1, Count: 2}}, Outcomes: []controlplane.OutcomeCount{{Class: 1, Count: 2}}}, Evidence: ReportEvidence{Digest: core.SHA256Of([]byte("manifest")), Bytes: 8}, Policy: schedule.Policy}, key)
	if err != nil {
		f.Fatalf("SignReportPayload() = %v, want nil", err)
	}
	digest, err := report.Digest()
	if err != nil {
		f.Fatalf("Digest() = %v, want nil", err)
	}
	permission, err := SignProjectPermission(ProjectPermission{Scope: scope, IssuedAt: temporal.InstantFromNanoseconds(0), NotBefore: temporal.InstantFromNanoseconds(0), ExpiresAt: temporal.InstantFromNanoseconds(1000), Schedule: schedule}, key)
	if err != nil {
		f.Fatalf("SignProjectPermission() = %v, want nil", err)
	}
	ack, err := SignReportAcknowledgment(ReportAcknowledgment{Scope: scope, Sequence: 1, ReportDigest: digest, ProjectRevision: 1, AcceptedAt: temporal.InstantFromNanoseconds(0), Schedule: schedule}, key)
	if err != nil {
		f.Fatalf("SignReportAcknowledgment() = %v, want nil", err)
	}
	reportSeed, err := report.MarshalJSON()
	if err != nil {
		f.Fatalf("report MarshalJSON() = %v, want nil", err)
	}
	permissionSeed, err := permission.MarshalJSON()
	if err != nil {
		f.Fatalf("permission MarshalJSON() = %v, want nil", err)
	}
	ackSeed, err := ack.MarshalJSON()
	if err != nil {
		f.Fatalf("ack MarshalJSON() = %v, want nil", err)
	}
	f.Add(uint8(0), reportSeed)
	f.Add(uint8(1), permissionSeed)
	f.Add(uint8(2), ackSeed)
	f.Add(uint8(0), []byte{})
	f.Add(uint8(1), []byte(`{"payload":null}`))
	f.Fuzz(func(t *testing.T, kind uint8, data []byte) {
		var gotBytes, wantBytes, second []byte
		var decodeErr, verifyErr, encodeErr, error2 error
		switch kind % 3 {
		case 0:
			got := report.Clone()
			decodeErr = got.UnmarshalJSON(data)
			gotBytes, encodeErr = got.MarshalJSON()
			wantBytes = reportSeed
			if decodeErr == nil {
				verifyErr = got.Verify(r.TrustedKeys, scope)
				var again SignedReport
				error2 = again.UnmarshalJSON(gotBytes)
				if error2 == nil {
					second, error2 = again.MarshalJSON()
				}
			}
		case 1:
			got := permission
			decodeErr = got.UnmarshalJSON(data)
			gotBytes, encodeErr = got.MarshalJSON()
			wantBytes = permissionSeed
			if decodeErr == nil {
				verifyErr = got.Verify(r.TrustedKeys, scope)
				var again SignedProjectPermission
				error2 = again.UnmarshalJSON(gotBytes)
				if error2 == nil {
					second, error2 = again.MarshalJSON()
				}
			}
		case 2:
			got := ack
			decodeErr = got.UnmarshalJSON(data)
			gotBytes, encodeErr = got.MarshalJSON()
			wantBytes = ackSeed
			if decodeErr == nil {
				verifyErr = got.Verify(r.TrustedKeys, scope)
				var again SignedReportAcknowledgment
				error2 = again.UnmarshalJSON(gotBytes)
				if error2 == nil {
					second, error2 = again.MarshalJSON()
				}
			}
		}
		if encodeErr != nil || len(gotBytes) > ReportDocumentMaximumBytes {
			t.Fatalf("marshal accepted/preserved = %v/%d, want nil/bounded", encodeErr, len(gotBytes))
		}
		if decodeErr != nil {
			if !errors.Is(decodeErr, core.ErrReportContract) && !errors.Is(decodeErr, core.ErrReportSchedule) && !errors.Is(decodeErr, core.ErrReportBinding) {
				t.Fatalf("decode refusal = %v, want typed report error", decodeErr)
			}
			if !bytes.Equal(gotBytes, wantBytes) {
				t.Fatalf("rejected receiver preserved = false, want true")
			}
			return
		}
		if error2 != nil || !bytes.Equal(second, gotBytes) {
			t.Fatalf("canonical closure = %v/%t, want nil/true", error2, bytes.Equal(second, gotBytes))
		}
		if verifyErr == nil && !bytes.Equal(gotBytes, wantBytes) {
			t.Fatalf("authenticated mutation equals signed seed = false, want true")
		}
		if verifyErr != nil && !errors.Is(verifyErr, core.ErrReportAuthentication) && !errors.Is(verifyErr, core.ErrReportBinding) {
			t.Fatalf("verification refusal = %v, want typed binding/authentication", verifyErr)
		}
	})
}
func FuzzReportProjectAndDomain(f *testing.F) {
	project, err := ParseReportProjectID("parser")
	if err != nil {
		f.Fatalf("project = %v, want nil", err)
	}
	seed, err := project.MarshalJSON()
	if err != nil {
		f.Fatalf("marshal = %v, want nil", err)
	}
	f.Add(seed)
	f.Add([]byte("primitive-project-report-v1"))
	f.Fuzz(func(t *testing.T, data []byte) {
		got := project
		err := got.UnmarshalJSON(data)
		if err != nil {
			if !errors.Is(err, core.ErrReportContract) || got != project {
				t.Fatalf("project rejection = %v/%v, want preserved/typed", got, err)
			}
		} else {
			encoded, err := got.MarshalJSON()
			if err != nil || got.Validate() != nil {
				t.Fatalf("accepted project = %v/%v, want valid/nil", got, err)
			}
			var again ReportProjectID
			if err := again.UnmarshalJSON(encoded); err != nil || again != got {
				t.Fatalf("project closure = %v/%v, want %v/nil", again, err, got)
			}
		}
		domain, err := (ReportDomainUnknown).ParseCanonicalText(data)
		if err != nil {
			if domain != ReportDomainUnknown || !errors.Is(err, core.ErrReportContract) {
				t.Fatalf("domain refusal = %v/%v, want zero/typed", domain, err)
			}
			return
		}
		encoded, err := domain.MarshalText()
		if err != nil || !bytes.Equal(encoded, data) {
			t.Fatalf("domain closure = %q/%v, want %q/nil", encoded, err, data)
		}
	})
}
