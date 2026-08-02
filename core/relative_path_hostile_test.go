package core

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestParseRelativePathHostileBoundaryMatrix(t *testing.T) {
	t.Parallel()

	separator := string(filepath.Separator)
	component254 := strings.Repeat("a", filesystemPathComponentMaximumBytes-1)
	component255 := strings.Repeat("a", filesystemPathComponentMaximumBytes)
	component256 := strings.Repeat("a", filesystemPathComponentMaximumBytes+1)
	components255 := strings.Repeat("a"+separator, FilesystemPathMaximumComponents-1) + "a"
	components256 := strings.Repeat("a"+separator, FilesystemPathMaximumComponents)
	runes4095 := relativePathWithRunes(t, filesystemPathMaximumRunes-1)
	runes4096 := relativePathWithRunes(t, filesystemPathMaximumRunes)
	runes4097 := relativePathWithRunes(t, filesystemPathMaximumRunes+1)

	cases := []struct {
		name      string
		value     string
		wantValid bool
	}{
		// Expected-valid domain: ordinary, Unicode, and both exact ceilings.
		{name: "v01 root dot names the os root itself", value: ".", wantValid: true},
		{name: "v02 one component is local", value: "child", wantValid: true},
		{name: "v03 two components are local", value: filepath.Join("a", "b"), wantValid: true},
		{name: "v04 dotfile component is local", value: ".ledger", wantValid: true},
		{name: "v05 spaces remain native filename bytes", value: "dated evidence", wantValid: true},
		{name: "v06 unicode component is valid utf8", value: "évidence", wantValid: true},
		{name: "v07 component one below byte ceiling", value: component254, wantValid: true},
		{name: "v08 component at byte ceiling", value: component255, wantValid: true},
		{name: "v09 component count at ceiling", value: components255, wantValid: true},
		{name: "v10 path runes one below ceiling", value: runes4095, wantValid: true},
		{name: "v11 path runes at ceiling", value: runes4096, wantValid: true},
		{name: "v12 punctuation remains native", value: filepath.Join("2026-07-28", "ledger.0001.jsonl"), wantValid: true},

		// Expected-invalid domain: every lexical escape and malformed encoding.
		{name: "n01 empty path has no rooted meaning", value: ""},
		{name: "n02 absolute path bypasses root", value: filepath.Join(separator, "outside")},
		{name: "n03 bare parent escapes root", value: ".."},
		{name: "n04 parent child escapes root", value: filepath.Join("..", "outside")},
		{name: "n05 embedded parent traversal is nonlocal", value: "a" + separator + ".." + separator + "b"},
		{name: "n06 embedded current directory is noncanonical", value: "a" + separator + "." + separator + "b"},
		{name: "n07 trailing separator is noncanonical", value: "a" + separator},
		{name: "n08 repeated separator is noncanonical", value: "a" + separator + separator + "b"},
		{name: "n09 nul cannot reach os root", value: "a\x00b"},
		{name: "n10 invalid utf8 cannot name a typed path", value: string([]byte{0xff})},
		{name: "n11 component one above byte ceiling", value: component256},
		{name: "n12 component count one above ceiling", value: components256},
		{name: "n13 path runes one above ceiling", value: runes4097},

		// Boundary pressure around normalization, bytes, components, and runes.
		{name: "b01 single ASCII byte", value: "a", wantValid: true},
		{name: "b02 single multibyte rune", value: "界", wantValid: true},
		{name: "b03 two maximum independent components", value: filepath.Join(component255, component255), wantValid: true},
		{name: "b04 unicode bytes one below component ceiling", value: strings.Repeat("é", 127), wantValid: true},
		{name: "b05 unicode bytes at component ceiling", value: strings.Repeat("é", 127) + "a", wantValid: true},
		{name: "b06 unicode bytes one above component ceiling", value: strings.Repeat("é", 128)},
		{name: "b07 component count one below ceiling", value: strings.Repeat("a"+separator, FilesystemPathMaximumComponents-2) + "a", wantValid: true},
		{name: "b08 component count exact ceiling", value: components255, wantValid: true},
		{name: "b09 component count one above ceiling", value: components256},
		{name: "b10 rune count one below ceiling", value: runes4095, wantValid: true},
		{name: "b11 rune count exact ceiling", value: runes4096, wantValid: true},
		{name: "b12 rune count one above ceiling", value: runes4097},
		{name: "b13 leading current directory is noncanonical", value: "." + separator + "a"},
		{name: "b14 two leading current directories are noncanonical", value: "." + separator + "." + separator + "a"},
		{name: "b15 cleaned parent pair is rejected rather than rewritten", value: "a" + separator + "b" + separator + ".."},
		{name: "b16 clean sibling name remains local", value: filepath.Join("a", "bc"), wantValid: true},
		{name: "b17 root-looking sibling prefix remains relative", value: "tmp-root-sibling", wantValid: true},
		{name: "b18 newline is a native filename byte", value: "line\nbreak", wantValid: true},
		{name: "b19 tab is a native filename byte", value: "tab\tname", wantValid: true},
		{name: "b20 zero value is represented only by failed parse", value: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := ParseRelativePath(tc.value)
			if tc.wantValid {
				if gotErr != nil {
					t.Fatalf("ParseRelativePath(%q) error = %v, want nil", tc.value, gotErr)
				}
				if got.String() != tc.value {
					t.Fatalf("ParseRelativePath(%q).String() = %q, want %q", tc.value, got.String(), tc.value)
				}
				if got.Validate() != nil || !filepath.IsLocal(got.String()) ||
					filepath.Clean(got.String()) != got.String() {
					t.Fatalf("RelativePath(%q) lost local canonical invariants", got.String())
				}
				return
			}
			if !errors.Is(gotErr, ErrPrimitiveContract) {
				t.Fatalf("ParseRelativePath(%q) error = %v, want %v", tc.value, gotErr, ErrPrimitiveContract)
			}
			if got != (RelativePath{}) {
				t.Fatalf("ParseRelativePath(%q) = %v, want zero value", tc.value, got)
			}
		})
	}
}

