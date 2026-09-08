package process

import (
	"github.com/deliri/primitive/v2026/core"
	"testing"
)

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
		identity uint32
		wantOK   bool
	}{
		{name: "ordinary user process is a sighting", identity: 4321, image: "worker.exe", wantOK: true},
		{name: "identity one is a sighting", identity: 1, image: "init", wantOK: true},
		{name: "single-character image is a sighting", identity: 2, image: "a", wantOK: true},
		{name: "image with spaces is a sighting", identity: 5, image: "Some Service Host", wantOK: true},
		{name: "maximum signed identity is a sighting", identity: 2147483647, image: "svchost.exe", wantOK: true},
		{name: "one above the signed identity domain is a sighting", identity: 2147483648, image: "svchost.exe", wantOK: true},
		{name: "maximum Windows identity is a sighting", identity: 4294967295, image: "svchost.exe", wantOK: true},
		{name: "idle pseudo-process at identity zero is dropped", identity: 0, image: "System", wantOK: false},
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
				if got != (ProcessSighting{}) {
					t.Fatalf("dropped row exposed partial sighting: %+v", got)
				}
				return
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("admitted snapshotSighting(%d, %q).Validate() error = %v, want nil", tc.identity, tc.image, err)
			}
			if got.Identity != ProcessIdentity(tc.identity) || got.Image.String() != tc.image {
				t.Fatalf("snapshotSighting(%d, %q).Image = %q, want the recorded image", tc.identity, tc.image, got.Image.String())
			}
		})
	}
}

// Toolhelp supplies this raw leaf on Windows. Fuzzing the leaf on every host
// does not claim execution of the Windows snapshot API.
func FuzzSnapshotSightingNativeIngress(f *testing.F) {
	image, err := core.ParsePathComponent("fixture.exe")
	if err != nil {
		f.Fatal(err)
	}
	seed := ProcessSighting{Identity: 1, Image: image}
	if err := seed.Validate(); err != nil {
		f.Fatal(err)
	}
	f.Add(uint32(seed.Identity), seed.Image.String())
	f.Add(uint32(0), image.String())
	f.Add(^uint32(0), image.String())
	f.Add(uint32(1), "bad\x00image")
	f.Add(uint32(1), "../image")
	f.Fuzz(func(t *testing.T, identity uint32, raw string) {
		image, imageErr := core.ParsePathComponent(raw)
		want := identity != 0 && imageErr == nil
		got, accepted := snapshotSighting(ProcessIdentity(identity), raw)
		if accepted != want {
			t.Fatalf("snapshot admission=%t, want %t", accepted, want)
		}
		if !want {
			if got != (ProcessSighting{}) {
				t.Fatalf("refused snapshot exposed facts: %+v", got)
			}
			return
		}
		if got.Validate() != nil || got.Identity != ProcessIdentity(identity) || got.Image != image || got.Image.String() != raw {
			t.Fatalf("snapshot changed native facts: %+v", got)
		}
	})
}
