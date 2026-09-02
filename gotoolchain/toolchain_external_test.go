package gotoolchain_test

import (
	"context"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gotoolchain"
	"github.com/deliri/primitive/v2026/process"
)

func TestCapabilityProductionPathLayerTriad(t *testing.T) {
	t.Parallel()

	limits, err := gotoolchain.DefaultLimits()
	if err != nil {
		t.Fatalf("gotoolchain.DefaultLimits() error = %v, want nil", err)
	}
	capability, err := gotoolchain.Open(context.Background(), gotoolchain.Configuration{
		Workspace: gotoolchain.WorkspaceModeDisabled,
		Limits:    limits,
	})
	if err != nil {
		t.Fatalf("gotoolchain.Open() error = %v, want nil", err)
	}
	directory, err := process.WorkingDirectory()
	if err != nil {
		t.Fatalf("process.WorkingDirectory() error = %v, want nil", err)
	}
	directory, err = directory.Parent()
	if err != nil {
		t.Fatalf("working directory Parent() error = %v, want nil", err)
	}

	t.Run("positive cmd go emits typed module build package and compilation evidence", func(t *testing.T) {
		t.Parallel()

		request := gotoolchain.ObservationRequest{WorkingDirectory: directory}
		module, moduleErr := capability.ObserveModule(context.Background(), request)
		if moduleErr != nil || module.String() != core.PrimitiveModulePath {
			t.Fatalf("Capability.ObserveModule() = (%q, %v), want (%q, nil)", module.String(), moduleErr, core.PrimitiveModulePath)
		}
		build, buildErr := capability.ObserveBuildContext(context.Background(), request)
		if buildErr != nil {
			t.Fatalf("Capability.ObserveBuildContext() error = %v, want nil", buildErr)
		}
		if err := build.Validate(); err != nil {
			t.Fatalf("BuildContext.Validate() error = %v, want nil", err)
		}
		catalog, listErr := capability.ListPackages(context.Background(), gotoolchain.ListRequest{
			WorkingDirectory: directory,
			Pattern:          "./gomodule",
		})
		if listErr != nil {
			t.Fatalf("Capability.ListPackages() error = %v, want nil", listErr)
		}
		if len(catalog.Packages) != 1 || catalog.Packages[0].ImportPath.String() != core.PrimitiveModulePath+"/gomodule" {
			t.Fatalf("Capability.ListPackages() = %+v, want exact gomodule package", catalog)
		}
		compilation, compileErr := capability.CompilePackage(context.Background(), gotoolchain.CompileRequest{
			WorkingDirectory: directory,
			Pattern:          "./gotoolchain/testdata/compileonly",
		})
		if compileErr != nil {
			t.Fatalf("Capability.CompilePackage() error = %v, want nil", compileErr)
		}
		if err := compilation.Validate(); err != nil {
			t.Fatalf("Compilation.Validate() error = %v, want nil", err)
		}
	})

	t.Run("negative flag-shaped package operands are refused before cmd go", func(t *testing.T) {
		t.Parallel()

		got, gotErr := capability.CompilePackage(context.Background(), gotoolchain.CompileRequest{
			WorkingDirectory: directory,
			Pattern:          "-run=TestMain",
		})
		if !errors.Is(gotErr, core.ErrGoToolchainContract) || got != (gotoolchain.Compilation{}) {
			t.Fatalf("Capability.CompilePackage(flag operand) = (%v, %v), want zero and errors.Is(..., %v)", got, gotErr, core.ErrGoToolchainContract)
		}
	})

	t.Run("negative absent package pattern is refused before cmd go", func(t *testing.T) {
		t.Parallel()

		got, gotErr := capability.ListPackages(context.Background(), gotoolchain.ListRequest{WorkingDirectory: directory})
		if !errors.Is(gotErr, core.ErrGoToolchainContract) || len(got.Packages) != 0 {
			t.Fatalf("Capability.ListPackages(absent pattern) = (%v, %v), want zero and errors.Is(..., %v)", got, gotErr, core.ErrGoToolchainContract)
		}
	})

	t.Run("neutral pre-cancelled observation preserves cancellation and emits no module", func(t *testing.T) {
		t.Parallel()

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		got, gotErr := capability.ObserveModule(ctx, gotoolchain.ObservationRequest{WorkingDirectory: directory})
		if !errors.Is(gotErr, context.Canceled) || !errors.Is(gotErr, core.ErrGoToolchainExecution) || got.String() != "" {
			t.Fatalf("Capability.ObserveModule(cancelled) = (%q, %v), want zero with context.Canceled and %v", got.String(), gotErr, core.ErrGoToolchainExecution)
		}
	})
}

func TestCompilerScalarsRejectUnknownAndPreserveCanonicalValues(t *testing.T) {
	t.Parallel()

	versions := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "minimum canonical release", value: "go1.0"},
		{name: "current canonical patch release", value: "go1.27.0"},
		{name: "future canonical major component", value: "go1.100.2"},
		{name: "absent token refused", value: "", wantErr: true},
		{name: "missing go prefix refused", value: "1.27.0", wantErr: true},
		{name: "missing major one refused", value: "go2.0", wantErr: true},
		{name: "empty numeric component refused", value: "go1..2", wantErr: true},
		{name: "trailing empty numeric component refused", value: "go1.27.", wantErr: true},
		{name: "nonnumeric suffix refused", value: "go1.27rc1", wantErr: true},
		{name: "whitespace refused", value: "go1.27.0 ", wantErr: true},
	}
	for _, tc := range versions {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := gotoolchain.ParseToolchainVersion(tc.value)
			if (gotErr != nil) != tc.wantErr {
				t.Fatalf("ParseToolchainVersion(%q) error = %v, want error %t", tc.value, gotErr, tc.wantErr)
			}
			if tc.wantErr {
				if !errors.Is(gotErr, core.ErrGoToolchainContract) || got.String() != "" {
					t.Fatalf("ParseToolchainVersion(%q) = (%q, %v), want zero and typed rejection", tc.value, got.String(), gotErr)
				}
				return
			}
			if got.String() != tc.value {
				t.Fatalf("ParseToolchainVersion(%q).String() = %q, want %q", tc.value, got.String(), tc.value)
			}
		})
	}
}