func TestRelativePathZeroValueRejectsEveryPublicOperation(t *testing.T) {
	t.Parallel()

	if gotErr := (RelativePath{}).Validate(); !errors.Is(gotErr, ErrPrimitiveContract) {
		t.Fatalf("RelativePath{}.Validate() error = %v, want %v", gotErr, ErrPrimitiveContract)
	}
	if got := (RelativePath{}).String(); got != "" {
		t.Fatalf("RelativePath{}.String() = %q, want empty", got)
	}
}

func FuzzParseRelativePathSemanticClosure(f *testing.F) {
	for _, seed := range []string{
		".", "a", filepath.Join("a", "b"), "..", filepath.Join("..", "x"),
		"a\x00b", string([]byte{0xff}), strings.Repeat("a", filesystemPathComponentMaximumBytes),
		strings.Repeat("a", filesystemPathComponentMaximumBytes+1),
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		got, gotErr := ParseRelativePath(value)
		if gotErr != nil {
			if !errors.Is(gotErr, ErrPrimitiveContract) {
				t.Fatalf("ParseRelativePath(%q) error = %v, want %v", value, gotErr, ErrPrimitiveContract)
			}
			if got != (RelativePath{}) {
				t.Fatalf("ParseRelativePath(%q) rejected with value %v, want zero", value, got)
			}
			return
		}
		if got.Validate() != nil || got.String() != value || !utf8.ValidString(value) ||
			!filepath.IsLocal(value) || filepath.Clean(value) != value {
			t.Fatalf("ParseRelativePath(%q) accepted value outside semantic closure", value)
		}
	})
}

func relativePathWithRunes(t *testing.T, runes int) string {
	t.Helper()

	if runes < 1 {
		t.Fatalf("relativePathWithRunes(%d) requires positive runes", runes)
	}
	componentCount := (runes + filesystemPathComponentMaximumBytes + 1) /
		(filesystemPathComponentMaximumBytes + 1)
	if componentCount > FilesystemPathMaximumComponents {
		t.Fatalf("relativePathWithRunes(%d) needs %d components, maximum %d", runes, componentCount, FilesystemPathMaximumComponents)
	}
	componentRunes := runes - (componentCount - 1)
	components := make([]string, componentCount)
	for index := range componentCount {
		remainingComponents := componentCount - index
		size := min(
			componentRunes-(remainingComponents-1),
			filesystemPathComponentMaximumBytes,
		)
		components[index] = strings.Repeat("a", size)
		componentRunes -= size
	}
	return filepath.Join(components...)
}
