package release

import (
	"bytes"
	"encoding/base64"
	json "encoding/json/v2"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/keygen"
)

// Contract ratchet: a malformed sibling or seed must leave the receiver's live
// custody and bytes intact. Accepted replacement consumes the previous custody
// even when its bytes are unchanged. Every row decodes into separately owned
// custody, so a defect in one parallel case cannot destroy a sibling fixture.
func TestMaterialResponseLayerTriadPreservesOrReplacesExactCustody(t *testing.T) {
	t.Parallel()
	fixture := newReleaseFixture(t, core.NewReleaseVersion(2026, 1, 1), 1)
	_, _, response := materialFixturesForFuzz(t, fixture)
	canonical, err := response.MarshalJSON()
	if err != nil {
		t.Fatalf("MaterialResponse.MarshalJSON() error = %v, want nil", err)
	}
	var base materialResponseWire
	if err := json.Unmarshal(canonical, &base); err != nil {
		t.Fatalf("Go Unmarshal(fixture wire) error = %v, want nil", err)
	}
	seedDocument := func(seed []byte) []byte {
		wire := base
		encoded := base64.StdEncoding.EncodeToString(seed)
		wire.ReleaseSigningSeed = &encoded
		return materialBoundaryJSON(t, wire)
	}
	missingSeed, missingRequest, missingServer := base, base, base
	missingSeed.ReleaseSigningSeed = nil
	missingRequest.Request = nil
	missingServer.ServerPublicKey = nil
	unpadded := base
	unpaddedSeed := (*base.ReleaseSigningSeed)[:base64.StdEncoding.EncodedLen(keygen.SeedSize)-1]
	unpadded.ReleaseSigningSeed = &unpaddedSeed
	for _, tc := range []struct {
		name   string
		input  []byte
		accept bool
	}{
		{name: "equivalent replacement consumes old custody without changing facts", input: canonical, accept: true},
		{name: "changed seed replaces exact secret bytes", input: seedDocument(bytes.Repeat([]byte{0x53}, keygen.SeedSize)), accept: true},
		{name: "empty input cannot consume existing custody"},
		{name: "truncated enclosing object cannot consume existing custody", input: canonical[:len(canonical)-1]},
		{name: "second document cannot become an accepted first document", input: append(bytes.Clone(canonical), canonical...)},
		{name: "unknown sibling cannot be ignored", input: append(bytes.Clone(canonical[:len(canonical)-1]), []byte(`,"future":1}`)...)},
		{name: "null seed cannot hide behind an unrelated unknown field", input: materialBoundaryJSON(t, missingSeed)},
		{name: "null request cannot allocate accepted seed custody", input: materialBoundaryJSON(t, missingRequest)},
		{name: "null server cannot allocate accepted seed custody", input: materialBoundaryJSON(t, missingServer)},
		{name: "omitted padding cannot be silently repaired", input: materialBoundaryJSON(t, unpadded)},
		{name: "short seed cannot be padded into a signing identity", input: seedDocument(bytes.Repeat([]byte{0x53}, keygen.SeedSize-1))},
		{name: "long seed cannot be truncated into a signing identity", input: seedDocument(bytes.Repeat([]byte{0x53}, keygen.SeedSize+1))},
		{name: "zero seed cannot become usable custody", input: seedDocument(make([]byte, keygen.SeedSize))},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var got MaterialResponse
			if err := got.UnmarshalJSON(canonical); err != nil {
				t.Fatalf("UnmarshalJSON(owned fixture) error = %v, want nil", err)
			}
			defer func() {
				if err := got.Destroy(); err != nil {
					t.Errorf("MaterialResponse.Destroy() error = %v, want nil", err)
				}
			}()
			previous := got
			err := got.UnmarshalJSON(tc.input)
			want := canonical
			if tc.accept {
				want = tc.input
				if err != nil || !errors.Is(previous.Validate(), core.ErrReleaseContract) {
					t.Fatalf("replacement errors = (decode %v, previous custody %v), want (nil, Release refusal)", err, previous.Validate())
				}
			} else if !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, core.ErrReleaseContract) || got != previous || previous.Validate() != nil {
				t.Fatalf("refusal = (error %v, same receiver %t, previous custody %v), want JSON/Release refusal and unchanged valid custody", err, got == previous, previous.Validate())
			}
			encoded, err := got.MarshalJSON()
			if err != nil || !bytes.Equal(encoded, want) {
				t.Fatalf("receiver projection = (%d bytes, %v, exact %t), want %d exact admitted bytes", len(encoded), err, bytes.Equal(encoded, want), len(want))
			}
		})
	}
}

func materialBoundaryJSON(t testing.TB, wire materialResponseWire) []byte {
	t.Helper()
	document, err := json.Marshal(wire)
	if err != nil {
		t.Fatalf("Go Marshal(material wire) error = %v, want nil", err)
	}
	return document
}
