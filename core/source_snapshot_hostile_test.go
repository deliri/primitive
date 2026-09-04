package core_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestSourceSnapshotCanonicalBoundaryAndPreservedRefusal(t *testing.T) {
	t.Parallel()

	baseline := core.SourceSnapshot{Digest: core.SHA256Of([]byte("admitted source stream"))}
	canonical, canonicalErr := baseline.MarshalJSON()
	if canonicalErr != nil {
		t.Fatalf("SourceSnapshot.MarshalJSON() error = %v, want nil", canonicalErr)
	}

	cases := []struct {
		name    string
		input   string
		wantErr error
	}{
		{name: "canonical snapshot round trips", input: string(canonical)},
		{name: "empty token is not JSON", input: "", wantErr: core.ErrJSONContract},
		{name: "null cannot erase a snapshot", input: "null", wantErr: core.ErrJSONContract},
		{name: "object cannot impersonate nominal snapshot", input: `{}`, wantErr: core.ErrJSONContract},
		{name: "sha1 Git object width is rejected", input: `"` + strings.Repeat("a", 40) + `"`, wantErr: core.ErrJSONContract},
		{name: "one nibble below SHA256 width is rejected", input: `"` + strings.Repeat("a", 63) + `"`, wantErr: core.ErrJSONContract},
		{name: "one nibble above SHA256 width is rejected", input: `"` + strings.Repeat("a", 65) + `"`, wantErr: core.ErrJSONContract},
		{name: "uppercase SHA256 is noncanonical", input: `"` + strings.Repeat("A", 64) + `"`, wantErr: core.ErrJSONContract},
		{name: "nonhex SHA256 is rejected", input: `"` + strings.Repeat("g", 64) + `"`, wantErr: core.ErrJSONContract},
		{name: "trailing JSON is rejected", input: string(canonical) + " 0", wantErr: core.ErrJSONContract},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := baseline
			gotErr := got.UnmarshalJSON([]byte(tc.input))
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("SourceSnapshot.UnmarshalJSON(%q) error = %v, want %v", tc.input, gotErr, tc.wantErr)
			}
			if gotErr != nil && got != baseline {
				t.Fatalf("SourceSnapshot.UnmarshalJSON(rejected) receiver = %v, want preserved %v", got, baseline)
			}
			if gotErr == nil && got != baseline {
				t.Fatalf("SourceSnapshot.UnmarshalJSON(canonical) receiver = %v, want %v", got, baseline)
			}
		})
	}
}

func TestSourceSnapshotTextBoundaryIsNominalSHA256(t *testing.T) {
	t.Parallel()

	want := core.SourceSnapshot{Digest: core.SHA256Of([]byte("snapshot text boundary"))}
	got := core.SourceSnapshot{Digest: core.SHA256Of([]byte("preserved receiver"))}
	gotErr := got.UnmarshalText([]byte(want.String()))
	if gotErr != nil || got != want {
		t.Fatalf("SourceSnapshot.UnmarshalText(canonical) = (%v, %v), want (%v, nil)", got, gotErr, want)
	}

	before := got
	gotErr = got.UnmarshalText([]byte(strings.Repeat("a", 40)))
	if !errors.Is(gotErr, core.ErrPrimitiveContract) || got != before {
		t.Fatalf("SourceSnapshot.UnmarshalText(Git SHA1) = (%v, %v), want (%v, %v)", got, gotErr, before, core.ErrPrimitiveContract)
	}
}
