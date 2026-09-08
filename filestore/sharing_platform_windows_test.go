//go:build windows

package filestore_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

type sharingNativeShape uint8

const (
	sharingNativeFile sharingNativeShape = iota
	sharingNativeHeld
	sharingNativeReleased
	sharingNativeAbsent
	sharingNativeMissingParent
	sharingNativeDirectory
)

func TestSharingWindowsNativeProbeLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		shape   sharingNativeShape
		want    filestore.Sharing
		wantErr error
	}{
		{name: "closed regular file permits zero share read", shape: sharingNativeFile, want: filestore.SharingAvailable},
		{name: "same process read handle conflicts with zero share", shape: sharingNativeHeld, want: filestore.SharingHeld},
		{name: "closing conflicting handle restores availability", shape: sharingNativeReleased, want: filestore.SharingAvailable},
		{name: "missing leaf cannot be called held", shape: sharingNativeAbsent, want: filestore.SharingUnknown, wantErr: syscall.ERROR_FILE_NOT_FOUND},
		{name: "missing parent retains distinct native path refusal", shape: sharingNativeMissingParent, want: filestore.SharingUnknown, wantErr: syscall.ERROR_PATH_NOT_FOUND},
		{name: "directory without backup semantics cannot be called held", shape: sharingNativeDirectory, want: filestore.SharingUnknown, wantErr: syscall.ERROR_ACCESS_DENIED},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			name := filepath.Join(directory, "entry")
			payload := []byte{0, 255, 1, 127}
			if err := os.WriteFile(name, payload, 0o600); err != nil {
				t.Fatal(err)
			}
			before, err := os.Stat(name)
			if err != nil {
				t.Fatal(err)
			}
			probe := name
			var file *os.File
			switch tc.shape {
			case sharingNativeFile:
			case sharingNativeHeld, sharingNativeReleased:
				file, err = os.Open(name)
				if err != nil {
					t.Fatal(err)
				}
				if tc.shape == sharingNativeReleased {
					if err := file.Close(); err != nil {
						t.Fatal(err)
					}
					file = nil
				} else {
					t.Cleanup(func() {
						if err := file.Close(); err != nil {
							t.Error(err)
						}
					})
				}
			case sharingNativeAbsent:
				probe = filepath.Join(directory, "absent")
			case sharingNativeMissingParent:
				probe = filepath.Join(directory, "missing", "child")
			case sharingNativeDirectory:
				probe = directory
			default:
				t.Fatalf("shape = %v, want declared fixture", tc.shape)
			}
			native, nativeErr := nativeSharingProbe(probe)
			if native != tc.want || !errors.Is(nativeErr, tc.wantErr) {
				t.Fatalf("native probe = (%v,%v), want (%v,%v)", native, nativeErr, tc.want, tc.wantErr)
			}
			got, gotErr := filestore.ObserveSharing(t.Context(), mustAbsolute(t, probe))
			if got != tc.want || !errors.Is(gotErr, tc.wantErr) || errors.Is(gotErr, core.ErrFilestoreSource) != (tc.wantErr != nil) {
				t.Fatalf("ObserveSharing = (%v,%v), want (%v,%v) with exact Source classification", got, gotErr, tc.want, tc.wantErr)
			}
			// A second exclusive native probe catches a successful probe leaking its
			// handle. A held row must still be held; errors retain their native identity.
			afterProbe, afterErr := nativeSharingProbe(probe)
			if afterProbe != native || !errors.Is(afterErr, nativeErr) {
				t.Fatalf("post-observation native probe = (%v,%v), want (%v,%v)", afterProbe, afterErr, native, nativeErr)
			}
			if file != nil {
				data := make([]byte, len(payload))
				n, err := io.ReadFull(file, data)
				if err != nil || n != len(payload) || !bytes.Equal(data, payload) {
					t.Fatalf("held read = (%v,%d,%v), want untouched cursor", data, n, err)
				}
			}
			after, err := os.Stat(name)
			if err != nil || !os.SameFile(before, after) || before.Mode() != after.Mode() || before.ModTime().UnixNano() != after.ModTime().UnixNano() {
				t.Fatalf("entry = (%v,%v), want original metadata", after, err)
			}
			data, err := os.ReadFile(name)
			if err != nil || !bytes.Equal(data, payload) {
				t.Fatalf("entry bytes = (%v,%v), want %v", data, err, payload)
			}
			entries, err := os.ReadDir(directory)
			if err != nil || len(entries) != 1 || entries[0].Name() != "entry" {
				t.Fatalf("namespace = (%v,%v), want only original entry", entries, err)
			}
		})
	}
}
