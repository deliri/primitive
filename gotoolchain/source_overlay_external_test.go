package gotoolchain_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
	"github.com/deliri/primitive/v2026/gotoolchain"
)

func TestSourceOverlayCompilerBoundaryLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive deletion is honored by module list compile and compiler analysis", func(t *testing.T) {
		t.Parallel()

		projectDirectory := t.TempDir()
		scratchDirectory := t.TempDir()
		fixture := writeOverlayModule(t, projectDirectory)
		overlay := openDeletionOverlay(t, scratchDirectory, fixture.broken)
		argument, argumentErr := overlay.Argument()
		argumentValue, valueErr := argument.Value()
		if argumentErr != nil || valueErr != nil || argumentValue == "" {
			t.Fatalf("SourceOverlay.Argument() = (%q, %v, %v), want nonempty typed argument and nil errors", argumentValue, argumentErr, valueErr)
		}

		capability := openToolchainWithOverlay(t, &overlay)
		request := gotoolchain.ObservationRequest{WorkingDirectory: fixture.root}
		module, moduleErr := capability.ObserveModule(t.Context(), request)
		if moduleErr != nil || module.String() != overlayFixtureModule {
			t.Fatalf("Capability.ObserveModule(overlay) = (%q, %v), want (%q, nil)", module.String(), moduleErr, overlayFixtureModule)
		}
		catalog, listErr := capability.ListPackages(t.Context(), gotoolchain.ListRequest{
			WorkingDirectory: fixture.root,
			Pattern:          "./...",
		})
		if listErr != nil || len(catalog.Packages) != 1 || catalog.Packages[0].ImportPath.String() != overlayFixtureModule {
			t.Fatalf("Capability.ListPackages(overlay) = (%+v, %v), want one %q package", catalog, listErr, overlayFixtureModule)
		}
		compilation, compileErr := capability.CompilePackage(t.Context(), gotoolchain.CompileRequest{
			WorkingDirectory: fixture.root,
			Pattern:          "./...",
		})
		if compileErr != nil {
			t.Fatalf("Capability.CompilePackage(overlay) = (%v, %v), want nonzero compilation and nil", compilation, compileErr)
		}
		packagePath, pathErr := gomodule.ParseImportPath(overlayFixtureModule)
		if pathErr != nil {
			t.Fatalf("gomodule.ParseImportPath(overlay fixture) error = %v, want nil", pathErr)
		}
		analysis, analysisErr := capability.AnalyzePackage(t.Context(), gotoolchain.AnalysisRequest{
			WorkingDirectory: fixture.root,
			Package:          packagePath,
		})
		if analysisErr != nil || len(analysis.Units) != 1 {
			t.Fatalf("Capability.AnalyzePackage(overlay) = (%d units, %v), want (1, nil)", len(analysis.Units), analysisErr)
		}

		if closeErr := overlay.Close(); closeErr != nil {
			t.Fatalf("SourceOverlay.Close() error = %v, want nil", closeErr)
		}
		if closeErr := overlay.Close(); closeErr != nil {
			t.Fatalf("SourceOverlay.Close(repeated) error = %v, want nil", closeErr)
		}
		entries, readErr := os.ReadDir(scratchDirectory)
		if readErr != nil || len(entries) != 0 {
			t.Fatalf("overlay scratch after Close() = (%d entries, %v), want (0, nil)", len(entries), readErr)
		}
		got, gotErr := capability.CompilePackage(t.Context(), gotoolchain.CompileRequest{
			WorkingDirectory: fixture.root,
			Pattern:          "./...",
		})
		if !errors.Is(gotErr, core.ErrGoToolchainExecution) || got != (gotoolchain.Compilation{}) {
			t.Fatalf("Capability.CompilePackage(closed overlay) = (%v, %v), want zero and %v", got, gotErr, core.ErrGoToolchainExecution)
		}
	})

	t.Run("positive replacement makes cmd go consume the exact backing bytes", func(t *testing.T) {
		t.Parallel()

		projectDirectory := t.TempDir()
		scratchDirectory := t.TempDir()
		backingDirectory := t.TempDir()
		fixture := writeOverlayModule(t, projectDirectory)
		backing := filepath.Join(backingDirectory, "visible.go")
		if err := os.WriteFile(backing, []byte("package fixture\n\nconst Visible = true\n"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(replacement backing) error = %v, want nil", err)
		}
		if err := os.WriteFile(fixture.visible.String(), []byte("package fixture\n\nfunc Broken(\n"), 0o600); err != nil {
			t.Fatalf("os.WriteFile(live source mutation) error = %v, want nil", err)
		}
		overlay, err := gotoolchain.OpenSourceOverlay(t.Context(), gotoolchain.SourceOverlayRequest{
			Directory: mustOverlayAbsolutePath(t, scratchDirectory),
			Mappings: func(emit gotoolchain.EmitSourceOverlayMapping) error {
				if err := emit(gotoolchain.SourceOverlayMapping{Path: fixture.broken, Action: gotoolchain.SourceOverlayDelete}); err != nil {
					return err
				}
				return emit(gotoolchain.SourceOverlayMapping{
					Path: fixture.visible, Backing: mustOverlayAbsolutePath(t, backing), Action: gotoolchain.SourceOverlayReplace,
				})
			},
		})
		if err != nil {
			t.Fatalf("gotoolchain.OpenSourceOverlay(replacement) error = %v, want nil", err)
		}
		capability := openToolchainWithOverlay(t, &overlay)
		got, gotErr := capability.CompilePackage(t.Context(), gotoolchain.CompileRequest{WorkingDirectory: fixture.root, Pattern: "./..."})
		if gotErr != nil || got == (gotoolchain.Compilation{}) {
			t.Fatalf("Capability.CompilePackage(replaced source) = (%v, %v), want nonzero and nil", got, gotErr)
		}
		if err := overlay.Close(); err != nil {
			t.Fatalf("SourceOverlay.Close(replacement) error = %v, want nil", err)
		}
	})

	t.Run("negative duplicate deletion cannot create an ambiguous overlay", func(t *testing.T) {
		t.Parallel()

		projectDirectory := t.TempDir()
		scratchDirectory := t.TempDir()
		fixture := writeOverlayModule(t, projectDirectory)
		overlay, gotErr := gotoolchain.OpenSourceOverlay(t.Context(), gotoolchain.SourceOverlayRequest{
			Directory: mustOverlayAbsolutePath(t, scratchDirectory),
			Mappings: func(emit gotoolchain.EmitSourceOverlayMapping) error {
				deletion := gotoolchain.SourceOverlayMapping{Path: fixture.broken, Action: gotoolchain.SourceOverlayDelete}
				if err := emit(deletion); err != nil {
					return err
				}
				return emit(deletion)
			},
		})
		if !errors.Is(gotErr, core.ErrGoToolchainContract) || overlay != (gotoolchain.SourceOverlay{}) {
			t.Fatalf("OpenSourceOverlay(duplicate) = (%v, %v), want zero and %v", overlay, gotErr, core.ErrGoToolchainContract)
		}
		requireOverlayScratchEmpty(t, scratchDirectory)
	})

	t.Run("negative reverse-ordered deletion cannot change first-match behavior", func(t *testing.T) {
		t.Parallel()

		projectDirectory := t.TempDir()
		scratchDirectory := t.TempDir()
		fixture := writeOverlayModule(t, projectDirectory)
		later := mustOverlayAbsolutePath(t, filepath.Join(fixture.root.String(), "z.go"))
		earlier := fixture.broken
		overlay, gotErr := gotoolchain.OpenSourceOverlay(t.Context(), gotoolchain.SourceOverlayRequest{
			Directory: mustOverlayAbsolutePath(t, scratchDirectory),
			Mappings: func(emit gotoolchain.EmitSourceOverlayMapping) error {
				if err := emit(gotoolchain.SourceOverlayMapping{Path: later, Action: gotoolchain.SourceOverlayDelete}); err != nil {
					return err
				}
				return emit(gotoolchain.SourceOverlayMapping{Path: earlier, Action: gotoolchain.SourceOverlayDelete})
			},
		})
		if !errors.Is(gotErr, core.ErrGoToolchainContract) || overlay != (gotoolchain.SourceOverlay{}) {
			t.Fatalf("OpenSourceOverlay(reverse order) = (%v, %v), want zero and %v", overlay, gotErr, core.ErrGoToolchainContract)
		}
		requireOverlayScratchEmpty(t, scratchDirectory)
	})

	t.Run("negative cancelled open creates no overlay artifact", func(t *testing.T) {
		t.Parallel()

		scratchDirectory := t.TempDir()
		ctx, cancel := context.WithCancel(t.Context())
		cancel()
		overlay, gotErr := gotoolchain.OpenSourceOverlay(ctx, gotoolchain.SourceOverlayRequest{
			Directory: mustOverlayAbsolutePath(t, scratchDirectory),
			Mappings:  func(gotoolchain.EmitSourceOverlayMapping) error { return nil },
		})
		if !errors.Is(gotErr, context.Canceled) || !errors.Is(gotErr, core.ErrGoToolchainExecution) || overlay != (gotoolchain.SourceOverlay{}) {
			t.Fatalf("OpenSourceOverlay(cancelled) = (%v, %v), want zero, %v, and %v", overlay, gotErr, context.Canceled, core.ErrGoToolchainExecution)
		}
		requireOverlayScratchEmpty(t, scratchDirectory)
	})

	t.Run("negative collision preserves the first owned overlay", func(t *testing.T) {
		t.Parallel()

		projectDirectory := t.TempDir()
		scratchDirectory := t.TempDir()
		fixture := writeOverlayModule(t, projectDirectory)
		first := openDeletionOverlay(t, scratchDirectory, fixture.broken)
		second, gotErr := gotoolchain.OpenSourceOverlay(t.Context(), gotoolchain.SourceOverlayRequest{
			Directory: mustOverlayAbsolutePath(t, scratchDirectory),
			Mappings: func(emit gotoolchain.EmitSourceOverlayMapping) error {
				return emit(gotoolchain.SourceOverlayMapping{Path: fixture.broken, Action: gotoolchain.SourceOverlayDelete})
			},
		})
		if !errors.Is(gotErr, core.ErrGoToolchainExecution) || second != (gotoolchain.SourceOverlay{}) {
			t.Fatalf("OpenSourceOverlay(collision) = (%v, %v), want zero and %v", second, gotErr, core.ErrGoToolchainExecution)
		}

		capability := openToolchainWithOverlay(t, &first)
		got, compileErr := capability.CompilePackage(t.Context(), gotoolchain.CompileRequest{
			WorkingDirectory: fixture.root,
			Pattern:          "./...",
		})
		if compileErr != nil || got == (gotoolchain.Compilation{}) {
			t.Fatalf("Capability.CompilePackage(first overlay after collision) = (%v, %v), want nonzero and nil", got, compileErr)
		}
		if closeErr := first.Close(); closeErr != nil {
			t.Fatalf("SourceOverlay.Close(first after collision) error = %v, want nil", closeErr)
		}
		requireOverlayScratchEmpty(t, scratchDirectory)
	})

	t.Run("negative empty mapping stream cannot fabricate a no-op overlay", func(t *testing.T) {
		t.Parallel()

		scratchDirectory := t.TempDir()
		overlay, gotErr := gotoolchain.OpenSourceOverlay(t.Context(), gotoolchain.SourceOverlayRequest{
			Directory: mustOverlayAbsolutePath(t, scratchDirectory),
			Mappings:  func(gotoolchain.EmitSourceOverlayMapping) error { return nil },
		})
		if !errors.Is(gotErr, core.ErrGoToolchainContract) || overlay != (gotoolchain.SourceOverlay{}) {
			t.Fatalf("OpenSourceOverlay(empty) = (%v, %v), want zero and %v", overlay, gotErr, core.ErrGoToolchainContract)
		}
		requireOverlayScratchEmpty(t, scratchDirectory)
	})

	t.Run("negative zero deletion path cannot escape into cmd go", func(t *testing.T) {
		t.Parallel()

		scratchDirectory := t.TempDir()
		overlay, gotErr := gotoolchain.OpenSourceOverlay(t.Context(), gotoolchain.SourceOverlayRequest{
			Directory: mustOverlayAbsolutePath(t, scratchDirectory),
			Mappings: func(emit gotoolchain.EmitSourceOverlayMapping) error {
				return emit(gotoolchain.SourceOverlayMapping{})
			},
		})
		if !errors.Is(gotErr, core.ErrGoToolchainContract) || overlay != (gotoolchain.SourceOverlay{}) {
			t.Fatalf("OpenSourceOverlay(zero mapping) = (%v, %v), want zero and %v", overlay, gotErr, core.ErrGoToolchainContract)
		}
		requireOverlayScratchEmpty(t, scratchDirectory)
	})

	t.Run("neutral absent overlay leaves invalid disk source visible", func(t *testing.T) {
		t.Parallel()

		projectDirectory := t.TempDir()
		fixture := writeOverlayModule(t, projectDirectory)
		capability := openToolchainWithOverlay(t, nil)
		got, gotErr := capability.CompilePackage(t.Context(), gotoolchain.CompileRequest{
			WorkingDirectory: fixture.root,
			Pattern:          "./...",
		})
		if !errors.Is(gotErr, core.ErrGoToolchainExecution) || got != (gotoolchain.Compilation{}) {
			t.Fatalf("Capability.CompilePackage(no overlay) = (%v, %v), want zero and %v", got, gotErr, core.ErrGoToolchainExecution)
		}
	})
}

