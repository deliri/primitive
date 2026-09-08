package filestore

import (
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestSymbolicLinkTargetOpaqueByteBoundaryLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, input, want string
		wantErr           error
	}{
		{name: "zero observation has no target", wantErr: core.ErrFilestoreContract},
		{name: "one opaque byte survives without path interpretation", input: "?", want: "?"},
		{name: "one below byte ceiling remains exact", input: strings.Repeat("x", SymbolicLinkTargetMaximumBytes-1), want: strings.Repeat("x", SymbolicLinkTargetMaximumBytes-1)},
		{name: "exact byte ceiling remains exact", input: strings.Repeat("x", SymbolicLinkTargetMaximumBytes), want: strings.Repeat("x", SymbolicLinkTargetMaximumBytes)},
		{name: "one above byte ceiling cannot project a prefix", input: strings.Repeat("x", SymbolicLinkTargetMaximumBytes+1), wantErr: core.ErrFilestoreContract},
		{name: "embedded NUL cannot project truncated target", input: "x\x00y", wantErr: core.ErrFilestoreContract},
		{name: "invalid UTF8 is opaque native material", input: "\xff\xfe", want: "\xff\xfe"},
		{name: "multibyte text is bounded in bytes not runes", input: strings.Repeat("é", SymbolicLinkTargetMaximumBytes/2) + "x", wantErr: core.ErrFilestoreContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			target := SymbolicLinkTarget{value: tc.input}
			gotErr := target.Validate()
			if !errors.Is(gotErr, tc.wantErr) || errors.Is(gotErr, core.ErrFilestoreSource) {
				t.Fatalf("Validate() = %v, want %v without source classification", gotErr, tc.wantErr)
			}
			if got := target.String(); got != tc.want {
				t.Fatalf("String() = %q, want %q", got, tc.want)
			}
			if target.value != tc.input {
				t.Fatalf("retained observation = %q, want original %q", target.value, tc.input)
			}
		})
	}
}
