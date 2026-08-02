package fuzzfinder

import (
	"encoding/json"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestArtifactKindExhaustsClosedWireDomain(t *testing.T) {
	t.Parallel()

	for raw := range 256 {
		kind := ArtifactKind(raw)
		gotErr := kind.Validate()
		wantValid := kind == ArtifactCorpus || kind == ArtifactCrasher
		if (gotErr == nil) != wantValid || kind.IsValid() != wantValid {
			t.Fatalf("ArtifactKind(%d) validity = Validate:%v IsValid:%t, want %t", raw, gotErr, kind.IsValid(), wantValid)
		}
		if !wantValid {
			if !errors.Is(gotErr, core.ErrFuzzFinderContract) {
				t.Fatalf("ArtifactKind(%d).Validate() error = %v, want %v", raw, gotErr, core.ErrFuzzFinderContract)
			}
			if kind.String() != core.UnknownEnumDiagnostic {
				t.Fatalf("ArtifactKind(%d).String() = %q, want %q", raw, kind.String(), core.UnknownEnumDiagnostic)
			}
			if _, marshalErr := json.Marshal(kind); !errors.Is(marshalErr, core.ErrFuzzFinderContract) {
				t.Fatalf("json.Marshal(ArtifactKind(%d)) error = %v, want %v", raw, marshalErr, core.ErrFuzzFinderContract)
			}
			continue
		}
		wire, gotErr := json.Marshal(kind)
		if gotErr != nil {
			t.Fatalf("json.Marshal(ArtifactKind(%d)) error = %v, want nil", raw, gotErr)
		}
		wantWire := strconv.Quote(kind.String())
		if string(wire) != wantWire {
			t.Fatalf("json.Marshal(ArtifactKind(%d)) = %s, want %s", raw, wire, wantWire)
		}
		var got ArtifactKind
		gotErr = json.Unmarshal(wire, &got)
		if gotErr != nil || got != kind {
			t.Fatalf("json.Unmarshal(%s) = (%d, %v), want (%d, nil)", wire, got, gotErr, kind)
		}
	}
}

func TestArtifactKindWireLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive every admitted token round trips to its exact kind", func(t *testing.T) {
		t.Parallel()

		for _, want := range []ArtifactKind{ArtifactCorpus, ArtifactCrasher} {
			wire, marshalErr := json.Marshal(want)
			if marshalErr != nil {
				t.Fatalf("json.Marshal(%d) error = %v, want nil", want, marshalErr)
			}
			parsed, parseErr := ParseArtifactKind(want.String())
			if parseErr != nil || parsed != want {
				t.Fatalf("ParseArtifactKind(%q) = (%d, %v), want (%d, nil)", want.String(), parsed, parseErr, want)
			}
			var got ArtifactKind
			if unmarshalErr := json.Unmarshal(wire, &got); unmarshalErr != nil || got != want {
				t.Fatalf("json.Unmarshal(%s) = (%d, %v), want (%d, nil)", wire, got, unmarshalErr, want)
			}
		}
	})
	t.Run("negative a rejected document leaves the receiver untouched", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name       string
			wire       []byte
			wantSyntax bool
		}{
			{name: "empty document", wire: nil, wantSyntax: true},
			{name: "null token", wire: []byte("null")},
			{name: "number token", wire: []byte("1")},
			{name: "boolean token", wire: []byte("true")},
			{name: "array token", wire: []byte("[]")},
			{name: "object token", wire: []byte("{}")},
			{name: "quoted array", wire: []byte(`"[]"`)},
			{name: "empty string", wire: []byte(`""`)},
			{name: "diagnostic unknown token is not admitted as wire", wire: []byte(`"` + core.UnknownEnumDiagnostic + `"`)},
			{name: "case changed corpus", wire: []byte(`"FUZZ-CORPUS"`)},
			{name: "underscore instead of hyphen", wire: []byte(`"fuzz_corpus"`)},
			{name: "leading whitespace inside the token", wire: []byte(`" fuzz-corpus"`)},
			{name: "trailing whitespace inside the token", wire: []byte(`"fuzz-corpus "`)},
			{name: "corpus token with an escaped nul byte", wire: []byte(`"fuzz-corpus\u0000"`)},
			{name: "unpaired high surrogate escape", wire: []byte(`"\ud800"`)},
			{name: "unpaired low surrogate escape", wire: []byte(`"\udc00"`)},
			{name: "invalid utf-8 byte inside the token", wire: []byte("\"fuzz-corpus\xff\"")},
			{name: "truncated string", wire: []byte(`"fuzz-corpus`), wantSyntax: true},
			{name: "valid token with trailing document", wire: []byte(`"fuzz-corpus" true`), wantSyntax: true},
			{name: "token at the exact extent ceiling", wire: []byte(strconv.Quote(strings.Repeat("a", artifactKindJSONMaximumBytes-2)))},
			{name: "token one byte past the extent ceiling", wire: []byte(strconv.Quote(strings.Repeat("a", artifactKindJSONMaximumBytes-1)))},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				t.Parallel()

				got := ArtifactCrasher
				gotErr := json.Unmarshal(tc.wire, &got)
				var syntaxErr *json.SyntaxError
				if tc.wantSyntax {
					if !errors.As(gotErr, &syntaxErr) || got != ArtifactCrasher {
						t.Fatalf("json.Unmarshal(%q) = (%d, %v), want unchanged %d and *json.SyntaxError", tc.wire, got, gotErr, ArtifactCrasher)
					}
					return
				}
				if !errors.Is(gotErr, core.ErrFuzzFinderContract) || got != ArtifactCrasher {
					t.Fatalf("json.Unmarshal(%q) = (%d, %v), want unchanged %d and %v", tc.wire, got, gotErr, ArtifactCrasher, core.ErrFuzzFinderContract)
				}
			})
		}
	})
	t.Run("neutral a nil receiver refuses instead of writing through", func(t *testing.T) {
		t.Parallel()

		var target *ArtifactKind
		gotErr := target.UnmarshalJSON([]byte(strconv.Quote(ArtifactCorpus.String())))
		if !errors.Is(gotErr, core.ErrFuzzFinderContract) {
			t.Fatalf("nil ArtifactKind.UnmarshalJSON() error = %v, want %v", gotErr, core.ErrFuzzFinderContract)
		}
	})
}

func TestArtifactKindDecodeInheritsTheCoreStringTokenContract(t *testing.T) {
	t.Parallel()

	// The package must not carry its own JSON string-token admission rule. These
	// two documents are refused by core's shared decoder and were previously
	// admitted far enough for Go's decoder to replace the surrogate with U+FFFD
	// before any domain parse ran.
	for _, wire := range [][]byte{[]byte(`"\ud800"`), []byte("\"\xff\"")} {
		got := ArtifactCorpus
		gotErr := (&got).UnmarshalJSON(wire)
		if !errors.Is(gotErr, core.ErrJSONContract) || !errors.Is(gotErr, core.ErrFuzzFinderContract) {
			t.Fatalf("ArtifactKind.UnmarshalJSON(%q) error = %v, want %v and %v", wire, gotErr, core.ErrJSONContract, core.ErrFuzzFinderContract)
		}
		if got != ArtifactCorpus {
			t.Fatalf("ArtifactKind.UnmarshalJSON(%q) receiver = %d, want unchanged %d", wire, got, ArtifactCorpus)
		}
	}
}
