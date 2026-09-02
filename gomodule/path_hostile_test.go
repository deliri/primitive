package gomodule_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
)

type modulePathCaseClass uint8

const (
	modulePathCaseValid modulePathCaseClass = iota + 1
	modulePathCaseReject
	modulePathCaseBoundary
)

func TestParsePathHostileDomain(t *testing.T) {
	t.Parallel()

	prefix := "example.com/"
	atMaximum := prefix + strings.Repeat("a", gomodule.PathMaximumBytes-len(prefix))
	cases := []struct {
		name    string
		value   string
		class   modulePathCaseClass
		wantErr bool
	}{
		{name: "valid ordinary repository", value: "example.com/project", class: modulePathCaseValid},
		{name: "valid subdomain repository", value: "code.example.com/project", class: modulePathCaseValid},
		{name: "valid major version suffix", value: "example.com/project/v2", class: modulePathCaseValid},
		{name: "valid gopkg version form", value: "gopkg.in/yaml.v3", class: modulePathCaseValid},
		{name: "valid hyphenated element", value: "example.com/project-name", class: modulePathCaseValid},
		{name: "valid underscored element", value: "example.com/project_name", class: modulePathCaseValid},
		{name: "valid nondigit tilde suffix", value: "example.com/project~next", class: modulePathCaseValid},
		{name: "valid uppercase repository element", value: "example.com/Project", class: modulePathCaseValid},
		{name: "valid dotted repository element", value: "example.com/project.name", class: modulePathCaseValid},
		{name: "valid punycode domain", value: "xn--bcher-kva.example/project", class: modulePathCaseValid},

		{name: "reject absent path", value: "", class: modulePathCaseReject, wantErr: true},
		{name: "reject first element without dot", value: "example/project", class: modulePathCaseReject, wantErr: true},
		{name: "reject leading slash", value: "/example.com/project", class: modulePathCaseReject, wantErr: true},
		{name: "reject trailing slash", value: "example.com/project/", class: modulePathCaseReject, wantErr: true},
		{name: "reject empty middle element", value: "example.com//project", class: modulePathCaseReject, wantErr: true},
		{name: "reject uppercase domain", value: "Example.com/project", class: modulePathCaseReject, wantErr: true},
		{name: "reject leading dash domain", value: "-example.com/project", class: modulePathCaseReject, wantErr: true},
		{name: "reject hidden repository element", value: "example.com/.project", class: modulePathCaseReject, wantErr: true},
		{name: "reject trailing dot element", value: "example.com/project.", class: modulePathCaseReject, wantErr: true},
		{name: "reject shell punctuation", value: "example.com/project@next", class: modulePathCaseReject, wantErr: true},

		{name: "boundary byte ceiling minus one accepted", value: atMaximum[:len(atMaximum)-1], class: modulePathCaseBoundary},
		{name: "boundary exact byte ceiling accepted", value: atMaximum, class: modulePathCaseBoundary},
		{name: "boundary byte ceiling plus one refused", value: atMaximum + "a", class: modulePathCaseBoundary, wantErr: true},
		{name: "boundary byte ceiling extreme refused", value: atMaximum + strings.Repeat("a", gomodule.PathMaximumBytes), class: modulePathCaseBoundary, wantErr: true},
		{name: "boundary version one refused", value: "example.com/project/v1", class: modulePathCaseBoundary, wantErr: true},
		{name: "boundary version two accepted", value: "example.com/project/v2", class: modulePathCaseBoundary},
		{name: "boundary version leading zero refused", value: "example.com/project/v02", class: modulePathCaseBoundary, wantErr: true},
		{name: "boundary large major version accepted", value: "example.com/project/v999999", class: modulePathCaseBoundary},
		{name: "boundary Windows con element refused", value: "example.com/con", class: modulePathCaseBoundary, wantErr: true},
		{name: "boundary Windows con extension refused", value: "example.com/con.txt", class: modulePathCaseBoundary, wantErr: true},
		{name: "boundary Windows console element accepted", value: "example.com/console", class: modulePathCaseBoundary},
		{name: "boundary Windows com zero element accepted", value: "example.com/com0", class: modulePathCaseBoundary},
		{name: "boundary short name one refused", value: "example.com/projec~1", class: modulePathCaseBoundary, wantErr: true},
		{name: "boundary short name large refused", value: "example.com/projec~999", class: modulePathCaseBoundary, wantErr: true},
		{name: "boundary bare tilde accepted", value: "example.com/project~", class: modulePathCaseBoundary},
		{name: "boundary nondigit tilde suffix accepted", value: "example.com/project~x", class: modulePathCaseBoundary},
		{name: "boundary leading dot refused", value: "example.com/.hidden", class: modulePathCaseBoundary, wantErr: true},
		{name: "boundary trailing dot refused", value: "example.com/hidden.", class: modulePathCaseBoundary, wantErr: true},
		{name: "boundary all dot element refused", value: "example.com/..", class: modulePathCaseBoundary, wantErr: true},
		{name: "boundary internal single dot accepted", value: "example.com/hidden.file", class: modulePathCaseBoundary},
	}

	counts := [4]int{}
	for _, tc := range cases {
		counts[tc.class]++
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := gomodule.ParsePath(tc.value)
			if (gotErr != nil) != tc.wantErr {
				t.Fatalf("gomodule.ParsePath(%q) error = %v, want error %t", tc.value, gotErr, tc.wantErr)
			}
			if tc.wantErr {
				if !errors.Is(gotErr, core.ErrGoModuleContract) {
					t.Fatalf("gomodule.ParsePath(%q) error = %v, want errors.Is(..., %v)", tc.value, gotErr, core.ErrGoModuleContract)
				}
				if got != (gomodule.Path{}) {
					t.Fatalf("gomodule.ParsePath(%q) = %v, want zero on rejection", tc.value, got)
				}
				return
			}
			if err := got.Validate(); err != nil {
				t.Fatalf("gomodule.ParsePath(%q).Validate() error = %v, want nil", tc.value, err)
			}
			if got.String() != tc.value {
				t.Fatalf("gomodule.ParsePath(%q).String() = %q, want %q", tc.value, got.String(), tc.value)
			}
		})
	}
	if counts[modulePathCaseValid] != 10 || counts[modulePathCaseReject] != 10 || counts[modulePathCaseBoundary] != 20 {
		t.Fatalf("hostile path classes = valid:%d reject:%d boundary:%d, want 10/10/20", counts[modulePathCaseValid], counts[modulePathCaseReject], counts[modulePathCaseBoundary])
	}
}

func TestPathJSONLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive canonical string round trip", func(t *testing.T) {
		t.Parallel()

		path, err := gomodule.ParsePath("example.com/project/v2")
		if err != nil {
			t.Fatalf("gomodule.ParsePath() error = %v, want nil", err)
		}
		encoded, err := path.MarshalJSON()
		if err != nil {
			t.Fatalf("Path.MarshalJSON() error = %v, want nil", err)
		}
		var got gomodule.Path
		if err := got.UnmarshalJSON(encoded); err != nil {
			t.Fatalf("Path.UnmarshalJSON() error = %v, want nil", err)
		}
		if got != path || string(encoded) != `"example.com/project/v2"` {
			t.Fatalf("Path JSON round trip = (%v, %q), want (%v, canonical string)", got, encoded, path)
		}
	})

	t.Run("negative rejected JSON preserves populated receiver", func(t *testing.T) {
		t.Parallel()

		got, err := gomodule.ParsePath("example.com/original")
		if err != nil {
			t.Fatalf("gomodule.ParsePath() error = %v, want nil", err)
		}
		want := got
		gotErr := got.UnmarshalJSON([]byte(`"example/project"`))
		if !errors.Is(gotErr, core.ErrGoModuleContract) || got != want {
			t.Fatalf("Path.UnmarshalJSON(rejected) = (%v, %v), want preserved %v and errors.Is(..., %v)", got, gotErr, want, core.ErrGoModuleContract)
		}
	})

	t.Run("neutral zero path refuses outward projection", func(t *testing.T) {
		t.Parallel()

		var path gomodule.Path
		encoded, gotErr := path.MarshalJSON()
		if !errors.Is(gotErr, core.ErrGoModuleContract) || encoded != nil {
			t.Fatalf("zero Path.MarshalJSON() = (%q, %v), want nil and errors.Is(..., %v)", encoded, gotErr, core.ErrGoModuleContract)
		}
	})
}

