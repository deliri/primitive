package controlplane_test

import (
	"bytes"
	"testing"

	"github.com/deliri/primitive/v2026/controlplane"
)

// These fixed workloads use real canonical documents and real Ed25519 verification.
// Fixture construction and signing are excluded; each benchmark observes its result.
func BenchmarkRegistrationCodec(b *testing.B) {
	b.ReportAllocs()
	issued := issueTestRegistration(b)
	encoded, err := issued.document.MarshalJSON()
	if err != nil || len(encoded) == 0 {
		b.Fatalf("registration fixture = (%d bytes, %v), want nonempty and nil", len(encoded), err)
	}
	b.Run("marshal", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(encoded)))
		var last []byte
		for b.Loop() {
			last, err = issued.document.MarshalJSON()
			if err != nil {
				b.Fatalf("MarshalJSON() error = %v, want nil", err)
			}
		}
		if !bytes.Equal(last, encoded) {
			b.Fatalf("MarshalJSON() = %d bytes, want exact %d canonical bytes", len(last), len(encoded))
		}
	})
	b.Run("unmarshal", func(b *testing.B) {
		b.ReportAllocs()
		b.SetBytes(int64(len(encoded)))
		var last controlplane.RegistrationDocument
		for b.Loop() {
			if err := last.UnmarshalJSON(encoded); err != nil {
				b.Fatalf("UnmarshalJSON() error = %v, want nil", err)
			}
		}
		got, err := last.MarshalJSON()
		if err != nil || !bytes.Equal(got, encoded) {
			b.Fatalf("decoded projection = (%d bytes, %v), want exact %d bytes", len(got), err, len(encoded))
		}
	})
}

func BenchmarkVerifyRegistration(b *testing.B) {
	issued := issueTestRegistration(b)
	client := issued.client(b)
	request := issued.verification()
	if err := request.Validate(); err != nil {
		b.Fatalf("verification fixture error = %v, want nil", err)
	}
	b.ReportAllocs()
	var last controlplane.VerifiedRegistration
	for b.Loop() {
		got, err := client.VerifyRegistration(request)
		if err != nil {
			b.Fatalf("VerifyRegistration() error = %v, want nil", err)
		}
		last = got
	}
	body, err := last.Payload()
	if err != nil || body.Header != issued.document.Payload.Header {
		b.Fatalf("verified header = (%+v, %v), want %+v", body.Header, err, issued.document.Payload.Header)
	}
}

func BenchmarkVerifyCheckIn(b *testing.B) {
	issued := issueTestCheckIn(b, controlplaneOffering(b, 3), testCheckInWindow())
	server := issued.server(b)
	request := controlplane.CheckInVerification{Request: issued.request}
	if err := request.Validate(); err != nil {
		b.Fatalf("verification fixture error = %v, want nil", err)
	}
	b.ReportAllocs()
	var last controlplane.VerifiedCheckIn
	for b.Loop() {
		got, err := server.VerifyCheckIn(request)
		if err != nil {
			b.Fatalf("VerifyCheckIn() error = %v, want nil", err)
		}
		last = got
	}
	got, err := last.Request()
	if err != nil || got.Attestation != issued.request.Attestation {
		b.Fatalf("verified attestation = (%v, %v), want %v", got.Attestation, err, issued.request.Attestation)
	}
}

func BenchmarkCommitCheckIn(b *testing.B) {
	issued := issueTestCheckIn(b, controlplaneOffering(b, 3), testCheckInWindow())
	server := issued.server(b)
	verified, err := server.VerifyCheckIn(controlplane.CheckInVerification{Request: issued.request})
	if err != nil {
		b.Fatalf("VerifyCheckIn() error = %v, want nil", err)
	}
	request := controlplane.CheckInCommitRequest{CheckIn: verified, Current: issued.request.Payload.PreviousWatermark, RequiredPolicy: issued.request.Payload.AppliedPolicy}
	if err := request.Validate(); err != nil {
		b.Fatalf("commit fixture error = %v, want nil", err)
	}
	b.ReportAllocs()
	var last controlplane.VerifiedCheckInCommit
	for b.Loop() {
		got, err := server.CommitCheckIn(request)
		if err != nil {
			b.Fatalf("CommitCheckIn() error = %v, want nil", err)
		}
		last = got
	}
	got, err := last.Disposition()
	if err != nil || got != controlplane.UsageDispositionAccepted {
		b.Fatalf("Disposition() = (%v, %v), want accepted and nil", got, err)
	}
}

func BenchmarkAuthenticatedResponse(b *testing.B) {
	b.ReportAllocs()
	fixture := authenticatedResponseForTest(b, 30)
	document := decodeAuthenticatedResponse(b, fixture.canonical)
	verification := controlplane.ResponseVerification[controlplane.RegistrationDocument, *controlplane.RegistrationDocument]{Client: fixture.client, Document: document, Expected: fixture.expected}
	proof, err := controlplane.VerifyResponse(verification)
	if err != nil {
		b.Fatalf("VerifyResponse(fixture) error = %v, want nil", err)
	}
	b.Run("verify", func(b *testing.B) {
		b.ReportAllocs()
		var last controlplane.VerifiedResponse[controlplane.RegistrationDocument, *controlplane.RegistrationDocument]
		for b.Loop() {
			got, err := controlplane.VerifyResponse(verification)
			if err != nil {
				b.Fatalf("VerifyResponse() error = %v, want nil", err)
			}
			last = got
		}
		got, err := last.Header()
		if err != nil || got != fixture.header {
			b.Fatalf("Header() = (%+v, %v), want %+v", got, err, fixture.header)
		}
	})
	b.Run("body", func(b *testing.B) {
		b.ReportAllocs()
		var last controlplane.RegistrationDocument
		for b.Loop() {
			got, err := proof.Body()
			if err != nil {
				b.Fatalf("Body() error = %v, want nil", err)
			}
			last = got
		}
		got, err := last.MarshalJSON()
		want, wantErr := fixture.body.MarshalJSON()
		if err != nil || wantErr != nil || !bytes.Equal(got, want) {
			b.Fatalf("Body() projection = (%d bytes, %v), want (%d bytes, %v)", len(got), err, len(want), wantErr)
		}
	})
}

func BenchmarkAdvanceUsageWatermark(b *testing.B) {
	b.ReportAllocs()
	for _, tc := range []struct {
		name   string
		window controlplane.UsageWindow
	}{
		{name: "one_class", window: testCheckInWindow()},
		{name: "maximum_classes", window: testWindow(fullUnitLadder(), fullOutcomeLadder())},
	} {
		b.Run(tc.name, func(b *testing.B) {
			issued := issueTestCheckIn(b, controlplaneOffering(b, 3), tc.window)
			current := issued.request.Payload.PreviousWatermark
			want, err := controlplane.AdvanceUsageWatermark(current, tc.window)
			if err != nil || want == current {
				b.Fatalf("watermark fixture = (%v, %v), want advancement", want, err)
			}
			b.ReportAllocs()
			var last controlplane.UsageWatermark
			for b.Loop() {
				got, err := controlplane.AdvanceUsageWatermark(current, tc.window)
				if err != nil {
					b.Fatalf("AdvanceUsageWatermark() error = %v, want nil", err)
				}
				last = got
			}
			if last != want {
				b.Fatalf("AdvanceUsageWatermark() = %v, want %v", last, want)
			}
		})
	}
}