func TestSourceOverlayMappingClosedContract(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	path := mustOverlayAbsolutePath(t, filepath.Join(root, "source.go"))
	backing := mustOverlayAbsolutePath(t, filepath.Join(root, "backing.go"))
	cases := []struct {
		name       string
		mapping    gotoolchain.SourceOverlayMapping
		wantAction string
		wantErr    bool
	}{
		{name: "delete requires no backing path", mapping: gotoolchain.SourceOverlayMapping{Path: path, Action: gotoolchain.SourceOverlayDelete}, wantAction: "delete"},
		{name: "replace requires a distinct absolute backing path", mapping: gotoolchain.SourceOverlayMapping{Path: path, Backing: backing, Action: gotoolchain.SourceOverlayReplace}, wantAction: "replace"},
		{name: "unknown action is refused", mapping: gotoolchain.SourceOverlayMapping{Path: path}, wantErr: true},
		{name: "future action is refused", mapping: gotoolchain.SourceOverlayMapping{Path: path, Action: gotoolchain.SourceOverlayAction(255)}, wantErr: true},
		{name: "delete with backing is contradictory", mapping: gotoolchain.SourceOverlayMapping{Path: path, Backing: backing, Action: gotoolchain.SourceOverlayDelete}, wantAction: "delete", wantErr: true},
		{name: "replace without backing is incomplete", mapping: gotoolchain.SourceOverlayMapping{Path: path, Action: gotoolchain.SourceOverlayReplace}, wantAction: "replace", wantErr: true},
		{name: "replacement of a path by itself is a no-op", mapping: gotoolchain.SourceOverlayMapping{Path: path, Backing: path, Action: gotoolchain.SourceOverlayReplace}, wantAction: "replace", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotErr := tc.mapping.Validate()
			if (gotErr != nil) != tc.wantErr {
				t.Fatalf("SourceOverlayMapping.Validate() error = %v, want error %t", gotErr, tc.wantErr)
			}
			if gotErr != nil && !errors.Is(gotErr, core.ErrGoToolchainContract) {
				t.Fatalf("SourceOverlayMapping.Validate() error = %v, want %v", gotErr, core.ErrGoToolchainContract)
			}
			gotAction := tc.mapping.Action.String()
			if gotAction != tc.wantAction && !(tc.wantAction == "" && gotAction == core.UnknownEnumDiagnostic) {
				t.Fatalf("SourceOverlayAction.String() = %q, want %q", gotAction, tc.wantAction)
			}
		})
	}
}

