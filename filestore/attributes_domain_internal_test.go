package filestore

import (
	"errors"
	"fmt"
	"io/fs"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// Exhaust the complete permission field without depending on the host chmod
// representation. Public native observations are covered by Inspect's matrix.
func TestPermissionsCompleteFieldAndRefusalLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name     string
		value    Permissions
		wantMode fs.FileMode
		wantErr  error
	}{
		{name: "unobserved zero cannot become observed mode zero", wantErr: core.ErrFilestoreContract},
		{name: "unobserved residue cannot leak a permission bit", value: Permissions{value: 1}, wantErr: core.ErrFilestoreContract},
	}
	for mode := fs.FileMode(0); mode <= fs.ModePerm; mode++ {
		cases = append(cases, struct {
			name     string
			value    Permissions
			wantMode fs.FileMode
			wantErr  error
		}{name: fmt.Sprintf("observed permission field %#o retains every bit", mode), value: Permissions{value: mode, set: true}, wantMode: mode})
	}
	for bit := fs.ModePerm + 1; bit != 0; bit <<= 1 {
		cases = append(cases, struct {
			name     string
			value    Permissions
			wantMode fs.FileMode
			wantErr  error
		}{name: fmt.Sprintf("non-permission bit %#x cannot masquerade as permissions", bit), value: Permissions{value: bit | fs.ModePerm, set: true}, wantErr: core.ErrFilestoreContract})
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotValidation := tc.value.Validate()
			gotMode, modeErr := tc.value.FileMode()
			gotBits, bitsErr := tc.value.Bits()
			for _, gotErr := range []error{gotValidation, modeErr, bitsErr} {
				if !errors.Is(gotErr, tc.wantErr) || errors.Is(gotErr, core.ErrFilestoreSource) || errors.Is(gotErr, core.ErrFilestoreActivation) {
					t.Fatalf("permission projection = %v, want pure %v", gotErr, tc.wantErr)
				}
			}
			if gotMode != tc.wantMode || gotBits != uint32(tc.wantMode) || tc.value.IsSet() != tc.value.set {
				t.Fatalf("projections = (%v,%d,%t), want (%v,%d,%t)", gotMode, gotBits, tc.value.IsSet(), tc.wantMode, uint32(tc.wantMode), tc.value.set)
			}
			// String is diagnostic Go notation; it does not serialize a contract.
			if tc.value.set && tc.value.String() != tc.value.value.String() {
				t.Fatalf("diagnostic = %q, want Go %q", tc.value.String(), tc.value.value.String())
			}
			if !tc.value.set && tc.value.String() == (fs.FileMode(0)).String() {
				t.Fatalf("unobserved diagnostic = %q, want distinct from observed zero %q", tc.value.String(), (fs.FileMode(0)).String())
			}
		})
	}
}

func TestOwnershipUnsignedCoordinatesLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name             string
		value            Ownership
		wantUID, wantGID uint32
		wantErr          error
	}{
		{name: "unobserved zero cannot claim root ownership", wantErr: core.ErrFilestoreContract},
		{name: "unobserved UID residue cannot escape", value: Ownership{uid: 1}, wantErr: core.ErrFilestoreContract},
		{name: "unobserved GID residue cannot escape", value: Ownership{gid: math.MaxUint32}, wantErr: core.ErrFilestoreContract},
		{name: "observed zero identifiers remain real coordinates", value: Ownership{set: true}},
		{name: "UID and GID cannot be transposed", value: Ownership{uid: 1, gid: 2, set: true}, wantUID: 1, wantGID: 2},
		{name: "maximum signed coordinates remain unsigned facts", value: Ownership{uid: math.MaxInt32, gid: math.MaxInt32, set: true}, wantUID: math.MaxInt32, wantGID: math.MaxInt32},
		{name: "UID above signed range cannot narrow", value: Ownership{uid: math.MaxInt32 + 1, set: true}, wantUID: math.MaxInt32 + 1},
		{name: "GID above signed range cannot narrow", value: Ownership{gid: math.MaxInt32 + 1, set: true}, wantGID: math.MaxInt32 + 1},
		{name: "maximum UID cannot become missing ownership", value: Ownership{uid: math.MaxUint32, set: true}, wantUID: math.MaxUint32},
		{name: "maximum GID cannot become missing ownership", value: Ownership{gid: math.MaxUint32, set: true}, wantGID: math.MaxUint32},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotValidation := tc.value.Validate()
			uid, uidErr := tc.value.UID()
			gid, gidErr := tc.value.GID()
			for _, gotErr := range []error{gotValidation, uidErr, gidErr} {
				if !errors.Is(gotErr, tc.wantErr) || errors.Is(gotErr, core.ErrFilestoreSource) || errors.Is(gotErr, core.ErrFilestoreActivation) {
					t.Fatalf("ownership projection = %v, want pure %v", gotErr, tc.wantErr)
				}
			}
			if uid != tc.wantUID || gid != tc.wantGID || tc.value.IsSet() != tc.value.set {
				t.Fatalf("owner = (%d,%d,%t), want (%d,%d,%t)", uid, gid, tc.value.IsSet(), tc.wantUID, tc.wantGID, tc.value.set)
			}
		})
	}
}
