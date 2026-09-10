package filestore_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestDirectoryComponentExtentUsesNativeAdmissionLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct{ name, component string }{
		{name: "254 ASCII bytes retain native admission", component: strings.Repeat("a", 254)},
		{name: "255 ASCII bytes retain native admission", component: strings.Repeat("a", 255)},
		{name: "256 ASCII bytes reach native representability", component: strings.Repeat("a", 256)},
		{name: "256 UTF-8 bytes keep the native character-unit decision", component: strings.Repeat("é", 128)},
		{name: "4097 bytes reach the native filename refusal", component: strings.Repeat("a", 4097)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			nativeDirectory := filepath.Join(directory, "native")
			ownedDirectory := filepath.Join(directory, "owned")
			for _, name := range []string{nativeDirectory, ownedDirectory} {
				if err := os.Mkdir(name, 0700); err != nil {
					t.Fatal(err)
				}
			}
			nativeErr := os.Mkdir(filepath.Join(nativeDirectory, tc.component), 0700)
			component, err := core.ParsePathComponent(tc.component)
			if err != nil || component.String() != tc.component {
				t.Fatalf("lexical component = %d bytes/%v, want exact %d bytes", len(component.String()), err, len(tc.component))
			}
			path := mustRelativePath(t, component.String())
			root := requireTestRoot(t, ownedDirectory)
			gotErr := filestore.EnsureDirectory(t.Context(), filestore.DirectoryRequest{Location: filestore.Location{Root: root, Path: path}, Mode: 0700})
			if nativeErr == nil {
				info, err := os.Stat(filepath.Join(ownedDirectory, tc.component))
				if gotErr != nil || err != nil || !info.IsDir() {
					t.Fatalf("owned directory = %v/%v, want native success", gotErr, err)
				}
				return
			}
			var nativePathErr *fs.PathError
			var gotPathErr *fs.PathError
			if !errors.As(nativeErr, &nativePathErr) || !errors.As(gotErr, &gotPathErr) || !errors.Is(gotErr, nativePathErr.Err) {
				t.Fatalf("owned refusal = %v, want native path identity %v", gotErr, nativeErr)
			}
			entries, err := os.ReadDir(ownedDirectory)
			if err != nil || len(entries) != 0 {
				t.Fatalf("refused namespace = %d entries/%v, want empty", len(entries), err)
			}
		})
	}
}
