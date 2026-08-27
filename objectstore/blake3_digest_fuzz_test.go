package objectstore_test

import (
	"bytes"
	json "encoding/json/v2"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/objectstore"
)

func FuzzBLAKE3DigestTextSemanticClosure(f *testing.F) {
	seed := objectstore.NewBLAKE3Digest([objectstore.BLAKE3DigestBytes]byte{1})
	canonical, err := seed.Hex()
	if err != nil {
		f.Fatalf("BLAKE3Digest.Hex(seed) error = %v, want nil", err)
	}
	f.Add(canonical)
	for _, hostile := range []string{"", "0", canonical[:len(canonical)-1], canonical + "0", "A" + canonical[1:], "\x00"} {
		f.Add(hostile)
	}
	f.Fuzz(func(t *testing.T, value string) {
		got := seed
		gotErr := got.UnmarshalText([]byte(value))
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrObjectStoreContract) || !errors.Is(gotErr, core.ErrPrimitiveContract) || got != seed {
				t.Fatalf("BLAKE3Digest.UnmarshalText(rejected) = (%v, %v), want preserved with typed contract identities", got, gotErr)
			}
			return
		}
		if got.Validate() != nil {
			t.Fatalf("BLAKE3Digest.UnmarshalText(accepted).Validate() error = %v, want nil", got.Validate())
		}
		projected, projectionErr := got.Hex()
		if projectionErr != nil || projected != value {
			t.Fatalf("BLAKE3Digest.Hex(accepted) = (%q, %v), want (%q, nil)", projected, projectionErr, value)
		}
		var roundTrip objectstore.BLAKE3Digest
		roundTripErr := roundTrip.UnmarshalText([]byte(projected))
		if roundTripErr != nil || roundTrip != got {
			t.Fatalf("BLAKE3 text fixed point = (%v, %v), want (%v, nil)", roundTrip, roundTripErr, got)
		}
	})
}

func FuzzBLAKE3DigestJSONSemanticClosure(f *testing.F) {
	seed := objectstore.NewBLAKE3Digest([objectstore.BLAKE3DigestBytes]byte{1})
	canonical, err := seed.MarshalJSON()
	if err != nil {
		f.Fatalf("BLAKE3Digest.MarshalJSON(seed) error = %v, want nil", err)
	}
	f.Add(canonical)
	for _, hostile := range [][]byte{nil, {}, []byte(`null`), []byte(`0`), []byte(`{}`), []byte(`"0"`), append(bytes.Clone(canonical), '0')} {
		f.Add(hostile)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		got := seed
		gotErr := got.UnmarshalJSON(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrJSONContract) || !errors.Is(gotErr, core.ErrObjectStoreContract) || got != seed {
				t.Fatalf("BLAKE3Digest.UnmarshalJSON(rejected) = (%v, %v), want preserved with typed contract identities", got, gotErr)
			}
			return
		}
		if got.Validate() != nil {
			t.Fatalf("BLAKE3Digest.UnmarshalJSON(accepted).Validate() error = %v, want nil", got.Validate())
		}
		projected, projectionErr := got.MarshalJSON()
		if projectionErr != nil || len(projected) > core.JSONDocumentMaximumBytes {
			t.Fatalf("BLAKE3Digest.MarshalJSON(accepted) = (%d bytes, %v), want bounded and nil", len(projected), projectionErr)
		}
		var roundTrip objectstore.BLAKE3Digest
		roundTripErr := json.Unmarshal(projected, &roundTrip)
		second, secondErr := roundTrip.MarshalJSON()
		if roundTripErr != nil || roundTrip != got || secondErr != nil || !bytes.Equal(second, projected) {
			t.Fatalf("BLAKE3 JSON fixed point = (%v, decode %v, second %q/%v), want (%v, nil, %q/nil)", roundTrip, roundTripErr, second, secondErr, got, projected)
		}
	})
}
