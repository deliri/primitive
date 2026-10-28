package gomodule_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
	"golang.org/x/mod/modfile"
)

func TestRequireEditPreservesUnrelatedSourceLayerTriad(t *testing.T) {
	t.Parallel()
	path, err := gomodule.ParsePath("example.com/dependency")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, source, want string
		neutral            bool
	}{
		{"new direct requirement", "module example.com/app\n", "require example.com/dependency v1.3.0", false},
		{"existing direct version changes", "module example.com/app\nrequire example.com/dependency v1.0.0\n", "require example.com/dependency v1.3.0", false},
		{"existing indirect remains indirect", "module example.com/app\nrequire example.com/dependency v1.0.0 // indirect\n", "require example.com/dependency v1.3.0 // indirect", false},
		{"requirement comment preserved", "module example.com/app\nrequire example.com/dependency v1.0.0 // owned comment\n", "require example.com/dependency v1.3.0 // owned comment", false},
		{"local replacement preserved", "module example.com/app\nreplace example.com/dependency => ../local\n", "replace example.com/dependency => ../local", false},
		{"versioned replacement preserved", "module example.com/app\nreplace example.com/other v1.0.0 => example.com/fork v1.1.0\n", "replace example.com/other v1.0.0 => example.com/fork v1.1.0", false},
		{"exclusion preserved", "module example.com/app\nexclude example.com/other v1.0.0\n", "exclude example.com/other v1.0.0", false},
		{"retraction preserved", "module example.com/app\nretract [v1.0.0, v1.1.0] // withdrawn\n", "retract [v1.0.0, v1.1.0] // withdrawn", false},
		{"toolchain and go version preserved", "module example.com/app\ngo 1.27.2\ntoolchain go1.27.3\n", "toolchain go1.27.3", false},
		{"local main module supported", "module app\n", "module app", false},
		{"unrelated indirect preserved", "module example.com/app\nrequire example.com/other v1.1.0 // indirect\n", "example.com/other v1.1.0 // indirect", false},
		{"empty edit is exact independent source", "// keep spacing\r\nmodule  example.com/app\r\n", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			source := []byte(tc.source)
			request := gomodule.RequireEditRequest{Source: source}
			if !tc.neutral {
				request.Requirements = []gomodule.Requirement{{Path: path, Version: "v1.3.0"}}
			}
			got, err := gomodule.EditRequirements(t.Context(), request)
			if err != nil || !bytes.Equal(source, []byte(tc.source)) {
				t.Fatalf("edit = %q/%v, source preserved=%t; want admitted isolated output", got, err, bytes.Equal(source, []byte(tc.source)))
			}
			if tc.neutral {
				if !bytes.Equal(got, source) {
					t.Fatalf("neutral edit = %q, want exact %q", got, source)
				}
				got[0] ^= 1
				if !bytes.Equal(source, []byte(tc.source)) {
					t.Fatal("neutral output aliases borrowed source")
				}
				return
			}
			if !bytes.Contains(got, []byte(tc.want)) {
				t.Fatalf("edited source = %q, want retained directive %q", got, tc.want)
			}
			parsed, err := modfile.Parse("go.mod", got, nil)
			if err != nil {
				t.Fatal(err)
			}
			matches := 0
			for _, requirement := range parsed.Require {
				if requirement.Mod.Path == path.String() {
					matches++
					if requirement.Mod.Version != "v1.3.0" {
						t.Fatalf("required version = %q, want v1.3.0", requirement.Mod.Version)
					}
				}
			}
			if matches != 1 {
				t.Fatalf("edited path occurrences = %d, want one", matches)
			}
			second, err := gomodule.EditRequirements(t.Context(), gomodule.RequireEditRequest{Source: got, Requirements: request.Requirements})
			if err != nil || !bytes.Equal(second, got) {
				t.Fatalf("second edit = %q/%v, want canonical %q", second, err, got)
			}
		})
	}
}

func TestRequireEditRefusesMalformedAndAmbiguousSources(t *testing.T) {
	t.Parallel()
	path, err := gomodule.ParsePath("example.com/dependency")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, source string }{
		{"absent source", ""},
		{"missing module declaration", "go 1.27.2\n"},
		{"truncated requirement block", "module example.com/app\nrequire (\n"},
		{"duplicate module declaration", "module example.com/app\nmodule example.com/other\n"},
		{"duplicate conflicting source requirement", "module example.com/app\nrequire example.com/dependency v1.0.0\nrequire example.com/dependency v1.2.0\n"},
		{"duplicate equal source requirement", "module example.com/app\nrequire example.com/dependency v1.0.0\nrequire example.com/dependency v1.0.0\n"},
		{"unknown source directive", "module example.com/app\nfuture value\n"},
		{"malformed source version", "module example.com/app\nrequire example.com/other latest\n"},
		{"source version path major mismatch", "module example.com/app\nrequire example.com/other/v2 v1.2.0\n"},
		{"module traversal declaration", "module example.com/app/../other\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			source := []byte(tc.source)
			got, err := gomodule.EditRequirements(t.Context(), gomodule.RequireEditRequest{Source: source, Requirements: []gomodule.Requirement{{Path: path, Version: "v1.3.0"}}})
			if !errors.Is(err, core.ErrGoModuleContract) || got != nil || !bytes.Equal(source, []byte(tc.source)) {
				t.Fatalf("refused edit = %q/%v source=%q, want nil, module contract and preserved source", got, err, source)
			}
		})
	}
}

