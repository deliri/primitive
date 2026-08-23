package controlwire_test

import (
	json "encoding/json/v2"
	"errors"
	"strings"
	"testing"

	"encoding/json/jsontext"

	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
)

// TestRevisionValidateExhaustsTheClosedDomain walks every representable
// revision, not a sample. The domain is one published value wide, so an
// off-by-one in the limit comparison is the entire bug class and exhaustion is
// cheaper than a table of guesses.
func TestRevisionValidateExhaustsTheClosedDomain(t *testing.T) {
	t.Parallel()

	for candidate := 0; candidate <= 255; candidate++ {
		revision := controlwire.Revision(candidate)
		wantValid := revision == controlwire.Revision2026V1
		if got := revision.IsValid(); got != wantValid {
			t.Errorf("Revision(%d).IsValid() = %t, want %t", candidate, got, wantValid)
		}
		err := revision.Validate()
		if wantValid {
			if err != nil {
				t.Errorf("Revision(%d).Validate() error = %v, want nil", candidate, err)
			}
			continue
		}
		if !errors.Is(err, core.ErrControlWireRevision) {
			t.Errorf("Revision(%d).Validate() error = %v, want %v", candidate, err, core.ErrControlWireRevision)
		}
		if !errors.Is(err, core.ErrControlWireContract) {
			t.Errorf("Revision(%d).Validate() error = %v, want %v", candidate, err, core.ErrControlWireContract)
		}
		if !errors.Is(err, core.ErrPrimitiveContract) {
			t.Errorf("Revision(%d).Validate() parent = %v, want %v", candidate, err, core.ErrPrimitiveContract)
		}
	}
}

// TestRevisionStringIsEmptyOutsideTheDomain proves an unpublished revision
// renders as empty text rather than indexing past the token array or inventing
// a token a peer might accept.
func TestRevisionStringIsEmptyOutsideTheDomain(t *testing.T) {
	t.Parallel()

	if got, want := controlwire.Revision2026V1.String(), controlwire.Revision2026V1Token; got != want {
		t.Fatalf("Revision2026V1.String() = %q, want %q", got, want)
	}
	for candidate := 0; candidate <= 255; candidate++ {
		revision := controlwire.Revision(candidate)
		if revision == controlwire.Revision2026V1 {
			continue
		}
		if got := revision.String(); got != "" {
			t.Errorf("Revision(%d).String() = %q, want empty", candidate, got)
		}
	}
}

