package chit

import (
	"crypto/sha256"
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestManifestAuthenticatedExtentTable(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                    string
		extents                 []uint64
		wantObjects, wantBytes  uint64
		wantAddErr, wantSealErr error
	}{
		{name: "no receipts cannot invent a manifest", wantSealErr: core.ErrChitContract},
		{name: "one authenticated empty object", extents: []uint64{0}, wantObjects: 1},
		{name: "two authenticated empty objects", extents: []uint64{0, 0}, wantObjects: 2},
		{name: "empty object precedes one byte", extents: []uint64{0, 1}, wantObjects: 2, wantBytes: 1},
		{name: "empty object follows one byte", extents: []uint64{1, 0}, wantObjects: 2, wantBytes: 1},
		{name: "empty object between nonempty objects", extents: []uint64{1, 0, 1}, wantObjects: 3, wantBytes: 2},
		{name: "one below signed extent ceiling", extents: []uint64{math.MaxInt64 - 1}, wantObjects: 1, wantBytes: math.MaxInt64 - 1},
		{name: "exact signed extent ceiling", extents: []uint64{math.MaxInt64}, wantObjects: 1, wantBytes: math.MaxInt64},
		{name: "sum reaches exact ceiling", extents: []uint64{math.MaxInt64 - 1, 1}, wantObjects: 2, wantBytes: math.MaxInt64},
		{name: "empty suffix at saturated total", extents: []uint64{math.MaxInt64, 0}, wantObjects: 2, wantBytes: math.MaxInt64},
		{name: "empty prefix before saturated total", extents: []uint64{0, math.MaxInt64}, wantObjects: 2, wantBytes: math.MaxInt64},
		{name: "one byte overflow preserves saturated prefix", extents: []uint64{math.MaxInt64, 1}, wantObjects: 1, wantBytes: math.MaxInt64, wantAddErr: core.ErrNumericOverflow},
		{name: "two byte addition crosses ceiling", extents: []uint64{math.MaxInt64 - 1, 2}, wantObjects: 1, wantBytes: math.MaxInt64 - 1, wantAddErr: core.ErrNumericOverflow},
		{name: "large second extent cannot wrap prefix", extents: []uint64{1, math.MaxInt64}, wantObjects: 1, wantBytes: 1, wantAddErr: core.ErrNumericOverflow},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fixture := newChitFixture(t, 0x31, 1)
			accumulator := NewManifestAccumulator()
			reference := sha256.New()
			if _, err := reference.Write([]byte(manifestFrameDomain + "\x00")); err != nil {
				t.Fatal(err)
			}
			for index, extent := range tc.extents {
				length := mustChitByteLength(t, extent)
				addition := chitEvidenceEntryFixture(t, chitEntryFixtureRequest{Private: fixture.private, Trusted: fixture.trusted, Scope: fixture.scope, Sequence: uint64(index + 1), Marker: byte(index + 0x51), Name: chitFixtureNameA, Extent: &length})
				err := accumulator.Add(addition)
				var wantErr error
				if index == len(tc.extents)-1 {
					wantErr = tc.wantAddErr
				}
				if !errors.Is(err, wantErr) {
					t.Fatalf("Add(%d) = %v, want %v", index, err, wantErr)
				}
				if err != nil {
					continue
				}
				encoded, err := core.MarshalCanonicalJSONDocument(addition.Entry)
				if err != nil {
					t.Fatal(err)
				}
				if _, err := reference.Write(encoded); err != nil {
					t.Fatal(err)
				}
				if _, err := reference.Write([]byte{0}); err != nil {
					t.Fatal(err)
				}
			}
			summary, err := accumulator.Seal()
			if tc.wantSealErr != nil {
				if !errors.Is(err, tc.wantSealErr) || summary != (ManifestSummary{}) {
					t.Fatalf("Seal() = %v, %v; want zero and %v", summary, err, tc.wantSealErr)
				}
				return
			}
			var raw [sha256.Size]byte
			reference.Sum(raw[:0])
			digest, digestErr := newManifestDigest(core.NewSHA256Digest(raw))
			if err != nil || digestErr != nil || summary.Objects.Uint64() != tc.wantObjects || summary.TotalBytes.Uint64() != tc.wantBytes || summary.Digest != digest {
				t.Fatalf("Seal() = %+v, %v; want objects %d, bytes %d, independent digest %v (%v)", summary, err, tc.wantObjects, tc.wantBytes, digest, digestErr)
			}
			payload := fixture.document.Payload
			payload.Manifest = summary
			document, issueErr := Issue(Issuance{Signer: fixture.private, TrustedKeys: fixture.trusted, Payload: payload})
			proof, verifyErr := Verify(Verification{Document: document, TrustedKeys: fixture.trusted, Expected: Expectation{Identity: payload.Identity, Scope: payload.Scope}})
			observed, observeErr := proof.Document()
			if issueErr != nil || verifyErr != nil || observeErr != nil || observed.Payload != payload {
				t.Fatalf("manifest issuance/verification = %v, %v, %v; want exact signed payload", issueErr, verifyErr, observeErr)
			}

		})
	}
}
