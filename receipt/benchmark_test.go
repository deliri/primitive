package receipt

import (
	"io"
	"testing"
)

func BenchmarkVerifyEvidence(b *testing.B) {
	fixture := newReceiptFixture(b, 160)
	request := VerifyEvidenceRequest{Document: issueFixture(b, fixture), TrustedKeys: fixture.trusted, Expected: fixture.expectation}
	b.ReportAllocs()
	var result VerifiedEvidence
	var err error
	for b.Loop() {
		result, err = VerifyEvidence(request)
	}
	got, projectionErr := result.Document()
	if err != nil || projectionErr != nil || got != request.Document {
		b.Fatalf("verified=%v/%v document=%v, want exact authenticated document", err, projectionErr, got)
	}
}
func BenchmarkAdvanceWatermark(b *testing.B) {
	fixture := newReceiptFixture(b, 170)
	scope := Scope{Principal: fixture.principal, Offering: fixture.offering}
	request := AdvanceWatermarkRequest{Current: watermarkFixture(b, scope, 1, "benchmark-current"), Candidate: watermarkFixture(b, scope, 2, "benchmark-candidate")}
	b.ReportAllocs()
	var result AdvanceResult
	var err error
	for b.Loop() {
		result, err = AdvanceWatermark(request)
	}
	got, projectionErr := result.Watermark()
	state, stateErr := result.State()
	if err != nil || projectionErr != nil || stateErr != nil || state != AdvanceAccepted || got != request.Candidate {
		b.Fatalf("advance=%v/%v state=%v/%v selected=%v, want accepted exact candidate", err, projectionErr, state, stateErr, got)
	}
}

type receiptBenchmarkSink struct{ bytes int }

func (w *receiptBenchmarkSink) Write(p []byte) (int, error) { w.bytes += len(p); return len(p), nil }

func BenchmarkReceiptBoundary(b *testing.B) {
	b.ReportAllocs()
	fixture := newReceiptFixture(b, 180)
	document := issueFixture(b, fixture)
	scope := Scope{Principal: fixture.principal, Offering: fixture.offering}
	watermark := watermarkFixture(b, scope, 1, "benchmark-json")
	encoded, err := document.MarshalJSON()
	if err != nil {
		b.Fatal(err)
	}
	scopeJSON, err := scope.MarshalJSON()
	if err != nil {
		b.Fatal(err)
	}
	watermarkJSON, err := watermark.MarshalJSON()
	if err != nil {
		b.Fatal(err)
	}
	payloadJSON, err := document.Payload.MarshalJSON()
	if err != nil {
		b.Fatal(err)
	}
	for _, tc := range []struct {
		name  string
		bytes int
		run   func(*testing.B)
	}{
		{name: "issue", run: func(b *testing.B) {
			request := IssueEvidenceRequest{Offering: fixture.offering, Key: fixture.private, Body: fixture.body, OccurredAt: fixture.occurredAt, Identity: fixture.receipt, Principal: fixture.principal}
			var got EvidenceDocument
			var err error
			for b.Loop() {
				got, err = IssueEvidence(request)
			}
			if err != nil || got != document {
				b.Fatalf("issue=%v/%v, want exact signed document", got, err)
			}
		}},
		{name: "encode_document", bytes: len(encoded), run: func(b *testing.B) {
			var got []byte
			var err error
			for b.Loop() {
				got, err = document.MarshalJSON()
			}
			if err != nil || string(got) != string(encoded) {
				b.Fatalf("encoded=%d/%v, want exact %d bytes", len(got), err, len(encoded))
			}
		}},
		{name: "decode_document", bytes: len(encoded), run: func(b *testing.B) {
			var got EvidenceDocument
			var err error
			for b.Loop() {
				err = got.UnmarshalJSON(encoded)
			}
			if err != nil || got != document {
				b.Fatalf("decoded=%v/%v, want exact document", got, err)
			}
		}},
		{name: "write_canonical", bytes: len(payloadJSON), run: func(b *testing.B) {
			var sink receiptBenchmarkSink
			var err error
			for b.Loop() {
				sink.bytes = 0
				err = document.Payload.WriteCanonical(&sink)
			}
			if err != nil || sink.bytes != len(payloadJSON) {
				b.Fatalf("written=%d/%v, want %d", sink.bytes, err, len(payloadJSON))
			}
		}},
		{name: "decode_scope", bytes: len(scopeJSON), run: func(b *testing.B) {
			var got Scope
			var err error
			for b.Loop() {
				err = got.UnmarshalJSON(scopeJSON)
			}
			if err != nil || got != scope {
				b.Fatalf("scope=%v/%v, want exact source", got, err)
			}
		}},
		{name: "decode_watermark", bytes: len(watermarkJSON), run: func(b *testing.B) {
			var got Watermark
			var err error
			for b.Loop() {
				err = got.UnmarshalJSON(watermarkJSON)
			}
			if err != nil || got != watermark {
				b.Fatalf("watermark=%v/%v, want exact source", got, err)
			}
		}},
	} {
		b.Run(tc.name, func(b *testing.B) {
			b.ReportAllocs()
			if tc.bytes > 0 {
				b.SetBytes(int64(tc.bytes))
			}
			tc.run(b)
		})
	}
}

var _ io.Writer = (*receiptBenchmarkSink)(nil)