func TestParseRevisionRefusesEveryTokenButThePublishedOne(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name  string
		token string
		want  controlwire.Revision
	}{
		{name: "exact published token is accepted", token: "2026.1", want: controlwire.Revision2026V1},
		{name: "empty token is refused", token: ""},
		{name: "trailing space is refused rather than trimmed", token: "2026.1 "},
		{name: "leading space is refused rather than trimmed", token: " 2026.1"},
		{name: "leading tab is refused", token: "\t2026.1"},
		{name: "trailing newline is refused", token: "2026.1\n"},
		{name: "interior space is refused", token: "2026. 1"},
		{name: "future minor revision is refused not assumed compatible", token: "2026.2"},
		{name: "future major revision is refused not assumed compatible", token: "2027.1"},
		{name: "prior year revision is refused", token: "2025.1"},
		{name: "digit-appended token is refused", token: "2026.11"},
		{name: "digit-prepended token is refused", token: "12026.1"},
		{name: "truncated at separator is refused", token: "2026."},
		{name: "truncated before separator is refused", token: "2026"},
		{name: "minor alone is refused", token: ".1"},
		{name: "separator alone is refused", token: "."},
		{name: "comma separator is refused", token: "2026,1"},
		{name: "dash separator is refused", token: "2026-1"},
		{name: "underscore separator is refused", token: "2026_1"},
		{name: "zero-padded minor is refused", token: "2026.01"},
		{name: "trailing zero minor is refused", token: "2026.10"},
		{name: "lease revision token is refused", token: "v1"},
		{name: "v-prefixed token is refused", token: "v2026.1"},
		{name: "uppercase v-prefixed token is refused", token: "V2026.1"},
		{name: "semver-shaped token is refused", token: "2026.1.0"},
		{name: "quoted token is refused", token: `"2026.1"`},
		{name: "null byte suffix is refused", token: "2026.1\x00"},
		{name: "null byte prefix is refused", token: "\x002026.1"},
		{name: "embedded null byte is refused", token: "2026\x00.1"},
		{name: "fullwidth digit lookalike is refused", token: "２０２６.１"},
		{name: "arabic-indic digit lookalike is refused", token: "٢٠٢٦.١"},
		{name: "one-dot-leader lookalike separator is refused", token: "2026․1"},
		{name: "zero-width space inside token is refused", token: "2026.\u200b1"},
		{name: "byte-order mark prefix is refused", token: "\ufeff2026.1"},
		{name: "repeated token is refused", token: "2026.12026.1"},
		{name: "oversized token is refused", token: strings.Repeat("2026.1", 4096)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := controlwire.ParseRevision(tc.token)
			if got != tc.want {
				t.Errorf("ParseRevision(%q) = %v, want %v", tc.token, got, tc.want)
			}
			if tc.want != controlwire.RevisionUnknown {
				if err != nil {
					t.Fatalf("ParseRevision(%q) error = %v, want nil", tc.token, err)
				}
				return
			}
			if !errors.Is(err, core.ErrControlWireRevision) {
				t.Fatalf("ParseRevision(%q) error = %v, want %v", tc.token, err, core.ErrControlWireRevision)
			}
		})
	}
}

func TestRevisionMarshalJSONRefusesToGuessAContract(t *testing.T) {
	t.Parallel()

	got, err := json.Marshal(controlwire.Revision2026V1)
	if err != nil {
		t.Fatalf("json.Marshal(Revision2026V1) error = %v, want nil", err)
	}
	if want := `"2026.1"`; string(got) != want {
		t.Fatalf("json.Marshal(Revision2026V1) = %s, want %s", got, want)
	}
	for _, revision := range []controlwire.Revision{
		controlwire.RevisionUnknown,
		controlwire.Revision(2),
		controlwire.Revision(255),
	} {
		encoded, err := json.Marshal(revision)
		if !errors.Is(err, core.ErrControlWireRevision) {
			t.Errorf("json.Marshal(Revision(%d)) error = %v, want %v", revision, err, core.ErrControlWireRevision)
		}
		if !errors.Is(err, core.ErrJSONContract) {
			t.Errorf("json.Marshal(Revision(%d)) error = %v, want %v", revision, err, core.ErrJSONContract)
		}
		if len(encoded) != 0 {
			t.Errorf("json.Marshal(Revision(%d)) = %s, want no bytes", revision, encoded)
		}
	}
}

