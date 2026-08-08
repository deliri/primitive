package core_test

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// TestResolveTextIsExactlyLexicalAbsIngress is the hostile table for the one
// door that admits operator-supplied path text. Every case states the exact
// absolute answer or the refusal identity; the base never moves, so a wrong
// answer is a wrong resolution, not a fixture accident.
func TestResolveTextIsExactlyLexicalAbsIngress(t *testing.T) {
	t.Parallel()

	separator := string(filepath.Separator)
	join := func(parts ...string) string {
		return separator + strings.Join(parts, separator)
	}
	base := absolutePathForTest(t, join("work", "dir"))
	maximumComponent := strings.Repeat("a", 255)
	oversizedComponent := strings.Repeat("a", 256)

	cases := []struct {
		name    string
		text    string
		want    string
		wantErr bool
	}{
		{name: "absolute clean text is admitted as itself", text: join("etc", "peach.json"), want: join("etc", "peach.json")},
		{name: "absolute noncanonical text is cleaned", text: join("etc") + separator + separator + "sub" + separator + "." + separator + "x", want: join("etc", "sub", "x")},
		{name: "absolute parent references resolve lexically", text: join("etc", "..", "var", "x"), want: join("var", "x")},
		{name: "the filesystem root resolves to itself", text: separator, want: separator},
		{name: "an absolute climb above the root clamps at the root", text: join("..", "..", "x"), want: join("x")},
		{name: "a bare name resolves below the base", text: "peach.json", want: join("work", "dir", "peach.json")},
		{name: "a nested relative path resolves below the base", text: "a" + separator + "b" + separator + "c", want: join("work", "dir", "a", "b", "c")},
		{name: "dot resolves to the base itself", text: ".", want: join("work", "dir")},
		{name: "an inner dot segment is cleaned away", text: "a" + separator + "." + separator + "b", want: join("work", "dir", "a", "b")},
		{name: "one parent reference climbs one level", text: "..", want: join("work")},
		{name: "parent references may leave the base lexically", text: ".." + separator + ".." + separator + "other", want: join("other")},
		{name: "a relative climb above the root clamps at the root", text: strings.Repeat(".."+separator, 8) + "x", want: join("x")},
		{name: "a trailing separator is cleaned away", text: "sub" + separator, want: join("work", "dir", "sub")},
		{name: "a maximum-width component is admitted", text: maximumComponent, want: join("work", "dir", maximumComponent)},
		{name: "empty text is refused rather than meaning the base", text: "", wantErr: true},
		{name: "text carrying a NUL byte is refused", text: "bad\x00name", wantErr: true},
		{name: "invalid UTF-8 text is refused", text: "\xff\xfe", wantErr: true},
		{name: "an oversized component is refused", text: oversizedComponent, wantErr: true},
		{name: "an absolute path with an oversized component is refused", text: separator + oversizedComponent, wantErr: true},
		{name: "an absolute path carrying a NUL byte is refused", text: join("x") + "\x00", wantErr: true},
		{name: "invalid UTF-8 absolute text is refused", text: separator + "\xff", wantErr: true},
		{name: "a component-count blowout is refused", text: strings.Repeat("a"+separator, core.FilesystemPathMaximumComponents+8) + "a", wantErr: true},
		{name: "a rune-limit blowout is refused", text: separator + strings.Repeat("é", 4200), wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := base.ResolveText(tc.text)
			if tc.wantErr {
				if !errors.Is(gotErr, core.ErrPrimitiveContract) {
					t.Fatalf("ResolveText(%q) error = %v, want errors.Is %v", tc.text, gotErr, core.ErrPrimitiveContract)
				}
				return
			}
			if gotErr != nil {
				t.Fatalf("ResolveText(%q) error = %v, want nil", tc.text, gotErr)
			}
			if got.String() != tc.want {
				t.Fatalf("ResolveText(%q) = %q, want %q", tc.text, got.String(), tc.want)
			}
		})
	}
}

// TestResolveTextRefusesAnUnsetBase holds the receiver gate: resolution
// against nothing has no meaning and must not invent one.
func TestResolveTextRefusesAnUnsetBase(t *testing.T) {
	t.Parallel()

	var zero core.AbsolutePath
	if _, err := zero.ResolveText("x"); !errors.Is(err, core.ErrPrimitiveContract) {
		t.Fatalf("zero base ResolveText(x) error = %v, want errors.Is %v", err, core.ErrPrimitiveContract)
	}
}
