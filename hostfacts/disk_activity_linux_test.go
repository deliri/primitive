//go:build linux

package hostfacts

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestDiskActivityWholeDeviceMarkerLayerTriad(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name    string
		marker  bool
		absent  bool
		wantErr error
	}{
		{name: "no partition marker preserves whole device"},
		{name: "directory cannot impersonate partition marker", marker: true, wantErr: core.ErrHostFactsObservation},
		{name: "missing device preserves native absence", absent: true, wantErr: fs.ErrNotExist},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			if row.marker {
				if err := os.Mkdir(filepath.Join(directory, "partition"), 0700); err != nil {
					t.Fatal(err)
				}
			}
			if row.absent {
				directory = filepath.Join(directory, "absent")
			}
			device := mustAbsolutePathForHostfactsTest(t, directory)
			got, err := activityWholeDevice(t.Context(), device)
			if row.wantErr != nil {
				if !errors.Is(err, row.wantErr) || got != (core.AbsolutePath{}) {
					t.Fatalf("whole device=%v/%v, want unset/%v", got, err, row.wantErr)
				}
				return
			}
			if err != nil || got != device {
				t.Fatalf("whole device=%v/%v, want %v/nil", got, err, device)
			}
		})
	}
}