func TestRevisionUnmarshalJSONLeavesTheReceiverUnchangedOnRejection(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		document   string
		want       controlwire.Revision
		wantSyntax bool
	}{
		{name: "canonical token is accepted", document: `"2026.1"`, want: controlwire.Revision2026V1},
		{name: "escaped separator decodes to the published token", document: `"2026\u002e1"`, want: controlwire.Revision2026V1},
		{name: "json null is refused", document: `null`},
		{name: "bare token without quotes is refused", document: `2026.1`},
		{name: "number is refused", document: `2026`},
		{name: "float is refused", document: `2026.1e0`},
		{name: "boolean true is refused", document: `true`},
		{name: "boolean false is refused", document: `false`},
		{name: "empty string is refused", document: `""`},
		{name: "empty document is refused", document: ``, wantSyntax: true},
		{name: "object is refused", document: `{"revision":"2026.1"}`},
		{name: "array is refused", document: `["2026.1"]`},
		{name: "array of one token is refused", document: `["2026.1","2026.1"]`},
		{name: "unterminated string is refused", document: `"2026.1`, wantSyntax: true},
		{name: "unopened string is refused as a type mismatch", document: `2026.1"`},
		{name: "whitespace-padded token is refused after decoding", document: `" 2026.1 "`},
		{name: "future revision token is refused", document: `"2026.2"`},
		{name: "lease revision token is refused", document: `"v1"`},
		{name: "unpaired high surrogate is refused by JSON v2 syntax", document: `"\ud800"`, wantSyntax: true},
		{name: "unpaired low surrogate is refused by JSON v2 syntax", document: `"\udc00"`, wantSyntax: true},
		{name: "escaped null byte suffix is refused", document: `"2026.1\u0000"`},
		{name: "trailing content after token is refused", document: `"2026.1"trailing`, wantSyntax: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			// A non-zero prior value proves rejection does not silently reset
			// the field to the unset revision, which would read downstream as
			// "absent" rather than "refused".
			got := controlwire.Revision2026V1
			err := json.Unmarshal([]byte(tc.document), &got)
			if tc.want != controlwire.RevisionUnknown {
				if err != nil {
					t.Fatalf("json.Unmarshal(%s) error = %v, want nil", tc.document, err)
				}
				if got != tc.want {
					t.Fatalf("json.Unmarshal(%s) = %v, want %v", tc.document, got, tc.want)
				}
				return
			}
			if tc.wantSyntax {
				if _, ok := errors.AsType[*jsontext.SyntacticError](err); !ok {
					t.Fatalf("json.Unmarshal(%s) error = %v, want errors.As *jsontext.SyntacticError", tc.document, err)
				}
			} else if !errors.Is(err, core.ErrControlWireRevision) || !errors.Is(err, core.ErrJSONContract) {
				t.Fatalf("json.Unmarshal(%s) error = %v, want errors.Is %v and %v", tc.document, err,
					core.ErrControlWireRevision, core.ErrJSONContract)
			}
			if got != controlwire.Revision2026V1 {
				t.Fatalf("json.Unmarshal(%s) mutated receiver to %v, want %v", tc.document, got, controlwire.Revision2026V1)
			}
		})
	}
}

// TestRevisionUnmarshalJSONOnNilReceiverIsRejected proves the nil guard reports
// a typed rejection instead of panicking through a nil dereference.
func TestRevisionUnmarshalJSONOnNilReceiverIsRejected(t *testing.T) {
	t.Parallel()

	var revision *controlwire.Revision
	err := revision.UnmarshalJSON([]byte(`"2026.1"`))
	if !errors.Is(err, core.ErrControlWireRevision) {
		t.Fatalf("(*Revision)(nil).UnmarshalJSON() error = %v, want %v", err, core.ErrControlWireRevision)
	}
}

// TestRevisionRoundTripsThroughItsCanonicalToken proves marshal and unmarshal
// are inverses, so a value that survives a peer hop is the value that was sent.
func TestRevisionRoundTripsThroughItsCanonicalToken(t *testing.T) {
	t.Parallel()

	encoded, err := json.Marshal(controlwire.Revision2026V1)
	if err != nil {
		t.Fatalf("json.Marshal() error = %v, want nil", err)
	}
	var got controlwire.Revision
	if err := json.Unmarshal(encoded, &got); err != nil {
		t.Fatalf("json.Unmarshal(%s) error = %v, want nil", encoded, err)
	}
	if got != controlwire.Revision2026V1 {
		t.Fatalf("round trip = %v, want %v", got, controlwire.Revision2026V1)
	}
	parsed, err := controlwire.ParseRevision(got.String())
	if err != nil || parsed != controlwire.Revision2026V1 {
		t.Fatalf("ParseRevision(String()) = (%v, %v), want (%v, nil)", parsed, err, controlwire.Revision2026V1)
	}
}
