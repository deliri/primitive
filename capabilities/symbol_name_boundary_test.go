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
		{"minimum lowercase letter", "a", true}, {"minimum uppercase letter", "A", true},
		{"underscore followed by letter", "_a", true}, {"letter followed by digit", "a0", true},
		{"underscore followed by digit", "_0", true}, {"two underscores", "__", true},
		{"Latin non ASCII letter", "é", true}, {"Greek letter", "Ω", true},
		{"CJK letter", "界", true}, {"four byte letter", "𐐀", true},
		{"letter with Unicode decimal digit", "a١", true}, {"long identifier has no invented cap", strings.Repeat("a", core.JSONDocumentMaximumBytes+1), true},
		{"empty identifier", "", false}, {"blank identifier is not selectable", "_", false},
		{"ASCII digit prefix", "0a", false}, {"Unicode digit prefix", "١a", false},
		{"combining mark cannot begin identifier", "́a", false}, {"combining mark is not a letter tail", "á", false},
		{"emoji is not a letter", "😀", false}, {"zero width joiner is not identifier material", "a\u200d", false},
		{"dot means selector expression", "a.b", false}, {"pointer expression", "*File", false},
		{"type arguments are not selector text", "F[T]", false}, {"call expression", "F()", false},
		{"leading ASCII space", " a", false}, {"trailing ASCII space", "a ", false},
		{"leading newline", "\na", false}, {"trailing carriage return", "a\r", false},
		{"embedded NUL", "a\x00", false}, {"non breaking space", "a\u00a0", false},
		{"UTF8 continuation without leader", "\x80", false}, {"truncated two byte scalar", "\xc3", false},
		{"truncated three byte scalar", "\xe7\x95", false}, {"truncated four byte scalar", "\xf0\x90\x90", false},
		{"overlong ASCII letter", "\xc1\xa1", false}, {"UTF8 surrogate", "\xed\xa0\x80", false},
		{"scalar above Unicode ceiling", "\xf4\x90\x80\x80", false},
		{"replacement character is not identifier letter", "�", false},
		{"UTF8 BOM is not identifier material", "\ufeffa", false},
		{"comment is not an identifier", "a/*x*/", false},
	}
	for keyword := token.BREAK; keyword <= token.VAR; keyword++ {
		cases = append(cases, struct {
			name, source string
			wantValid    bool
		}{"keyword " + keyword.String(), keyword.String(), false})
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
