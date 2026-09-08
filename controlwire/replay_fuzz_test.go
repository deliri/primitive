package controlwire

import (
	"bytes"
	"encoding/json/jsontext"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzReplayIdentityExternalJSONSemanticClosure(f *testing.F) {
	fixtures := controlwireFixturesForFuzz(f)
	defer func() { _ = fixtures.token.Destroy() }()
	seed := fixtures.replayIdentity
	canonical, err := seed.MarshalJSON()
	if err != nil {
		f.Fatalf("replay seed error=%v, want nil", err)
	}
	f.Add(canonical)
	f.Add(append([]byte{' '}, canonical...))
	for _, bad := range [][]byte{nil, []byte(`null`), []byte(`{}`), []byte(`[]`), canonical[:len(canonical)-1], append(bytes.Clone(canonical), canonical...)} {
		f.Add(bad)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		wantAccept := controlwireJSONReferenceAccepts(controlwireJSONDoorReplayIdentity, data)
		got := seed
		err := got.UnmarshalJSON(data)
		if (err == nil) != wantAccept {
			t.Fatalf("replay decode error=%v, want acceptance=%v", err, wantAccept)
		}
		if err != nil {
			if !errors.Is(err, core.ErrControlWireContract) || !errors.Is(err, core.ErrJSONContract) || got != seed {
				t.Fatalf("replay=%v/%v, want preserved %v and typed refusal", got, err, seed)
			}
			return
		}
		canonical, err := got.MarshalJSON()
		if err != nil || got.Validate() != nil || len(canonical) > ReplayIdentityJSONMaximumBytes {
			t.Fatalf("replay output=%d bytes/%v, want valid and bounded", len(canonical), err)
		}
		wantFacts := jsontext.Value(bytes.Clone(data))
		gotFacts := jsontext.Value(bytes.Clone(canonical))
		wantErr := wantFacts.Canonicalize(jsontext.CanonicalizeRawInts(false))
		gotErr := gotFacts.Canonicalize(jsontext.CanonicalizeRawInts(false))
		if wantErr != nil || gotErr != nil || !bytes.Equal(gotFacts, wantFacts) {
			t.Fatalf("replay facts=%q/%v, want %q/%v", gotFacts, gotErr, wantFacts, wantErr)
		}
		var round ReplayIdentity
		roundErr := round.UnmarshalJSON(canonical)
		second, secondErr := round.MarshalJSON()
		if roundErr != nil || secondErr != nil || round != got || !bytes.Equal(second, canonical) {
			t.Fatalf("replay round trip=%v/%v/%v, want %v and canonical fixed point", round, roundErr, secondErr, got)
		}
	})
}
