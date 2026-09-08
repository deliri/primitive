package release

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/keygen"
)

func TestMaterialRequestConstructionNeverReturnsPartialIntent(t *testing.T) {
	t.Parallel()
	var nonceBytes [core.SHA256DigestBytes]byte
	nonceBytes[0] = 1
	nonce, err := controlwire.NewRequestNonce(nonceBytes)
	if err != nil {
		t.Fatalf("NewRequestNonce() error = %v, want nil", err)
	}
	commit, err := core.ParseBuildCommit("0123456789abcdef0123456789abcdef01234567")
	if err != nil {
		t.Fatalf("ParseBuildCommit() error = %v, want nil", err)
	}
	base := MaterialRequestInput{Offering: releaseOffering(t, 2), Version: core.NewReleaseVersion(2026, 1, 1), Commit: commit, Nonce: nonce}
	cases := []struct {
		name    string
		mutate  func(*MaterialRequestInput)
		wantErr error
	}{
		{name: "complete intent retains exact input fields", mutate: func(*MaterialRequestInput) {}},
		{name: "absent offering cannot retain other valid fields", mutate: func(r *MaterialRequestInput) { r.Offering = core.Offering{} }, wantErr: core.ErrReleaseContract},
		{name: "absent version cannot retain other valid fields", mutate: func(r *MaterialRequestInput) { r.Version = core.ReleaseVersion{} }, wantErr: core.ErrReleaseContract},
		{name: "absent commit cannot retain other valid fields", mutate: func(r *MaterialRequestInput) { r.Commit = core.BuildCommit{} }, wantErr: core.ErrReleaseContract},
		{name: "absent nonce cannot retain other valid fields", mutate: func(r *MaterialRequestInput) { r.Nonce = controlwire.RequestNonce{} }, wantErr: core.ErrReleaseContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			input := base
			tc.mutate(&input)
			got, gotErr := NewMaterialRequest(input)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("NewMaterialRequest() error = %v, want %v", gotErr, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (MaterialRequest{}) {
					t.Fatalf("NewMaterialRequest(refused) = %v, want exact zero", got)
				}
			} else if got.Validate() != nil || got.Offering != input.Offering || got.Version != input.Version || got.Commit != input.Commit || got.Nonce != input.Nonce {
				t.Fatalf("NewMaterialRequest() = %v, want exact validated intent %v", got, input)
			}
			if tc.wantErr == nil {
				route, routeErr := got.ControlRoute()
				if routeErr != nil || route.Offering() != input.Offering || route.Family() != controlwire.RouteFamilyReleaseMaterials || got.ControlNonce() != input.Nonce {
					t.Fatalf("material control projection = (%v, %v, %v), want offering %v, release-material family and nonce %v", route, got.ControlNonce(), routeErr, input.Offering, input.Nonce)
				}
			}
		})
	}
}

func TestReleaseSigningSeedJSONBoundsPreservePreviousCustody(t *testing.T) {
	t.Parallel()
	var fixed [keygen.SeedSize]byte
	fixed[0] = 1
	seed, err := NewReleaseSigningSeed(fixed)
	if err != nil {
		t.Fatalf("NewReleaseSigningSeed() error = %v, want nil", err)
	}
	defer func() {
		if err := seed.Destroy(); err != nil {
			t.Errorf("ReleaseSigningSeed.Destroy() error = %v, want nil", err)
		}
	}()
	canonical, err := seed.MarshalJSON()
	if err != nil {
		t.Fatalf("ReleaseSigningSeed.MarshalJSON() error = %v, want nil", err)
	}
	cases := []struct {
		name    string
		extent  int
		wantErr error
	}{
		{name: "one below document ceiling retains the exact seed", extent: documentExtentMaximum - 1},
		{name: "exact document ceiling retains the exact seed", extent: documentExtentMaximum},
		{name: "one above document ceiling preserves previous custody", extent: documentExtentMaximum + 1, wantErr: core.ErrJSONContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := NewReleaseSigningSeed(fixed)
			if err != nil {
				t.Fatalf("NewReleaseSigningSeed() error = %v, want nil", err)
			}
			defer func() {
				if err := got.Destroy(); err != nil {
					t.Errorf("ReleaseSigningSeed.Destroy() error = %v, want nil", err)
				}
			}()
			before := got
			input := make([]byte, tc.extent)
			copy(input, canonical)
			for i := len(canonical); i < len(input); i++ {
				input[i] = ' '
			}
			gotErr := got.UnmarshalJSON(input)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("ReleaseSigningSeed.UnmarshalJSON(%d bytes) error = %v, want %v", tc.extent, gotErr, tc.wantErr)
			}
			if tc.wantErr != nil && (!errors.Is(gotErr, core.ErrReleaseContract) || got != before || before.Validate() != nil) {
				t.Fatalf("refused seed = (%v, %v), want same active custody and Release refusal", got, gotErr)
			}
			encoded, err := got.MarshalJSON()
			if err != nil || !bytes.Equal(encoded, canonical) {
				t.Fatalf("seed projection = (%q, %v), want original bytes %q", encoded, err, canonical)
			}
			if tc.wantErr == nil && !errors.Is(before.Validate(), core.ErrReleaseContract) {
				t.Fatalf("replaced custody Validate() = %v, want refusal", before.Validate())
			}
		})
	}
}

// The JSON carrier must not own live secret custody while sibling fields are
// still being decoded. Changing it back to *ReleaseSigningSeed breaks the build.
var _ *string = materialResponseWire{}.ReleaseSigningSeed
