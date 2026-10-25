package runprotocol

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// The generated grammar dimensions have independently known candidate sets.
// The arbitrary child remains ASCII-rendered data, not removed; real slashes,
// alternations, bracket nesting, grouping and escapes still reach the parser.
// Arbitrary raw patterns additionally retain closure/refusal checks. No oracle
// calls the production splitter or its name-rewrite helper.
func FuzzGoTestPatternFollowsTestingSplit(f *testing.F) {
	canonical, err := ParseGoTestPattern("^FuzzF$/seed")
	if err != nil {
		f.Fatal(err)
	}
	f.Add(canonical.String())
	f.Add("FuzzG|x/(seed)\\tail")
	f.Add("\t\u2000\x00\xff")
	f.Add("\u200d") // Go testing's nonprintable rewrite can itself be invalid regex syntax.
	f.Add("")
	f.Fuzz(func(t *testing.T, raw string) {
		admitted, rawErr := ParseGoTestPattern(raw)
		if rawErr != nil {
			if !errors.Is(rawErr, core.ErrGoToolchainContract) || admitted != (GoTestPattern{}) {
				t.Fatalf("raw pattern refusal = (%v,%v), want zero and typed contract", admitted, rawErr)
			}
		} else if admitted.String() != raw || admitted.Validate() != nil {
			t.Fatalf("raw pattern closure = (%q,%v), want exact original and valid", admitted.String(), admitted.Validate())
		}
		// Rendering data with Go's own ASCII quoting keeps generated child syntax
		// independently valid even when raw testing.rewrite would emit \uNNNN.
		child := strings.ReplaceAll(regexp.QuoteMeta(strconv.QuoteToASCII(raw)), "/", `\/`)
		for _, row := range []struct {
			pattern                               string
			wantF, wantG, wantUnderscore, invalid bool
		}{
			{pattern: "^FuzzF$", wantF: true},
			{pattern: "[F/]uzzF$", wantF: true},
			{pattern: "FuzzF(/" + child + ")?", wantF: true},
			{pattern: "(F)uzzF/" + child, wantF: true},
			{pattern: "]?FuzzF/" + child, wantF: true},
			{pattern: `Fuzz\/F`},
			{pattern: "Other/" + child + "|^FuzzF$", wantF: true},
			{pattern: "^FuzzF$|^FuzzG$", wantF: true, wantG: true},
			{pattern: "^Missing$/^FuzzF$"},
			{pattern: "^FuzzF$/(", invalid: true},
			{pattern: "(FuzzF", invalid: true},
			{pattern: "Fuzz\tF$", wantUnderscore: true},
			{pattern: "Fuzz\u2000F$", wantUnderscore: true},
			{pattern: "Fuzz\u2028F$", wantUnderscore: true},
		} {
			pattern, err := ParseGoTestPattern(row.pattern)
			if row.invalid {
				if !errors.Is(err, core.ErrGoToolchainContract) || pattern != (GoTestPattern{}) {
					t.Fatalf("generated invalid pattern %q = (%v,%v), want zero typed refusal", row.pattern, pattern, err)
				}
				continue
			}
			if err != nil || pattern.Validate() != nil || pattern.String() != row.pattern {
				t.Fatalf("generated pattern %q = (%v,%v), want exact admitted pattern", row.pattern, pattern, err)
			}
			for _, candidate := range []struct {
				name string
				want bool
			}{
				{name: "FuzzF", want: row.wantF}, {name: "FuzzG", want: row.wantG}, {name: "Fuzz_F", want: row.wantUnderscore},
			} {
				got, err := pattern.SelectsTopLevel(candidate.name)
				if err != nil || got != candidate.want {
					t.Fatalf("generated %q selects %q = (%t,%v), want %t", row.pattern, candidate.name, got, err, candidate.want)
				}
			}
		}
	})
}
