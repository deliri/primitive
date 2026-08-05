package core_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// TestWithSuffixOnlyEverNamesASibling is the property that matters. String
// concatenation happily produces a path in another directory when the suffix
// carries a separator or a parent reference; the component gate makes that
// impossible rather than unlikely.
func TestWithSuffixOnlyEverNamesASibling(t *testing.T) {
	t.Parallel()

	root := string(filepath.Separator)
	base := absolutePathForTest(t, root+"work"+string(filepath.Separator)+"slot")

	cases := []struct {
		name    string
		suffix  string
		want    string
		wantErr bool
	}{
		{name: "plain suffix", suffix: ".lease", want: "slot.lease"},
		{name: "infix style suffix", suffix: "-quarantine-01", want: "slot-quarantine-01"},
		{name: "single character", suffix: "x", want: "slotx"},
		{name: "empty suffix names the path itself", suffix: "", want: "slot"},
		{name: "separator escapes into another directory", suffix: string(filepath.Separator) + "child", wantErr: true},
		{name: "leading separator escapes", suffix: string(filepath.Separator), wantErr: true},
		{name: "parent reference escapes", suffix: string(filepath.Separator) + "..", wantErr: true},
		{name: "embedded NUL is refused", suffix: "\x00", wantErr: true},
		{name: "oversized component is refused", suffix: strings.Repeat("a", 512), wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := base.WithSuffix(tc.suffix)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("WithSuffix(%q) = %q, want a refusal", tc.suffix, got.String())
				}
				return
			}
			if err != nil {
				t.Fatalf("WithSuffix(%q) error = %v, want nil", tc.suffix, err)
			}
			gotParent, err := got.Parent()
			if err != nil {
				t.Fatalf("Parent() error = %v, want nil", err)
			}
			wantParent, err := base.Parent()
			if err != nil {
				t.Fatalf("Parent() error = %v, want nil", err)
			}
			if gotParent != wantParent {
				t.Fatalf("WithSuffix(%q) parent = %q, want %q", tc.suffix, gotParent.String(), wantParent.String())
			}
			gotBase, err := got.Base()
			if err != nil {
				t.Fatalf("Base() error = %v, want nil", err)
			}
			if gotBase.String() != tc.want {
				t.Fatalf("WithSuffix(%q) base = %q, want %q", tc.suffix, gotBase.String(), tc.want)
			}
		})
	}
}

// TestWithSuffixRefusesAnUnvalidatedPath keeps the gate ahead of the naming.
func TestWithSuffixRefusesAnUnvalidatedPath(t *testing.T) {
	t.Parallel()

	if _, err := (core.AbsolutePath{}).WithSuffix(".lease"); err == nil {
		t.Fatalf("WithSuffix() on a zero path error = nil, want a refusal")
	}
}
