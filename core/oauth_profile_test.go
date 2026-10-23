package core

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"unicode"
	"unicode/utf8"
)

func TestOAuthProfileNameAndOutcomeLayerTriad(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name string
		text string
		want bool
	}{
		{name: "ordinary full name", text: "Ada Lovelace", want: true},
		{name: "provider spelling preserved", text: "  Ada  Lovelace  ", want: true},
		{name: "non-Latin human name", text: "李小龍", want: true},
		{name: "combining characters preserved", text: "He\u0301le\u0300ne", want: true},
		{name: "joiner is legitimate name data", text: "نیم\u200cفاصله", want: true},
		{name: "long valid name has no invented flow quota", text: strings.Repeat("名字", 4096), want: true},
		{name: "absent is not an issued name"},
		{name: "whitespace only is absent", text: " \u2003 "},
		{name: "embedded line break is unsafe name data", text: "Ada\nLovelace"},
		{name: "embedded zero is unsafe name data", text: "Ada\x00"},
		{name: "C1 control is unsafe name data", text: "Ada\u0085Lovelace"},
		{name: "malformed Unicode is refused", text: string([]byte{0xff})},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseOAuthProfileName(row.text)
			if (err == nil) != row.want || (row.want && got.String() != row.text) {
				t.Fatalf("profile name = (%q,%v), want admitted%t with exact source spelling", got.String(), err, row.want)
			}
			if err != nil && (!errors.Is(err, ErrOAuthContract) || got != (OAuthProfileName{})) {
				t.Fatalf("profile name refusal = (%+v,%v), want zero typed refusal", got, err)
			}
		})
	}
}

func TestOAuthProfileOutcomeClosedDomain(t *testing.T) {
	t.Parallel()
	for raw := range 256 {
		value := OAuthProfileOutcome(raw)
		encoded, err := value.MarshalJSON()
		if raw > int(OAuthProfileUnavailable) {
			if !errors.Is(err, ErrOAuthContract) || len(encoded) != 0 || value.IsValid() {
				t.Fatalf("future profile outcome%d = (%q,%v), want typed refusal", raw, encoded, err)
			}
			continue
		}
		var decoded OAuthProfileOutcome
		if err != nil || decoded.UnmarshalJSON(encoded) != nil || decoded != value {
			t.Fatalf("profile outcome%d = (%q,%v,%d), want exact closed roundtrip", raw, encoded, err, decoded)
		}
	}
}

func FuzzOAuthProfileNameSemanticClosure(f *testing.F) {
	seed, err := ParseOAuthProfileName("Ada Lovelace")
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seed.String())
	f.Add("")
	f.Add("\x00")
	f.Fuzz(func(t *testing.T, text string) {
		// Native Unicode predicates decide the open data domain independently of
		// the production receiver and its JSON methods.
		want := utf8.ValidString(text) && strings.IndexFunc(text, unicode.IsControl) < 0 && strings.IndexFunc(text, func(r rune) bool { return !unicode.IsSpace(r) }) >= 0
		got, err := ParseOAuthProfileName(text)
		if (err == nil) != want {
			t.Fatalf("profile name admission = %v, want%t", err, want)
		}
		if err != nil {
			if !errors.Is(err, ErrOAuthContract) || got != (OAuthProfileName{}) {
				t.Fatalf("profile name refusal = (%+v,%v), want zero typed refusal", got, err)
			}
			return
		}
		if got.String() != text || got.Validate() != nil {
			t.Fatalf("profile name projection = %q, want exact source%q", got.String(), text)
		}
		canonical, err := got.MarshalJSON()
		var roundTrip OAuthProfileName
		if err != nil || roundTrip.UnmarshalJSON(canonical) != nil || roundTrip != got {
			t.Fatalf("profile name JSON = (%q,%v), want exact canonical roundtrip", canonical, err)
		}
		second, err := roundTrip.MarshalJSON()
		if err != nil || !bytes.Equal(second, canonical) {
			t.Fatalf("profile name fixed point = (%q,%v), want%q", second, err, canonical)
		}
	})
}
