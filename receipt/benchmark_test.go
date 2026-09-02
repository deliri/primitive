package receipt

import (
	"runtime"
	"testing"
)

func BenchmarkVerifyEvidence(b *testing.B) {
	fixture := newReceiptFixture(b, 160)
	request := VerifyEvidenceRequest{
		Document:    issueFixture(b, fixture),
		TrustedKeys: fixture.trusted,
		Expected:    fixture.expectation,
	}
	b.ReportAllocs()

	var result VerifiedEvidence
	var err error
	for b.Loop() {
		result, err = VerifyEvidence(request)
	}
	runtime.KeepAlive(result)
	runtime.KeepAlive(err)
}

func BenchmarkAdvanceWatermark(b *testing.B) {
	fixture := newReceiptFixture(b, 170)
	scope := Scope{Principal: fixture.principal, Offering: fixture.offering}
	request := AdvanceWatermarkRequest{
		Current:   watermarkFixture(b, scope, 1, "benchmark-current"),
		Candidate: watermarkFixture(b, scope, 2, "benchmark-candidate"),
	}
	b.ReportAllocs()

	var result AdvanceResult
	var err error
	for b.Loop() {
		result, err = AdvanceWatermark(request)
	}
	runtime.KeepAlive(result)
	runtime.KeepAlive(err)
}
