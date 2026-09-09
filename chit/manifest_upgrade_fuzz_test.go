package chit

import (
	"crypto/sha256"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzManifestStreamingRefusalAndMembership(f *testing.F) {
	fixture := newChitFixture(f, 0x23, 1)
	foreign := newChitFixture(f, 0x63, 2)
	for _, count := range []uint8{1, 2, 3, 31, 32, 63, 64} {
		for _, selected := range []uint8{1, count, count + 1} {
			f.Add(count, selected, uint8(0))
			f.Add(count, selected, uint8(1))
		}
	}
	f.Fuzz(func(t *testing.T, rawCount, rawSelected, mutation uint8) {
		count := uint64(rawCount%64) + 1
		selected := uint64(rawSelected%66) + 1
		verifier, err := NewManifestEntryVerifier(mustEntrySequence(t, selected))
		if err != nil {
			t.Fatal(err)
		}
		reference := sha256.New()
		if _, err := reference.Write([]byte(manifestFrameDomain + "\x00")); err != nil {
			t.Fatal(err)
		}
		var wantAddition ManifestAddition
		for sequence := uint64(1); sequence <= count; sequence++ {
			addition := fixture.addition
			addition.Entry.Sequence = mustEntrySequence(t, sequence)
			hostile := addition
			var wantErr error
			if mutation%2 == 0 {
				hostile.Entry.Sequence = mustEntrySequence(t, sequence+1)
				wantErr = core.ErrChitConflict
			} else {
				hostile.Evidence = foreign.addition.Evidence
				wantErr = core.ErrChitConflict
			}
			if hostile == addition {
				t.Fatalf("hostile addition = %+v, want distinct from %+v", hostile, addition)
			}
			if err := verifier.Add(hostile); !errors.Is(err, wantErr) {
				t.Fatalf("hostile Add = %v, want %v", err, wantErr)
			}
			if err := verifier.Add(addition); err != nil {
				t.Fatalf("valid Add after refusal = %v", err)
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
			if sequence == selected {
				wantAddition = addition
			}
		}
		var raw [sha256.Size]byte
		reference.Sum(raw[:0])
		digest, err := newManifestDigest(core.NewSHA256Digest(raw))
		if err != nil {
			t.Fatal(err)
		}
		objects, err := NewObjectCount(count)
		if err != nil {
			t.Fatal(err)
		}
		want := ManifestSummary{Digest: digest, Objects: objects, TotalBytes: mustChitByteLength(t, count*fixture.addition.Entry.Evidence.Payload.Body.Extent.Uint64())}
		proof, err := verifier.Seal(want)
		if selected > count {
			if !errors.Is(err, core.ErrChitConflict) || proof != (VerifiedManifestEntry{}) {
				t.Fatalf("absent membership = %v, %v", proof, err)
			}
		} else {
			got, additionErr := proof.Addition()
			summary, summaryErr := proof.Summary()
			if err != nil || additionErr != nil || summaryErr != nil || got != wantAddition || summary != want {
				t.Fatalf("membership lost exact entry or independent hash: %v", errors.Join(err, additionErr, summaryErr))
			}
		}
		repeated, repeatedErr := verifier.Seal(want)
		if !errors.Is(repeatedErr, core.ErrChitContract) || repeated != (VerifiedManifestEntry{}) {
			t.Fatalf("repeated seal = %v, %v", repeated, repeatedErr)
		}
	})
}
