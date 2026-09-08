package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestInspectAdmissionLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                            string
		unsetPath, nilContext, canceled bool
		wantErr                         error
	}{
		{name: "active ingress observes exact binary-file extent"},
		{name: "unset path cannot select a working directory", unsetPath: true, wantErr: core.ErrFilestoreContract},
		{name: "nil context refuses a valid path", nilContext: true, wantErr: core.ErrNilContext},
		{name: "nil context precedes even an unset path", nilContext: true, unsetPath: true, wantErr: core.ErrNilContext},
		{name: "canceled context cannot produce an observation", canceled: true, wantErr: context.Canceled},
		{name: "cancellation precedes path admission", canceled: true, unsetPath: true, wantErr: context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			name := filepath.Join(directory, "entry")
			payload := []byte{0, 255, 1}
			if err := os.WriteFile(name, payload, 0o600); err != nil {
				t.Fatal(err)
			}
			before, err := os.Lstat(name)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.canceled {
				cancel()
			}
			if tc.nilContext {
				ctx = nil
			}
			path := mustAbsolute(t, name)
			if tc.unsetPath {
				path = core.AbsolutePath{}
			}
			got, gotErr := filestore.Inspect(ctx, path)
			if !errors.Is(gotErr, tc.wantErr) || errors.Is(gotErr, core.ErrFilestoreSource) || errors.Is(gotErr, core.ErrFilestoreActivation) {
				t.Fatalf("Inspect = (%v,%v), want pure %v", got, gotErr, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (filestore.Inspection{}) || !errors.Is(got.Validate(), core.ErrFilestoreContract) {
					t.Fatalf("refused observation = %v, want zero invalid value", got)
				}
			} else {
				kind, kindErr := got.Kind()
				size, sizeErr := got.SizeBytes()
				if kindErr != nil || sizeErr != nil || kind != filestore.PathKindRegularFile || size.Uint64() != uint64(len(payload)) {
					t.Fatalf("observation = (%v,%v,%v,%v), want exact regular-file extent", kind, size, kindErr, sizeErr)
				}
			}
			after, err := os.Lstat(name)
			if err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(name)
			if err != nil || !bytes.Equal(data, payload) || !os.SameFile(before, after) || before.Mode() != after.Mode() || before.Size() != after.Size() || !before.ModTime().Equal(after.ModTime()) {
				t.Fatalf("entry = (%v,%v,%v), want unchanged bytes and metadata", after, data, err)
			}
			entries, err := os.ReadDir(directory)
			if err != nil || len(entries) != 1 || entries[0].Name() != "entry" {
				t.Fatalf("namespace = (%v,%v), want only original entry", entries, err)
			}
		})
	}
}

func mustAbsolute(t *testing.T, path string) core.AbsolutePath {
	t.Helper()
	value, err := core.ParseAbsolutePath(path)
	if err != nil {
		t.Fatal(err)
	}
	return value
}
