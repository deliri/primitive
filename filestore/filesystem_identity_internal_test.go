package filestore

import (
	"errors"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestFilesystemIdentityProjectionLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		raw      uint64
		observed bool
		want     uint64
		wantErr  error
	}{
		{name: "unobserved zero cannot become a host identity", wantErr: core.ErrFilestoreContract},
		{name: "unobserved numeric residue cannot escape", raw: math.MaxUint64, wantErr: core.ErrFilestoreContract},
		{name: "observed zero is a native coordinate without invented policy", observed: true},
		{name: "observed low bit survives projection", raw: 1, observed: true, want: 1},
		{name: "last uint32 coordinate is not treated as a sentinel", raw: math.MaxUint32, observed: true, want: math.MaxUint32},
		{name: "first coordinate above uint32 is not narrowed", raw: uint64(math.MaxUint32) + 1, observed: true, want: uint64(math.MaxUint32) + 1},
		{name: "largest native coordinate preserves all bits", raw: math.MaxUint64, observed: true, want: math.MaxUint64},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			identity := FilesystemIdentity{value: tc.raw}
			if tc.observed {
				identity = newFilesystemIdentity(tc.raw)
			}
			before := identity
			got, gotErr := identity.Uint64()
			validationErr := identity.Validate()
			if got != tc.want || !errors.Is(gotErr, tc.wantErr) || !errors.Is(validationErr, tc.wantErr) || errors.Is(gotErr, core.ErrFilestoreSource) || identity != before {
				t.Fatalf("identity projection = (%d,%v,%v,%+v), want (%d,%v) without source error or mutation", got, gotErr, validationErr, identity, tc.want, tc.wantErr)
			}
		})
	}
}
