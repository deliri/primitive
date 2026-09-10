package payment

import (
	"github.com/deliri/primitive/v2026/core"
	"testing"
)

func BenchmarkPaymentCatalogIssue(b *testing.B) {
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
			request := CatalogIssuance{Signer: fixture.private, Payload: fixture.payload}
			var got CatalogDocument
			b.ReportAllocs()
			for b.Loop() {
				var err error
				got, err = IssueCatalog(request)
				if err != nil {
					b.Fatal(err)
				}
			}
			if !samePaymentCatalogDocument(got, fixture.document) {
				b.Fatalf("issued document = %v, want %v", got, fixture.document)
			}
		})
	}
}
