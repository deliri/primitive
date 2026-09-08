package attest

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzTrustedKeysExternalAdmission(f *testing.F) {
	seed := make([]byte, 0, (TrustedKeyMaximumCount+1)*ed25519.PublicKeySize)
	for index := range TrustedKeyMaximumCount + 1 {
		key := internalPublicKeyFixture(f, string(rune('a'+index)))
		raw, err := key.Bytes()
		if err != nil {
			f.Fatalf("public seed bytes error = %v, want nil", err)
		}
		seed = append(seed, raw...)
		f.Add(bytes.Clone(seed))
	}
	f.Add([]byte{})
	f.Add(make([]byte, ed25519.PublicKeySize))
	f.Add(seed[:ed25519.PublicKeySize-1])
	f.Add(append(bytes.Clone(seed[:ed25519.PublicKeySize]), seed[:ed25519.PublicKeySize]...))
	f.Fuzz(func(t *testing.T, raw []byte) {
		// One extra slot reaches the real count rejection. No allocation follows
		// the fuzz input's unbounded length; all oracle state is fixed storage.
		var storage [TrustedKeyMaximumCount + 1]core.Ed25519PublicKey
		var zeroKey [ed25519.PublicKeySize]byte
		count := min(len(raw)/ed25519.PublicKeySize, len(storage))
		if len(raw)%ed25519.PublicKeySize != 0 {
			count = min(count+1, len(storage))
		}
		wantAdmitted := count > 0 && len(raw) <= TrustedKeyMaximumCount*ed25519.PublicKeySize && len(raw)%ed25519.PublicKeySize == 0
		for index := range count {
			begin := index * ed25519.PublicKeySize
			key, err := core.NewEd25519PublicKey(raw[begin:min(begin+ed25519.PublicKeySize, len(raw))])
			storage[index] = key
			wantAdmitted = wantAdmitted && err == nil && !bytes.Equal(raw[begin:min(begin+ed25519.PublicKeySize, len(raw))], zeroKey[:]) && !slices.Contains(storage[:index], key)
		}
		original := storage
		request := TrustedKeysRequest{Keys: storage[:count]}
		shapeErr := request.Validate()
		got, gotErr := NewTrustedKeys(request)
		if !wantAdmitted {
			if !errors.Is(shapeErr, core.ErrAttestContract) || !errors.Is(gotErr, core.ErrAttestContract) || got != (TrustedKeys{}) {
				t.Fatalf("trust admission = (%v, %+v, %v), want typed refusal and zero trust", shapeErr, got, gotErr)
			}
			return
		}
		if shapeErr != nil || gotErr != nil {
			t.Fatalf("trust admission errors = (%v, %v), want nil", shapeErr, gotErr)
		}
		clear(storage[:])
		if err := got.Validate(); err != nil {
			t.Fatalf("trusted copy after source clear error = %v, want nil", err)
		}
		for _, key := range original[:count] {
			if !got.contains(key) {
				t.Fatalf("trusted copy contains %v = false, want true", key)
			}
		}
		if got.count != count {
			t.Fatalf("trusted count = %d, want %d", got.count, count)
		}
		slices.Reverse(original[:count])
		reversed, err := NewTrustedKeys(TrustedKeysRequest{Keys: original[:count]})
		if err != nil {
			t.Fatalf("reverse trust admission error = %v, want nil", err)
		}
		for _, key := range original[:count] {
			if !reversed.contains(key) {
				t.Fatalf("reversed trust contains %v = false, want true", key)
			}
		}
	})
}
