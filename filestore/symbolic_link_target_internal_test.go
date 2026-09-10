package filestore

import (
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// Finite regression fixture, not an admitted observation ceiling.
const symbolicLinkTargetFixtureBytes = 64 << 10

func TestSymbolicLinkTargetOpaqueByteBoundaryLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, input, want string
		wantErr           error
	}{
		{name: "zero observation has no target", wantErr: core.ErrFilestoreContract},
		{name: "one opaque byte survives without path interpretation", input: "?", want: "?"},
		{name: "one below former quota remains exact", input: strings.Repeat("x", symbolicLinkTargetFixtureBytes-1), want: strings.Repeat("x", symbolicLinkTargetFixtureBytes-1)},
		{name: "former quota remains exact", input: strings.Repeat("x", symbolicLinkTargetFixtureBytes), want: strings.Repeat("x", symbolicLinkTargetFixtureBytes)},
		{name: "one above former quota preserves the complete target", input: strings.Repeat("x", symbolicLinkTargetFixtureBytes+1), want: strings.Repeat("x", symbolicLinkTargetFixtureBytes+1)},
		{name: "embedded NUL cannot project truncated target", input: "x\x00y", wantErr: core.ErrFilestoreContract},
		{name: "invalid UTF8 is opaque native material", input: "\xff\xfe", want: "\xff\xfe"},
		{name: "multibyte target beyond former quota preserves exact bytes", input: strings.Repeat("é", symbolicLinkTargetFixtureBytes/2) + "x", want: strings.Repeat("é", symbolicLinkTargetFixtureBytes/2) + "x"},
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
