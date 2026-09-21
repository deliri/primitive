package permit

import (
	"bytes"
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlplane"
	"github.com/deliri/primitive/v2026/controlplanetest"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func reportRequestFixture(t testing.TB, authoritySeed, deviceSeed byte) (ReportRequest, controlplane.Authority) {
	t.Helper()
	i, err := controlplanetest.IssueInstallation(controlplanetest.InstallationRequest{Offering: core.Offering{Token: "witness"}, AuthoritySeed: [32]byte{authoritySeed}, DeviceSeed: [32]byte{deviceSeed}})
	if err != nil {
		t.Fatalf("IssueInstallation() error = %v, want nil", err)
	}
	t.Cleanup(func() { clear(i.AuthorityPrivate); clear(i.DevicePrivate) })
	keys, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{Keys: []core.Ed25519PublicKey{i.AuthorityPublic}})
	if err != nil {
		t.Fatalf("trusted keys error = %v, want nil", err)
	}
	authority, err := controlplane.NewAuthority(controlplane.AuthorityConfiguration{TrustedAuthorityKeys: keys})
	if err != nil {
		t.Fatalf("authority error = %v, want nil", err)
	}
	scope := reportScopeFixture(t)
	scope.Installation = i.Certificate.Body.Subject.DeviceID
	previous, err := InitialReportDigest(scope)
	if err != nil {
		t.Fatalf("initial digest error = %v, want nil", err)
	}
	report, err := SignReportPayload(ReportPayload{Scope: scope, Sequence: 1, Previous: previous, Policy: reportSchedule(t).Policy,
		Window:   controlplane.UsageWindow{Bounds: temporal.IntervalBounds{Start: temporal.InstantFromNanoseconds(0), End: temporal.InstantFromNanoseconds(50)}, Freshness: temporal.InstantFromNanoseconds(50), Units: []controlplane.UsageCount{{Class: 1, Count: 2}}, Outcomes: []controlplane.OutcomeCount{{Class: 1, Count: 2}}},
		Evidence: ReportEvidence{Digest: core.SHA256Of([]byte("manifest")), Bytes: 8}}, i.DevicePrivate)
	if err != nil {
		t.Fatalf("SignReportPayload() error = %v, want nil", err)
	}
	return ReportRequest{Report: report, Certificate: i.Certificate}, authority
}

func TestReportRequestAuthenticationLayerTriad(t *testing.T) {
	t.Parallel()
	request, authority := reportRequestFixture(t, 17, 23)
	if err := request.Verify(authority); err != nil {
		t.Fatalf("Verify(issued request) error = %v, want nil", err)
	}
	encoded, err := request.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON() error = %v, want nil", err)
	}
	var roundTrip ReportRequest
	if err := roundTrip.UnmarshalJSON(encoded); err != nil {
		t.Fatalf("UnmarshalJSON() error = %v, want nil", err)
	}
	if err := roundTrip.Verify(authority); err != nil {
		t.Fatalf("Verify(round trip) error = %v, want nil", err)
	}
	second, err := roundTrip.MarshalJSON()
	if err != nil || !bytes.Equal(second, encoded) {
		t.Fatalf("canonical round trip = %q/%v, want original bytes", second, err)
	}
	if err := (ReportRequest{}).Verify(authority); !errors.Is(err, core.ErrReportContract) {
		t.Fatalf("Verify(absent request) error = %v, want %v", err, core.ErrReportContract)
	}
	if err := roundTrip.UnmarshalJSON(nil); !errors.Is(err, core.ErrReportContract) {
		t.Fatalf("UnmarshalJSON(absent) error = %v, want %v", err, core.ErrReportContract)
	}
	preserved, err := roundTrip.MarshalJSON()
	if err != nil || !bytes.Equal(preserved, encoded) {
		t.Fatalf("receiver after refusal = %q/%v, want original bytes", preserved, err)
	}
}

