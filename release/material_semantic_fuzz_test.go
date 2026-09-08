package release

import (
	"bytes"
	"crypto/ed25519"
	"encoding/base64"
	json "encoding/json/v2"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzReleaseMaterialResponseExternalSemanticOracle(f *testing.F) {
	fixture := newReleaseFixture(f, core.NewReleaseVersion(2026, 1, 1), 1)
	_, _, response := materialFixturesForFuzz(f, fixture)
	if err := response.Validate(); err != nil {
		f.Fatalf("MaterialResponse.Validate(seed) error = %v, want nil", err)
	}
	canonical, err := response.MarshalJSON()
	if err != nil {
		f.Fatalf("MaterialResponse.MarshalJSON(seed) error = %v, want nil", err)
	}
	f.Add(canonical)
	f.Add(canonical[:len(canonical)-1])
	f.Add(append(bytes.Clone(canonical), ' '))
	f.Fuzz(func(t *testing.T, data []byte) {
		var got MaterialResponse
		gotErr := got.UnmarshalJSON(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrReleaseContract) || !errors.Is(gotErr, core.ErrJSONContract) || got != (MaterialResponse{}) {
				t.Fatalf("MaterialResponse.UnmarshalJSON(refused) = (%v, %v), want typed refusal and zero", got, gotErr)
			}
			return
		}
		defer func() {
			if err := got.Destroy(); err != nil {
				t.Errorf("MaterialResponse.Destroy() error = %v, want nil", err)
			}
		}()
		if len(data) > documentExtentMaximum {
			t.Fatalf("admitted material document = %d bytes, want <= %d", len(data), documentExtentMaximum)
		}
		var wire materialResponseWire
		if err := json.Unmarshal(data, &wire); err != nil || wire.ReleaseSigningSeed == nil || wire.Request == nil || wire.ServerPublicKey == nil {
			t.Fatalf("Go Unmarshal(admitted material) error = %v, want complete input facts", err)
		}
		decoded, err := base64.StdEncoding.DecodeString(*wire.ReleaseSigningSeed)
		defer clear(decoded)
		if err != nil || len(decoded) != ed25519.SeedSize || base64.StdEncoding.EncodeToString(decoded) != *wire.ReleaseSigningSeed {
			t.Fatalf("admitted seed representation = (%d decoded bytes, %v), want canonical Go seed", len(decoded), err)
		}
		if got.Request != *wire.Request || got.ServerPublicKey != *wire.ServerPublicKey || got.Validate() != nil {
			t.Fatalf("admitted material = %v, want exact request and server identity from input", got)
		}
		encoded, err := got.MarshalJSON()
		if err != nil || len(encoded) > documentExtentMaximum {
			t.Fatalf("MaterialResponse.MarshalJSON() = (%d bytes, %v), want bounded and nil", len(encoded), err)
		}
		var roundTrip MaterialResponse
		if err := roundTrip.UnmarshalJSON(encoded); err != nil {
			t.Fatalf("MaterialResponse.UnmarshalJSON(canonical) error = %v, want nil", err)
		}
		defer func() {
			if err := roundTrip.Destroy(); err != nil {
				t.Errorf("MaterialResponse.Destroy(round trip) error = %v, want nil", err)
			}
		}()
		second, err := roundTrip.MarshalJSON()
		if err != nil || !bytes.Equal(second, encoded) {
			t.Fatalf("second material projection = (%d bytes, %v), want byte-identical canonical document", len(second), err)
		}
		opened, err := got.Open()
		if err != nil || got != (MaterialResponse{}) || opened.ServerPublicKey != *wire.ServerPublicKey {
			t.Fatalf("MaterialResponse.Open() = (%v, %v), want consumed source and exact server identity", opened, err)
		}
		defer func() {
			if err := opened.Destroy(); err != nil {
				t.Errorf("Material.Destroy() error = %v, want nil", err)
			}
		}()
		private, err := opened.SigningKey.PrivateKey()
		defer clear(private)
		want := ed25519.NewKeyFromSeed(decoded)
		defer clear(want)
		if err != nil || !bytes.Equal(private, want) {
			t.Fatalf("opened signing key = (%d bytes, %v), want exact Go derivation from input", len(private), err)
		}
		if err := opened.Destroy(); err != nil {
			t.Fatalf("Material.Destroy() error = %v, want nil", err)
		}
		if err := opened.Validate(); !errors.Is(err, core.ErrReleaseContract) {
			t.Fatalf("destroyed material Validate() error = %v, want Release refusal", err)
		}
	})
}
