package chit

import (
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func BenchmarkManifestAccumulatorStreaming128(b *testing.B) {
	b.ReportAllocs()
	benchmarkManifestAccumulatorStreaming(b, 128)
}

func BenchmarkManifestAccumulatorStreaming4096(b *testing.B) {
	b.ReportAllocs()
	benchmarkManifestAccumulatorStreaming(b, 4096)
}

func benchmarkManifestAccumulatorStreaming(b *testing.B, objects uint64) {
	b.Helper()

	fixture := newChitFixture(b, 0x21, 1)
	addition := fixture.addition
	var canonicalBytes int64
	for sequence := uint64(1); sequence <= objects; sequence++ {
		entrySequence, err := NewEntrySequence(sequence)
		if err != nil {
			b.Fatalf("NewEntrySequence(%d) error = %v, want nil", sequence, err)
		}
		addition.Entry.Sequence = entrySequence
		encoded, err := core.MarshalCanonicalJSONDocument(addition.Entry)
		if err != nil {
			b.Fatalf("core.MarshalCanonicalJSONDocument(entry %d) error = %v, want nil", sequence, err)
		}
		canonicalBytes += int64(len(encoded) + 1)
	}
	b.ReportAllocs()
	b.SetBytes(canonicalBytes)
	b.ResetTimer()
	for range b.N {
		accumulator := NewManifestAccumulator()
		for sequence := uint64(1); sequence <= objects; sequence++ {
			entrySequence, err := NewEntrySequence(sequence)
			if err != nil {
				b.Fatalf("NewEntrySequence(%d) error = %v, want nil", sequence, err)
			}
			addition.Entry.Sequence = entrySequence
			if err := accumulator.Add(addition); err != nil {
				b.Fatalf("ManifestAccumulator.Add(entry %d) error = %v, want nil", sequence, err)
			}
		}
		summary, err := accumulator.Seal()
		if err != nil || summary.Objects.Uint64() != objects {
			b.Fatalf("ManifestAccumulator.Seal() = (%v, %v), want %d objects", summary, err, objects)
		}
	}
}

func BenchmarkVerifyCatalogPageOne(b *testing.B) {
	b.ReportAllocs()
	benchmarkVerifyCatalogPage(b, 1)
}

func BenchmarkVerifyCatalogPageMaximum(b *testing.B) {
	b.ReportAllocs()
	benchmarkVerifyCatalogPage(b, core.CatalogPageMaximumEntries)
}

func benchmarkVerifyCatalogPage(b *testing.B, entries int) {
	b.Helper()

	fixture := newCatalogFixture(b, 0x31, 1)
	pageEntries := catalogHistoryEntries(b, fixture, entries)
	request := fixture.request
	request.Query.Limit = catalogPageLimitFixture(b, uint16(entries))
	commitment, err := CommitQuery(request)
	if err != nil {
		b.Fatalf("CommitQuery() error = %v, want nil", err)
	}
	payload := fixture.payload
	payload.Entries = pageEntries
	payload.Request = commitment
	payload.Continuation = End()
	document, err := IssueCatalog(CatalogIssuance{Signer: fixture.private, Payload: payload})
	if err != nil {
		b.Fatalf("IssueCatalog() error = %v, want nil", err)
	}
	canonical, err := document.MarshalJSON()
	if err != nil {
		b.Fatalf("CatalogDocument.MarshalJSON() error = %v, want nil", err)
	}
	verification := CatalogVerification{
		Document: document, Request: request, TrustedKeys: fixture.trusted,
	}
	b.ReportAllocs()
	b.SetBytes(int64(len(canonical)))
	b.ResetTimer()
	for range b.N {
		got, err := VerifyCatalog(verification)
		if err != nil || len(got.Entries) != entries {
			b.Fatalf("VerifyCatalog() = (%d entries, %v), want %d entries and nil", len(got.Entries), err, entries)
		}
	}
}