func TestRequirementVersionAndEditAdmission(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, path, version string
		wantErr             bool
	}{
		{"canonical release", "example.com/lib", "v1.2.3", false},
		{"canonical prerelease", "example.com/lib", "v1.2.3-rc.1", false},
		{"canonical pseudo version", "example.com/lib", "v0.0.0-20260101000000-0123456789ab", false},
		{"explicit major suffix", "example.com/lib/v2", "v2.0.0", false},
		{"incompatible unsuffixed major", "example.com/lib", "v2.0.0+incompatible", false},
		{"gopkg major spelling", "gopkg.in/yaml.v3", "v3.0.1", false},
		{"absent version", "example.com/lib", "", true},
		{"version query not pinned", "example.com/lib", "latest", true},
		{"short version not canonical", "example.com/lib", "v1.2", true},
		{"discarded build metadata refused", "example.com/lib", "v1.2.3+build", true},
		{"missing required path major", "example.com/lib", "v2.0.0", true},
		{"wrong explicit path major", "example.com/lib/v2", "v3.0.0", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path, err := gomodule.ParsePath(tc.path)
			if err != nil {
				t.Fatal(err)
			}
			requirement := gomodule.Requirement{Path: path, Version: tc.version}
			got, err := gomodule.EditRequirements(t.Context(), gomodule.RequireEditRequest{Source: []byte("module example.com/app\n"), Requirements: []gomodule.Requirement{requirement}})
			if tc.wantErr {
				if !errors.Is(err, core.ErrGoModuleContract) || got != nil || !errors.Is(requirement.Validate(), core.ErrGoModuleContract) {
					t.Fatalf("version refusal = %q/%v, want nil and module contract", got, err)
				}
			} else if err != nil || requirement.Validate() != nil || !bytes.Contains(got, []byte(tc.path+" "+tc.version)) {
				t.Fatalf("version edit = %q/%v, want exact %s@%s", got, err, tc.path, tc.version)
			}
		})
	}
}

func TestRequireEditContextAndDuplicateAdmission(t *testing.T) {
	t.Parallel()
	path, err := gomodule.ParsePath("example.com/lib")
	if err != nil {
		t.Fatal(err)
	}
	requirement := gomodule.Requirement{Path: path, Version: "v1.2.3"}
	ctx, cancel := context.WithCancelCause(t.Context())
	cancel(io.ErrClosedPipe)
	for _, tc := range []struct {
		name         string
		ctx          context.Context
		requirements []gomodule.Requirement
		want         error
	}{
		{"nil context", nil, []gomodule.Requirement{requirement}, core.ErrNilContext},
		{"cancellation retains terminal", ctx, []gomodule.Requirement{requirement}, context.Canceled},
		{"zero nominal path", t.Context(), []gomodule.Requirement{{Version: "v1.2.3"}}, core.ErrGoModuleContract},
		{"duplicate requested paths", t.Context(), []gomodule.Requirement{requirement, requirement}, core.ErrGoModuleContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := gomodule.EditRequirements(tc.ctx, gomodule.RequireEditRequest{Source: []byte("module app\n"), Requirements: tc.requirements})
			if got != nil || !errors.Is(err, tc.want) || !errors.Is(err, core.ErrGoModuleContract) || (tc.ctx == ctx && !errors.Is(err, io.ErrClosedPipe)) {
				t.Fatalf("admission = %q/%v, want nil and %v", got, err, tc.want)
			}
		})
	}
}

func FuzzRequireEditSemanticPreservation(f *testing.F) {
	path, err := gomodule.ParsePath("example.com/dependency")
	if err != nil {
		f.Fatal(err)
	}
	file := new(modfile.File)
	if err := file.AddModuleStmt("example.com/app"); err != nil {
		f.Fatal(err)
	}
	seed, err := file.Format()
	if err != nil {
		f.Fatal(err)
	}
	canonical, err := gomodule.EditRequirements(f.Context(), gomodule.RequireEditRequest{Source: seed, Requirements: []gomodule.Requirement{{Path: path, Version: "v1.2.3"}}})
	if err != nil {
		f.Fatal(err)
	}
	f.Add(canonical, "v1.3.0")
	f.Add([]byte(""), "v1.3.0")
	f.Add([]byte("module app\nrequire ("), "latest")
	f.Fuzz(func(t *testing.T, source []byte, version string) {
		before := bytes.Clone(source)
		request := gomodule.RequireEditRequest{Source: source, Requirements: []gomodule.Requirement{{Path: path, Version: version}}}
		got, err := gomodule.EditRequirements(t.Context(), request)
		if !bytes.Equal(before, source) {
			t.Fatal("editing mutated borrowed source")
		}
		if err != nil {
			if got != nil || !errors.Is(err, core.ErrGoModuleContract) {
				t.Fatalf("refusal = %q/%v, want nil and module contract", got, err)
			}
			return
		}
		parsed, err := modfile.Parse("go.mod", got, nil)
		if err != nil || parsed.Module == nil || request.Requirements[0].Validate() != nil {
			t.Fatalf("accepted edit = %q/%v, want valid module and pinned requirement", got, err)
		}
		matches := 0
		for _, requirement := range parsed.Require {
			if requirement.Mod.Path == path.String() {
				matches++
				if requirement.Mod.Version != version {
					t.Fatalf("edited version = %q, want exact %q", requirement.Mod.Version, version)
				}
			}
		}
		if matches != 1 {
			t.Fatalf("edited requirement count = %d, want one", matches)
		}
		second, err := gomodule.EditRequirements(t.Context(), gomodule.RequireEditRequest{Source: got, Requirements: request.Requirements})
		if err != nil || !bytes.Equal(second, got) {
			t.Fatalf("canonical repeated edit = %q/%v, want %q", second, err, got)
		}
	})
}
