package release

import (
	"context"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestInspectionStandingLayerTriadRefusesLostPathCustody(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                    string
		remove, replace, cancel bool
		wantErr                 error
	}{
		{name: "unchanged path retains held identity"},
		{name: "removed name cannot retain standing", remove: true, wantErr: core.ErrReleaseContract},
		{name: "same-sized replacement cannot impersonate held identity", remove: true, replace: true, wantErr: core.ErrReleaseContract},
		{name: "canceled observation cannot claim unchanged standing", cancel: true, wantErr: context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			fixture := inspectionContentRequest{Directory: directory, Content: []byte{1, 2, 3}, Extent: 3, Mode: 0o700}
			path := writeInspectionContent(t, fixture)
			location, err := filestore.OpenParent(t.Context(), path)
			if err != nil {
				t.Fatalf("OpenParent fixture error = %v, want nil", err)
			}
			t.Cleanup(func() {
				if err := location.Root.Close(); err != nil {
					t.Errorf("parent Close error = %v, want nil", err)
				}
			})
			held, err := filestore.OpenRead(t.Context(), filestore.ReadHandleRequest{Location: location})
			if err != nil {
				t.Fatalf("OpenRead fixture error = %v, want nil", err)
			}
			t.Cleanup(func() {
				if err := held.Close(); err != nil {
					t.Errorf("held Close error = %v, want nil", err)
				}
			})
			if tc.remove {
				if err := filestore.Remove(t.Context(), filestore.RemovalRequest{Location: location}); err != nil {
					t.Fatalf("Remove fixture error = %v, want nil", err)
				}
			}
			if tc.replace {
				replacement := writeInspectionContent(t, fixture)
				if replacement != path {
					t.Fatalf("replacement path = %v, want original %v", replacement, path)
				}
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.cancel {
				cancel()
			}
			err = validateInspectionStanding(ctx, held, path)
			if !errors.Is(err, tc.wantErr) || (tc.wantErr != nil && !errors.Is(err, core.ErrReleaseContract)) {
				t.Fatalf("standing error = %v, want %v with Release identity on refusal", err, tc.wantErr)
			}
			var bytes [3]byte
			if n, err := held.ReadAt(bytes[:], 0); err != nil || n != len(bytes) || bytes != [3]byte{1, 2, 3} {
				t.Fatalf("held bytes after standing = (%v, %d, %v), want original bytes and no handle consumption", bytes, n, err)
			}
		})
	}
}