func TestParseImportPathHostileDomain(t *testing.T) {
	t.Parallel()

	prefix := "example.com/"
	atMaximum := prefix + strings.Repeat("a", gomodule.ImportPathMaximumBytes-len(prefix))
	cases := []struct {
		name    string
		value   string
		class   modulePathCaseClass
		wantErr bool
	}{
		{name: "valid standard library root", value: "fmt", class: modulePathCaseValid},
		{name: "valid standard library child", value: "net/http", class: modulePathCaseValid},
		{name: "valid unicode standard library child", value: "unicode/utf8", class: modulePathCaseValid},
		{name: "valid module root package", value: "example.com/project", class: modulePathCaseValid},
		{name: "valid module child package", value: "example.com/project/internal/tool", class: modulePathCaseValid},
		{name: "valid major version package", value: "example.com/project/v2", class: modulePathCaseValid},
		{name: "valid hyphenated package element", value: "example.com/project-name", class: modulePathCaseValid},
		{name: "valid underscored package element", value: "example.com/project_name", class: modulePathCaseValid},
		{name: "valid dotted package element", value: "example.com/project.name", class: modulePathCaseValid},
		{name: "valid gopkg package identity", value: "gopkg.in/yaml.v3", class: modulePathCaseValid},

		{name: "reject absent import path", value: "", class: modulePathCaseReject, wantErr: true},
		{name: "reject leading slash", value: "/fmt", class: modulePathCaseReject, wantErr: true},
		{name: "reject trailing slash", value: "fmt/", class: modulePathCaseReject, wantErr: true},
		{name: "reject empty middle element", value: "net//http", class: modulePathCaseReject, wantErr: true},
		{name: "reject backslash separator", value: `net\http`, class: modulePathCaseReject, wantErr: true},
		{name: "reject embedded whitespace", value: "net /http", class: modulePathCaseReject, wantErr: true},
		{name: "reject version punctuation", value: "example.com/project@v2", class: modulePathCaseReject, wantErr: true},
		{name: "reject current directory element", value: "net/./http", class: modulePathCaseReject, wantErr: true},
		{name: "reject parent directory element", value: "net/../http", class: modulePathCaseReject, wantErr: true},
		{name: "reject quote punctuation", value: `example.com/project"next`, class: modulePathCaseReject, wantErr: true},

		{name: "boundary byte ceiling minus one accepted", value: atMaximum[:len(atMaximum)-1], class: modulePathCaseBoundary},
		{name: "boundary exact byte ceiling accepted", value: atMaximum, class: modulePathCaseBoundary},
		{name: "boundary byte ceiling plus one refused", value: atMaximum + "a", class: modulePathCaseBoundary, wantErr: true},
		{name: "boundary extreme byte extent refused", value: atMaximum + strings.Repeat("a", gomodule.ImportPathMaximumBytes), class: modulePathCaseBoundary, wantErr: true},
		{name: "boundary one-character import accepted", value: "x", class: modulePathCaseBoundary},
		{name: "boundary leading dot import accepted", value: ".x", class: modulePathCaseBoundary},
		{name: "boundary leading dash refused", value: "-x", class: modulePathCaseBoundary, wantErr: true},
		{name: "boundary leading underscore accepted", value: "_x", class: modulePathCaseBoundary},
		{name: "boundary trailing dot refused", value: "x.", class: modulePathCaseBoundary, wantErr: true},
		{name: "boundary trailing dash accepted", value: "x-", class: modulePathCaseBoundary},
		{name: "boundary Windows con element refused", value: "con", class: modulePathCaseBoundary, wantErr: true},
		{name: "boundary Windows con extension refused", value: "con.txt", class: modulePathCaseBoundary, wantErr: true},
		{name: "boundary Windows console element accepted", value: "console", class: modulePathCaseBoundary},
		{name: "boundary Windows com zero accepted", value: "com0", class: modulePathCaseBoundary},
		{name: "boundary Windows com one refused", value: "com1", class: modulePathCaseBoundary, wantErr: true},
		{name: "boundary uppercase package accepted", value: "Example", class: modulePathCaseBoundary},
		{name: "boundary tilde package accepted", value: "project~next", class: modulePathCaseBoundary},
		{name: "boundary percent punctuation refused", value: "project%next", class: modulePathCaseBoundary, wantErr: true},
		{name: "boundary NUL refused", value: "project\x00next", class: modulePathCaseBoundary, wantErr: true},
		{name: "boundary invalid UTF-8 refused", value: "project\xffnext", class: modulePathCaseBoundary, wantErr: true},
	}

	counts := [4]int{}
	for _, tc := range cases {
		counts[tc.class]++
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := gomodule.ParseImportPath(tc.value)
			if (gotErr != nil) != tc.wantErr {
				t.Fatalf("gomodule.ParseImportPath(%q) error = %v, want error %t", tc.value, gotErr, tc.wantErr)
			}
			if tc.wantErr {
				if !errors.Is(gotErr, core.ErrGoModuleContract) || got != (gomodule.ImportPath{}) {
					t.Fatalf("gomodule.ParseImportPath(%q) = (%v, %v), want zero and errors.Is(..., %v)", tc.value, got, gotErr, core.ErrGoModuleContract)
				}
				return
			}
			if err := got.Validate(); err != nil || got.String() != tc.value {
				t.Fatalf("gomodule.ParseImportPath(%q) = (%q, %v), want canonical value and nil", tc.value, got.String(), err)
			}
		})
	}
	if counts[modulePathCaseValid] != 10 || counts[modulePathCaseReject] != 10 || counts[modulePathCaseBoundary] != 20 {
		t.Fatalf("hostile import path classes = valid:%d reject:%d boundary:%d, want 10/10/20", counts[modulePathCaseValid], counts[modulePathCaseReject], counts[modulePathCaseBoundary])
	}
}

func TestImportPathJSONLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive canonical import string round trip", func(t *testing.T) {
		t.Parallel()

		path, err := gomodule.ParseImportPath("example.com/project/internal/tool")
		if err != nil {
			t.Fatalf("gomodule.ParseImportPath() error = %v, want nil", err)
		}
		encoded, err := path.MarshalJSON()
		if err != nil {
			t.Fatalf("ImportPath.MarshalJSON() error = %v, want nil", err)
		}
		var got gomodule.ImportPath
		if err := got.UnmarshalJSON(encoded); err != nil {
			t.Fatalf("ImportPath.UnmarshalJSON() error = %v, want nil", err)
		}
		if got != path || string(encoded) != `"example.com/project/internal/tool"` {
			t.Fatalf("ImportPath JSON round trip = (%v, %q), want (%v, canonical string)", got, encoded, path)
		}
	})

	t.Run("negative rejected import JSON preserves populated receiver", func(t *testing.T) {
		t.Parallel()

		got, err := gomodule.ParseImportPath("example.com/original")
		if err != nil {
			t.Fatalf("gomodule.ParseImportPath() error = %v, want nil", err)
		}
		want := got
		gotErr := got.UnmarshalJSON([]byte(`"net//http"`))
		if !errors.Is(gotErr, core.ErrGoModuleContract) || got != want {
			t.Fatalf("ImportPath.UnmarshalJSON(rejected) = (%v, %v), want preserved %v and errors.Is(..., %v)", got, gotErr, want, core.ErrGoModuleContract)
		}
	})

	t.Run("neutral zero import path refuses outward projection", func(t *testing.T) {
		t.Parallel()

		var path gomodule.ImportPath
		encoded, gotErr := path.MarshalJSON()
		if !errors.Is(gotErr, core.ErrGoModuleContract) || encoded != nil {
			t.Fatalf("zero ImportPath.MarshalJSON() = (%q, %v), want nil and errors.Is(..., %v)", encoded, gotErr, core.ErrGoModuleContract)
		}
	})
}