func TestReportRequestRejectsAuthenticForeignNominations(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		wantErr           error
		name              string
		authority, device byte
	}{
		{name: "foreign authority cannot nominate same device key", authority: 18, device: 23, wantErr: core.ErrReportAuthentication},
		{name: "same authority cannot substitute another device key", authority: 17, device: 24, wantErr: core.ErrReportBinding},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			original, authority := reportRequestFixture(t, 17, 23)
			foreign, _ := reportRequestFixture(t, tc.authority, tc.device)
			if foreign.Certificate == original.Certificate {
				t.Fatal("foreign certificate = original, want a real nomination mutation")
			}
			original.Certificate = foreign.Certificate
			if err := original.Verify(authority); !errors.Is(err, tc.wantErr) {
				t.Fatalf("Verify(foreign nomination) error = %v, want %v", err, tc.wantErr)
			}
		})
	}
}

func FuzzReportRequestSemanticClosure(f *testing.F) {
	seed, authority := reportRequestFixture(f, 17, 23)
	canonical, err := seed.MarshalJSON()
	if err != nil {
		f.Fatalf("MarshalJSON(seed) error = %v, want nil", err)
	}
	f.Add(canonical)
	f.Add([]byte{})
	f.Add([]byte(`{"report":null,"certificate":null}`))
	foreign, _ := reportRequestFixture(f, 18, 23)
	foreign.Report = seed.Report.Clone()
	encoded, err := foreign.MarshalJSON()
	if err != nil {
		f.Fatalf("MarshalJSON(foreign certificate) error = %v, want nil", err)
	}
	f.Add(encoded)
	f.Fuzz(func(t *testing.T, data []byte) {
		got := seed
		got.Report = seed.Report.Clone()
		decodeErr := got.UnmarshalJSON(data)
		encoded, encodeErr := got.MarshalJSON()
		if decodeErr != nil {
			if !errors.Is(decodeErr, core.ErrReportContract) || encodeErr != nil || !bytes.Equal(encoded, canonical) {
				t.Fatalf("decode refusal = %v, receiver=%q/%v, want typed refusal and preserved seed", decodeErr, encoded, encodeErr)
			}
			return
		}
		if encodeErr != nil || len(encoded) > ReportRequestMaximumBytes {
			t.Fatalf("accepted encoding = %d bytes/%v, want bounded canonical bytes", len(encoded), encodeErr)
		}
		verifyErr := got.Verify(authority)
		if verifyErr == nil && !bytes.Equal(encoded, canonical) {
			t.Fatalf("authenticated bytes = %q, want independently signed seed", encoded)
		}
		if verifyErr != nil && !errors.Is(verifyErr, core.ErrReportAuthentication) && !errors.Is(verifyErr, core.ErrReportBinding) {
			t.Fatalf("Verify() error = %v, want typed authentication/binding refusal", verifyErr)
		}
		var again ReportRequest
		if err := again.UnmarshalJSON(encoded); err != nil {
			t.Fatalf("decode canonical error = %v, want nil", err)
		}
		second, err := again.MarshalJSON()
		if err != nil || !bytes.Equal(encoded, second) {
			t.Fatalf("canonical closure = %q/%v, want %q", second, err, encoded)
		}
	})
}