const overlayFixtureModule = "example.com/primitive-overlay-fixture"

type overlayModuleFixture struct {
	root    core.AbsolutePath
	broken  core.AbsolutePath
	visible core.AbsolutePath
}

func writeOverlayModule(t testing.TB, directory string) overlayModuleFixture {
	t.Helper()
	files := []struct {
		name    string
		content string
	}{
		{name: "go.mod", content: "module " + overlayFixtureModule + "\n\ngo 1.27.1\n"},
		{name: "visible.go", content: "package fixture\n\nconst Visible = true\n"},
		{name: "broken.go", content: "package fixture\n\nfunc Broken(\n"},
	}
	for _, file := range files {
		if err := os.WriteFile(filepath.Join(directory, file.name), []byte(file.content), 0o600); err != nil {
			t.Fatalf("os.WriteFile(%s) error = %v, want nil", file.name, err)
		}
	}
	observedDirectory, err := filepath.EvalSymlinks(directory)
	if err != nil {
		t.Fatalf("filepath.EvalSymlinks(module root) error = %v, want nil", err)
	}
	root := mustOverlayAbsolutePath(t, observedDirectory)
	return overlayModuleFixture{
		root:    root,
		broken:  mustOverlayAbsolutePath(t, filepath.Join(root.String(), "broken.go")),
		visible: mustOverlayAbsolutePath(t, filepath.Join(root.String(), "visible.go")),
	}
}

