package gomodule_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
	"golang.org/x/mod/module"
)

func FuzzPathTextAdmission(f *testing.F) {
	seed, err := gomodule.ParsePath("example.com/project/v2")
	if err != nil {
		f.Fatalf("ParsePath(seed) error = %v, want nil", err)
	}
	f.Add(seed.String())
	f.Add("")
	f.Add("net//http")
	f.Fuzz(func(t *testing.T, text string) {
		got, gotErr := gomodule.ParsePath(text)
		// x/mod owns the Go grammar. This proves the wrapper preserves that
		// public agreement; named hostile rows independently pin its grammar.
		wantErr := module.CheckPath(text)
		if (gotErr == nil) != (wantErr == nil) {
			t.Fatalf("ParsePath(%q) error = %v, want upstream admission %v", text, gotErr, wantErr)
		}
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrGoModuleContract) || got != (gomodule.Path{}) {
				t.Fatalf("ParsePath(refused) = (%v, %v), want zero and %v", got, gotErr, core.ErrGoModuleContract)
			}
			if text != "" {
				var detail *module.InvalidPathError
				if !errors.As(gotErr, &detail) || detail.Path != text || detail.Kind != "module" {
					t.Fatalf("ParsePath refusal detail = %+v, want module path %q", detail, text)
				}
			}
			return
		}
		if err := got.Validate(); err != nil || got.String() != text {
			t.Fatalf("ParsePath(accepted) = (%q, %v), want (%q, nil)", got.String(), err, text)
		}
		encoded, err := got.MarshalJSON()
		if err != nil {
			t.Fatalf("Path.MarshalJSON() error = %v, want nil", err)
		}
		var decoded gomodule.Path
		if err := decoded.UnmarshalJSON(encoded); err != nil || decoded != got {
			t.Fatalf("Path round trip = (%v, %v), want (%v, nil)", decoded, err, got)
		}
	})
}

func FuzzImportPathTextAdmission(f *testing.F) {
	seed, err := gomodule.ParseImportPath("net/http")
	if err != nil {
		f.Fatalf("ParseImportPath(seed) error = %v, want nil", err)
	}
	f.Add(seed.String())
	f.Add("")
	f.Add("net//http")
	f.Fuzz(func(t *testing.T, text string) {
		got, gotErr := gomodule.ParseImportPath(text)
		wantErr := module.CheckImportPath(text)
		if (gotErr == nil) != (wantErr == nil) {
			t.Fatalf("ParseImportPath(%q) error = %v, want upstream admission %v", text, gotErr, wantErr)
		}
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrGoModuleContract) || got != (gomodule.ImportPath{}) {
				t.Fatalf("ParseImportPath(refused) = (%v, %v), want zero and %v", got, gotErr, core.ErrGoModuleContract)
			}
			if text != "" {
				var detail *module.InvalidPathError
				if !errors.As(gotErr, &detail) || detail.Path != text || detail.Kind != "import" {
					t.Fatalf("ParseImportPath refusal detail = %+v, want import path %q", detail, text)
				}
			}
			return
		}
		if err := got.Validate(); err != nil || got.String() != text {
			t.Fatalf("ParseImportPath(accepted) = (%q, %v), want (%q, nil)", got.String(), err, text)
		}
		encoded, err := got.MarshalJSON()
		if err != nil {
			t.Fatalf("ImportPath.MarshalJSON() error = %v, want nil", err)
		}
		var decoded gomodule.ImportPath
		if err := decoded.UnmarshalJSON(encoded); err != nil || decoded != got {
			t.Fatalf("ImportPath round trip = (%v, %v), want (%v, nil)", decoded, err, got)
		}
	})
}