// This ratchet attacks authenticated content after structural admission. Each
// mutation preserves a valid nominal request and changes one signed fact.
func TestReportRequestRejectsChangedSignedFacts(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		mutate func(*ReportRequest)
		name   string
	}{
		{name: "report sequence substituted", mutate: func(r *ReportRequest) { r.Report.Payload.Sequence++ }},
		{name: "report predecessor substituted", mutate: func(r *ReportRequest) { r.Report.Payload.Previous = core.SHA256Of([]byte("foreign predecessor")) }},
		{name: "report evidence digest substituted", mutate: func(r *ReportRequest) { r.Report.Payload.Evidence.Digest = core.SHA256Of([]byte("foreign evidence")) }},
		{name: "report evidence extent substituted", mutate: func(r *ReportRequest) { r.Report.Payload.Evidence.Bytes++ }},
		{name: "report evidence extent at signed integer maximum", mutate: func(r *ReportRequest) { r.Report.Payload.Evidence.Bytes = math.MaxInt64 }},
		{name: "report company substituted", mutate: func(r *ReportRequest) { r.Report.Payload.Scope.Company = r.Report.Payload.Scope.Project }},
		{name: "report project substituted", mutate: func(r *ReportRequest) { r.Report.Payload.Scope.Project = r.Report.Payload.Scope.Company }},
		{name: "report epoch substituted", mutate: func(r *ReportRequest) { r.Report.Payload.Scope.Epoch = r.Report.Payload.Scope.Company }},
		{name: "report interval start substituted", mutate: func(r *ReportRequest) { r.Report.Payload.Window.Bounds.Start = temporal.InstantFromNanoseconds(1) }},
		{name: "report interval end substituted", mutate: func(r *ReportRequest) { r.Report.Payload.Window.Bounds.End = temporal.InstantFromNanoseconds(51) }},
		{name: "report attested digest substituted", mutate: func(r *ReportRequest) { r.Report.Attestation.BodySHA256 = core.SHA256Of([]byte("foreign body")) }},
		{name: "report attested length substituted", mutate: func(r *ReportRequest) { r.Report.Attestation.BodyLength, _ = core.NewByteCount(1) }},
		{name: "certificate issue time substituted", mutate: func(r *ReportRequest) { r.Certificate.Body.IssuedAt = temporal.InstantFromNanoseconds(1) }},
		{name: "certificate attested digest substituted", mutate: func(r *ReportRequest) {
			r.Certificate.Attestation.BodySHA256 = core.SHA256Of([]byte("foreign certificate"))
		}},
		{name: "certificate attested length substituted", mutate: func(r *ReportRequest) { r.Certificate.Attestation.BodyLength, _ = core.NewByteCount(1) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, authority := reportRequestFixture(t, 17, 23)
			before, err := got.MarshalJSON()
			if err != nil || got.Verify(authority) != nil {
				t.Fatalf("baseline encoding/verification = %v/%v, want nil/nil", err, got.Verify(authority))
			}
			tc.mutate(&got)
			after, err := got.MarshalJSON()
			if err != nil || bytes.Equal(before, after) {
				t.Fatalf("mutation encoding = %v, unchanged = %t, want valid changed signed fact", err, bytes.Equal(before, after))
			}
			var admitted ReportRequest
			if err := admitted.UnmarshalJSON(after); err != nil {
				t.Fatalf("structural admission error = %v, want nil before authentication", err)
			}
			if err := admitted.Verify(authority); !errors.Is(err, core.ErrReportAuthentication) {
				t.Fatalf("Verify(changed signed fact) error = %v, want %v", err, core.ErrReportAuthentication)
			}
		})
	}
}

func TestReportRequestDocumentExtentBoundary(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		wantErr error
		name    string
		length  int
	}{
		{name: "one below request byte ceiling", length: ReportRequestMaximumBytes - 1, wantErr: nil},
		{name: "exact request byte ceiling", length: ReportRequestMaximumBytes, wantErr: nil},
		{name: "one above request byte ceiling", length: ReportRequestMaximumBytes + 1, wantErr: core.ErrReportContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			seed, authority := reportRequestFixture(t, 17, 23)
			canonical, err := seed.MarshalJSON()
			if err != nil || len(canonical) >= tc.length {
				t.Fatalf("seed encoding = %d bytes/%v, want below %d", len(canonical), err, tc.length)
			}
			input := append(bytes.Clone(canonical), bytes.Repeat([]byte{' '}, tc.length-len(canonical))...)
			got := seed
			got.Report = seed.Report.Clone()
			err = got.UnmarshalJSON(input)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("UnmarshalJSON(%d bytes) error = %v, want %v", len(input), err, tc.wantErr)
			}
			encoded, err := got.MarshalJSON()
			if err != nil || !bytes.Equal(encoded, canonical) || got.Verify(authority) != nil {
				t.Fatalf("accepted/preserved request = %q/%v, want original authenticated bytes", encoded, err)
			}
		})
	}
}