func openDeletionOverlay(t testing.TB, directory string, deleted core.AbsolutePath) gotoolchain.SourceOverlay {
	t.Helper()
	overlay, err := gotoolchain.OpenSourceOverlay(t.Context(), gotoolchain.SourceOverlayRequest{
		Directory: mustOverlayAbsolutePath(t, directory),
		Mappings: func(emit gotoolchain.EmitSourceOverlayMapping) error {
			return emit(gotoolchain.SourceOverlayMapping{Path: deleted, Action: gotoolchain.SourceOverlayDelete})
		},
	})
	if err != nil {
		t.Fatalf("gotoolchain.OpenSourceOverlay() error = %v, want nil", err)
	}
	return overlay
}

func openToolchainWithOverlay(t testing.TB, overlay *gotoolchain.SourceOverlay) gotoolchain.Capability {
	t.Helper()
	limits, limitsErr := gotoolchain.DefaultLimits()
	capability, openErr := gotoolchain.Open(t.Context(), gotoolchain.Configuration{
		Workspace:     gotoolchain.WorkspaceModeDisabled,
		SourceOverlay: overlay,
		Limits:        limits,
	})
	if err := errors.Join(limitsErr, openErr); err != nil {
		t.Fatalf("gotoolchain.Open(overlay) error = %v, want nil", err)
	}
	return capability
}

func mustOverlayAbsolutePath(t testing.TB, value string) core.AbsolutePath {
	t.Helper()
	path, err := core.ParseAbsolutePath(value)
	if err != nil {
		t.Fatalf("core.ParseAbsolutePath(%q) error = %v, want nil", value, err)
	}
	return path
}

func requireOverlayScratchEmpty(t testing.TB, directory string) {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 0 {
		t.Fatalf("overlay scratch entries = (%d, %v), want (0, nil)", len(entries), err)
	}
}