func FuzzPathJSONSemanticClosure(f *testing.F) {
	seed, err := gomodule.ParsePath("example.com/project/v2")
	if err != nil {
		f.Fatalf("gomodule.ParsePath(seed) error = %v, want nil", err)
	}
	canonical, err := seed.MarshalJSON()
	if err != nil {
		f.Fatalf("Path.MarshalJSON(seed) error = %v, want nil", err)
	}
	f.Add(canonical)
	f.Add([]byte{})
	f.Add([]byte(`{}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		got := seed
		gotErr := got.UnmarshalJSON(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrGoModuleContract) || got != seed {
				t.Fatalf("Path.UnmarshalJSON(rejected) = (%v, %v), want preserved seed and typed rejection", got, gotErr)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("Path.UnmarshalJSON(accepted).Validate() error = %v, want nil", err)
		}
		encoded, err := got.MarshalJSON()
		if err != nil || len(encoded) > gomodule.PathMaximumBytes+2 {
			t.Fatalf("Path.MarshalJSON(accepted) = (%d bytes, %v), want bounded and nil", len(encoded), err)
		}
		var roundTrip gomodule.Path
		if err := roundTrip.UnmarshalJSON(encoded); err != nil || roundTrip != got {
			t.Fatalf("Path canonical round trip = (%v, %v), want (%v, nil)", roundTrip, err, got)
		}
		second, err := roundTrip.MarshalJSON()
		if err != nil || string(second) != string(encoded) {
			t.Fatalf("Path second canonical projection = (%q, %v), want (%q, nil)", second, err, encoded)
		}
	})
}

func FuzzImportPathJSONSemanticClosure(f *testing.F) {
	seed, err := gomodule.ParseImportPath("example.com/project/internal/tool")
	if err != nil {
		f.Fatalf("gomodule.ParseImportPath(seed) error = %v, want nil", err)
	}
	canonical, err := seed.MarshalJSON()
	if err != nil {
		f.Fatalf("ImportPath.MarshalJSON(seed) error = %v, want nil", err)
	}
	f.Add(canonical)
	f.Add([]byte{})
	f.Add([]byte(`{}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		got := seed
		gotErr := got.UnmarshalJSON(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrGoModuleContract) || got != seed {
				t.Fatalf("ImportPath.UnmarshalJSON(rejected) = (%v, %v), want preserved seed and typed rejection", got, gotErr)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("ImportPath.UnmarshalJSON(accepted).Validate() error = %v, want nil", err)
		}
		encoded, err := got.MarshalJSON()
		if err != nil || len(encoded) > gomodule.ImportPathMaximumBytes+2 {
			t.Fatalf("ImportPath.MarshalJSON(accepted) = (%d bytes, %v), want bounded and nil", len(encoded), err)
		}
		var roundTrip gomodule.ImportPath
		if err := roundTrip.UnmarshalJSON(encoded); err != nil || roundTrip != got {
			t.Fatalf("ImportPath canonical round trip = (%v, %v), want (%v, nil)", roundTrip, err, got)
		}
		second, err := roundTrip.MarshalJSON()
		if err != nil || string(second) != string(encoded) {
			t.Fatalf("ImportPath second canonical projection = (%q, %v), want (%q, nil)", second, err, encoded)
		}
	})
}
