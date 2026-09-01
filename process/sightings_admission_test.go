package process

import "testing"

// TestSnapshotSightingAdmitsOnlyActionableRows pins exactly which snapshot rows
// become sightings and which the walk drops, on every host rather than only
// where the Toolhelp snapshot exists. A process a caller could signal or probe
// carries a positive identity and a one-component image and is admitted; a row
// outside either domain is reported as not a sighting, never surfaced. The
// spaces case records that an image component may hold them, so the identity
// domain, not the image, is what drops the idle pseudo-process.
func TestSnapshotSightingAdmitsOnlyActionableRows(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		image    string
		identity int32
		wantOK   bool
	}{
		{name: "ordinary user process is a sighting", identity: 4321, image: "worker.exe", wantOK: true},
		{name: "identity one is a sighting", identity: 1, image: "init", wantOK: true},
		{name: "single-character image is a sighting", identity: 2, image: "a", wantOK: true},
		{name: "image with spaces is a sighting", identity: 5, image: "Some Service Host", wantOK: true},
		{name: "maximum admitted identity is a sighting", identity: 2147483647, image: "svchost.exe", wantOK: true},
		{name: "idle pseudo-process at identity zero is dropped", identity: 0, image: "System", wantOK: false},
		{name: "negative identity is dropped", identity: -1, image: "svchost.exe", wantOK: false},
		{name: "minimum signed identity from an overflowed pid is dropped", identity: -2147483648, image: "svchost.exe", wantOK: false},
		{name: "empty image is dropped", identity: 100, image: "", wantOK: false},
		{name: "current-directory image is dropped", identity: 100, image: ".", wantOK: false},
		{name: "parent-directory image is dropped", identity: 100, image: "..", wantOK: false},
		{name: "path-separated image is dropped", identity: 100, image: "bin/sh", wantOK: false},
		{name: "NUL byte in image is dropped", identity: 100, image: "a\x00b", wantOK: false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, gotOK := snapshotSighting(ProcessIdentity(tc.identity), tc.image)
			if gotOK != tc.wantOK {
				t.Fatalf("snapshotSighting(%d, %q) ok = %t, want %t", tc.identity, tc.image, gotOK, tc.wantOK)
			}
			if !tc.wantOK {
				return
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("admitted snapshotSighting(%d, %q).Validate() error = %v, want nil", tc.identity, tc.image, err)
			}
			if got.Image.String() != tc.image {
				t.Fatalf("snapshotSighting(%d, %q).Image = %q, want the recorded image", tc.identity, tc.image, got.Image.String())
			}
		})
	}
}
