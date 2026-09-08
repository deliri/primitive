package gotoolchain

import (
	"errors"
	"go/types"
	"io"
	"os"
	"path"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/gomodule"
)

// Real cmd/go owns the vendor producer and build oracle. The capability must
// return the same defining object, or preserve compiler refusal as partial.
func TestCompilerVendorSelectionLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                                     string
		removeOriginal, breakSelected, removeUse bool
		wantObject                               bool
		wantErr                                  error
	}{
		{name: "selected vendor survives removal of original replacement", removeOriginal: true, wantObject: true},
		{name: "broken vendor cannot borrow a healthy replacement export", breakSelected: true, wantErr: core.ErrGoToolchainOutput},
		{name: "unused broken vendor creates no compiler reference", breakSelected: true, removeUse: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			root, err := core.ParseAbsolutePath(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			held, err := filestore.OpenRoot(t.Context(), root)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := held.Close(); err != nil {
					t.Error(err)
				}
			})
			const imported = "example.test/vendor-selected"
			const owner = "example.test/vendor-owner"
			const selectedSource = "vendor/" + imported + "/value.go"
			writeVendorCompilerFixture(t, held, "dependency/go.mod", "module "+imported+"\n\ngo 1.27.1\n")
			writeVendorCompilerFixture(t, held, "dependency/value.go", "package selected;func Value()int{return 7}")
			writeVendorCompilerFixture(t, held, "project/go.mod", "module "+owner+"\n\ngo 1.27.1\nrequire "+imported+" v0.0.0\nreplace "+imported+" => ../dependency\n")
			writeVendorCompilerFixture(t, held, "project/use.go", "package owner;import selected \""+imported+"\";func Use()int{return selected.Value()}")
			limits, err := DefaultLimits()
			if err != nil {
				t.Fatal(err)
			}
			capability, err := Open(t.Context(), Configuration{Workspace: WorkspaceModeDisabled, Limits: limits})
			if err != nil {
				t.Fatal(err)
			}
			project, err := root.ResolveText("project")
			if err != nil {
				t.Fatal(err)
			}
			if _, err := capability.executeTo(t.Context(), project, io.Discard, "mod", "vendor"); err != nil {
				t.Fatal(err)
			}
			if tc.removeOriginal {
				relative, err := core.ParseRelativePath("dependency")
				if err != nil {
					t.Fatal(err)
				}
				if err := filestore.RemoveTree(t.Context(), filestore.TreeRemovalRequest{Location: filestore.Location{Root: held, Path: relative}}); err != nil {
					t.Fatal(err)
				}
			}
			if tc.breakSelected {
				writeVendorCompilerFixture(t, held, "project/"+selectedSource, "package selected;func Value()int{return Missing}")
			}
			if tc.removeUse {
				writeVendorCompilerFixture(t, held, "project/use.go", "package owner;func Use()int{return 7}")
			}
			_, buildErr := capability.executeTo(t.Context(), project, io.Discard, "build", ".")
			var wantBuildErr error
			if tc.wantErr != nil {
				wantBuildErr = core.ErrGoToolchainExecution
			}
			if !errors.Is(buildErr, wantBuildErr) {
				t.Fatalf("independent Go build=%v, want %v", buildErr, wantBuildErr)
			}
			pkg, err := gomodule.ParseImportPath(owner)
			if err != nil {
				t.Fatal(err)
			}
			got, err := capability.AnalyzePackage(t.Context(), AnalysisRequest{WorkingDirectory: project, Package: pkg})
			if !errors.Is(err, tc.wantErr) || got.Incomplete != (tc.wantErr != nil) {
				t.Fatalf("AnalyzePackage incomplete=%t error=%v, want partial=%t error=%v", got.Incomplete, err, tc.wantErr != nil, tc.wantErr)
			}
			if err := got.Validate(); err != nil {
				t.Fatal(err)
			}
			found := false
			for _, unit := range got.Units {
				for _, object := range unit.TypesInfo.Uses {
					fn, ok := object.(*types.Func)
					if ok && fn.Pkg() != nil && fn.Pkg().Path() == imported && fn.Name() == "Value" {
						found = true
					}
				}
			}
			if found != tc.wantObject {
				t.Fatalf("defining vendor object=%t, want %t", found, tc.wantObject)
			}
			if len(got.Metadata) != 1 || len(got.Units) != 1 {
				t.Fatalf("metadata=%d units=%d, want exact requested package", len(got.Metadata), len(got.Units))
			}
		})
	}
}

func writeVendorCompilerFixture(t *testing.T, root *os.Root, name, content string) {
	t.Helper()
	parent, err := core.ParseRelativePath(path.Dir(name))
	if err != nil {
		t.Fatal(err)
	}
	if err := filestore.EnsureScratchDirectory(t.Context(), filestore.DirectoryRequest{Location: filestore.Location{Root: root, Path: parent}, Mode: 0o700}); err != nil {
		t.Fatal(err)
	}
	relative, err := core.ParseRelativePath(name)
	if err != nil {
		t.Fatal(err)
	}
	location := filestore.Location{Root: root, Path: relative}
	if err := filestore.Remove(t.Context(), filestore.RemovalRequest{Location: location}); err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	file, err := filestore.OpenScratch(t.Context(), filestore.ScratchRequest{Location: location, Mode: 0o600})
	if err != nil {
		t.Fatal(err)
	}
	n, writeErr := io.WriteString(file, content)
	if err := errors.Join(writeErr, file.Close()); err != nil || n != len(content) {
		t.Fatalf("fixture write=%d/%v, want %d bytes", n, err, len(content))
	}
}
