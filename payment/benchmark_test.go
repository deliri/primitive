package payment

import (
	"github.com/deliri/primitive/v2026/core"
	"testing"
)

func BenchmarkPaymentIssue(b *testing.B) {
	fixture := newPaymentFixture(b, paymentFixtureRequest{Marker: 0x41, Millisecond: 1, MinorUnits: 1})
	request := Issuance{Signer: fixture.private, Payload: fixture.document.Payload}
	b.ReportAllocs()
	for b.Loop() {
		got, err := Issue(request)
		if err != nil || got != fixture.document {
			b.Fatalf("Issue = (%v,%v), want exact signed document", got, err)
		}
	}
}

func BenchmarkPaymentVerify(b *testing.B) {
	fixture := newPaymentFixture(b, paymentFixtureRequest{Marker: 0x41, Millisecond: 1, MinorUnits: 1})
	request := Verification{Document: fixture.document, Expected: Expectation{Identity: fixture.identity, Scope: fixture.scope}, TrustedKeys: fixture.trusted}
	var got Verified
	b.ReportAllocs()
	for b.Loop() {
		var err error
		got, err = Verify(request)
		if err != nil {
			b.Fatal(err)
		}
	}
	document, err := got.Document()
	if err != nil || document != fixture.document {
		b.Fatalf("verified document = (%v,%v), want signed fixture", document, err)
	}
}

func BenchmarkPaymentCommitQuery(b *testing.B) {
	fixture := newSignedQueryFixture(b, signedQueryFixtureRequest{marker: 0x51, pageSize: 1})
	want, err := CommitQuery(fixture.payload)
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	for b.Loop() {
		got, err := CommitQuery(fixture.payload)
		if err != nil || got != want {
			b.Fatalf("CommitQuery = (%v,%v), want %v", got, err, want)
		}
	}
}

func BenchmarkPaymentCatalogVerify(b *testing.B) {
	b.ReportAllocs()
	for _, tc := range []struct {
		name    string
		entries uint16
	}{
		{name: "one", entries: 1},
		{name: "full_window", entries: core.CatalogPageMaximumEntries},
	} {
		b.Run(tc.name, func(b *testing.B) {
			fixture := newPaymentCatalogFixture(b, paymentCatalogFixtureRequest{Marker: 0x61, Entries: tc.entries})
			request := CatalogVerification{Document: fixture.document, Request: fixture.request, TrustedKeys: fixture.trusted}
			var got VerifiedCatalog
			b.ReportAllocs()
			for b.Loop() {
				var err error
				got, err = VerifyCatalog(request)
				if err != nil {
					b.Fatal(err)
				}
			}
			if !verifiedPaymentCatalogEqual(got, fixture.payload) {
				b.Fatalf("verified catalog = %v, want exact payload %v", got, fixture.payload)
			}
		})
	}
}

func BenchmarkPaymentCatalogDecode(b *testing.B) {
	fixture := newPaymentCatalogFixture(b, paymentCatalogFixtureRequest{Marker: 0x61, Entries: core.CatalogPageMaximumEntries})
	encoded, err := fixture.document.MarshalJSON()
	if err != nil || len(encoded) == 0 {
		b.Fatalf("catalog fixture = (%d bytes,%v)", len(encoded), err)
	}
	var got CatalogDocument
	b.ReportAllocs()
	b.SetBytes(int64(len(encoded)))
	for b.Loop() {
		if err := got.UnmarshalJSON(encoded); err != nil {
			b.Fatal(err)
		}
	}
	if !samePaymentCatalogDocument(got, fixture.document) {
		b.Fatalf("decoded catalog = %v, want %v", got, fixture.document)
	}
}

type paymentBenchmarkWriter struct{ bytes int }

func (w *paymentBenchmarkWriter) Write(p []byte) (int, error) { w.bytes += len(p); return len(p), nil }

func BenchmarkPaymentCatalogCanonical(b *testing.B) {
	fixture := newPaymentCatalogFixture(b, paymentCatalogFixtureRequest{Marker: 0x61, Entries: core.CatalogPageMaximumEntries})
	encoded, err := fixture.payload.MarshalJSON()
	if err != nil || len(encoded) == 0 {
		b.Fatalf("catalog fixture = (%d bytes,%v)", len(encoded), err)
	}
	sink := paymentBenchmarkWriter{}
	b.ReportAllocs()
	b.SetBytes(int64(len(encoded)))
	for b.Loop() {
		sink.bytes = 0
		if err := fixture.payload.WriteCanonical(&sink); err != nil {
			b.Fatal(err)
		}
		if sink.bytes != len(encoded) {
			b.Fatalf("written %d bytes, want %d", sink.bytes, len(encoded))
		}
	}
}
