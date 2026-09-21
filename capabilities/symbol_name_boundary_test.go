package capabilities

import (
	"errors"
	"go/token"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestSymbolNameUsesExactGoLexicalAdmission(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, source string
		wantValid    bool
	}{
		{name: "minimum lowercase letter", source: "a", wantValid: true}, {name: "minimum uppercase letter", source: "A", wantValid: true},
		{name: "underscore followed by letter", source: "_a", wantValid: true}, {name: "letter followed by digit", source: "a0", wantValid: true},
		{name: "underscore followed by digit", source: "_0", wantValid: true}, {name: "two underscores", source: "__", wantValid: true},
		{name: "Latin non ASCII letter", source: "é", wantValid: true}, {name: "Greek letter", source: "Ω", wantValid: true},
		{name: "CJK letter", source: "界", wantValid: true}, {name: "four byte letter", source: "𐐀", wantValid: true},
		{name: "letter with Unicode decimal digit", source: "a١", wantValid: true}, {name: "long identifier has no invented cap", source: strings.Repeat("a", core.JSONDocumentMaximumBytes+1), wantValid: true},
		{name: "empty identifier", source: "", wantValid: false}, {name: "blank identifier is not selectable", source: "_", wantValid: false},
		{name: "ASCII digit prefix", source: "0a", wantValid: false}, {name: "Unicode digit prefix", source: "١a", wantValid: false},
		{name: "combining mark cannot begin identifier", source: "́a", wantValid: false}, {name: "combining mark is not a letter tail", source: "á", wantValid: false},
		{name: "emoji is not a letter", source: "😀", wantValid: false}, {name: "zero width joiner is not identifier material", source: "a\u200d", wantValid: false},
		{name: "dot means selector expression", source: "a.b", wantValid: false}, {name: "pointer expression", source: "*File", wantValid: false},
		{name: "type arguments are not selector text", source: "F[T]", wantValid: false}, {name: "call expression", source: "F()", wantValid: false},
		{name: "leading ASCII space", source: " a", wantValid: false}, {name: "trailing ASCII space", source: "a ", wantValid: false},
		{name: "leading newline", source: "\na", wantValid: false}, {name: "trailing carriage return", source: "a\r", wantValid: false},
		{name: "embedded NUL", source: "a\x00", wantValid: false}, {name: "non breaking space", source: "a\u00a0", wantValid: false},
		{name: "UTF8 continuation without leader", source: "\x80", wantValid: false}, {name: "truncated two byte scalar", source: "\xc3", wantValid: false},
		{name: "truncated three byte scalar", source: "\xe7\x95", wantValid: false}, {name: "truncated four byte scalar", source: "\xf0\x90\x90", wantValid: false},
		{name: "overlong ASCII letter", source: "\xc1\xa1", wantValid: false}, {name: "UTF8 surrogate", source: "\xed\xa0\x80", wantValid: false},
		{name: "scalar above Unicode ceiling", source: "\xf4\x90\x80\x80", wantValid: false},
		{name: "replacement character is not identifier letter", source: "�", wantValid: false},
		{name: "UTF8 BOM is not identifier material", source: "\ufeffa", wantValid: false},
		{name: "comment is not an identifier", source: "a/*x*/", wantValid: false},
	}
	for keyword := token.BREAK; keyword <= token.VAR; keyword++ {
		cases = append(cases, struct {
			name, source string
			wantValid    bool
		}{name: "keyword " + keyword.String(), source: keyword.String(), wantValid: false})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseSymbolName(tc.source)
			if tc.wantValid {
				if err != nil || got.String() != tc.source {
					t.Fatalf("identifier = (%q,%v), want exact %q", got.String(), err, tc.source)
				}
				return
			}
			if !errors.Is(err, core.ErrCapabilitiesContract) || got != (SymbolName{}) {
				t.Fatalf("identifier = (%+v,%v), want zero typed refusal", got, err)
			}
		})
	}
}
