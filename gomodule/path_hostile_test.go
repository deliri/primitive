package gomodule_test

import (
	jsonv2 "encoding/json/v2"
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
	"golang.org/x/mod/module"
)

const longPathFixtureBytes = 1 << 20

func TestParsePathHostileDomain(t *testing.T) {
	t.Parallel()

	prefix := "example.com/"
	longPath := prefix + strings.Repeat("a", longPathFixtureBytes-len(prefix))
	cases := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "valid ordinary repository", value: "example.com/project"},
		{name: "valid subdomain repository", value: "code.example.com/project"},
		{name: "valid major version suffix", value: "example.com/project/v2"},
		{name: "valid gopkg version form", value: "gopkg.in/yaml.v3"},
		{name: "valid hyphenated element", value: "example.com/project-name"},
		{name: "valid underscored element", value: "example.com/project_name"},
		{name: "valid nondigit tilde suffix", value: "example.com/project~next"},
		{name: "valid uppercase repository element", value: "example.com/Project"},
		{name: "valid dotted repository element", value: "example.com/project.name"},
		{name: "valid punycode domain", value: "xn--bcher-kva.example/project"},

		{name: "reject absent path", value: "", wantErr: true},
		{name: "reject first element without dot", value: "example/project", wantErr: true},
		{name: "reject leading slash", value: "/example.com/project", wantErr: true},
		{name: "reject trailing slash", value: "example.com/project/", wantErr: true},
		{name: "reject empty middle element", value: "example.com//project", wantErr: true},
		{name: "reject uppercase domain", value: "Example.com/project", wantErr: true},
		{name: "reject leading dash domain", value: "-example.com/project", wantErr: true},
		{name: "reject hidden repository element", value: "example.com/.project", wantErr: true},
		{name: "reject trailing dot element", value: "example.com/project.", wantErr: true},
		{name: "reject shell punctuation", value: "example.com/project@next", wantErr: true},

		{name: "historical size cap remains absent", value: longPath + strings.Repeat("a", longPathFixtureBytes), wantErr: false},
		{name: "boundary version one refused", value: "example.com/project/v1", wantErr: true},
		{name: "boundary version leading zero refused", value: "example.com/project/v02", wantErr: true},
		{name: "boundary large major version accepted", value: "example.com/project/v999999"},
		{name: "boundary Windows con element refused", value: "example.com/con", wantErr: true},
		{name: "boundary Windows console element accepted", value: "example.com/console"},
		{name: "boundary Windows com zero element accepted", value: "example.com/com0"},
		{name: "boundary short name one refused", value: "example.com/projec~1", wantErr: true},
		{name: "boundary bare tilde accepted", value: "example.com/project~"},
		{name: "boundary all dot element refused", value: "example.com/..", wantErr: true},
	}

	for _, tc := range cases {
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
	longPath := prefix + strings.Repeat("a", longPathFixtureBytes-len(prefix))
	cases := []struct {
		name    string
		value   string
		wantErr bool
	}{
		{name: "valid standard library root", value: "fmt"},
		{name: "valid standard library child", value: "net/http"},
		{name: "valid unicode standard library child", value: "unicode/utf8"},
		{name: "valid module root package", value: "example.com/project"},
		{name: "valid module child package", value: "example.com/project/internal/tool"},
		{name: "valid major version package", value: "example.com/project/v2"},
		{name: "valid hyphenated package element", value: "example.com/project-name"},
		{name: "valid underscored package element", value: "example.com/project_name"},
		{name: "valid dotted package element", value: "example.com/project.name"},
		{name: "valid gopkg package identity", value: "gopkg.in/yaml.v3"},

		{name: "reject absent import path", value: "", wantErr: true},
		{name: "reject leading slash", value: "/fmt", wantErr: true},
		{name: "reject trailing slash", value: "fmt/", wantErr: true},
		{name: "reject empty middle element", value: "net//http", wantErr: true},
		{name: "reject backslash separator", value: `net\http`, wantErr: true},
		{name: "reject embedded whitespace", value: "net /http", wantErr: true},
		{name: "reject version punctuation", value: "example.com/project@v2", wantErr: true},
		{name: "reject current directory element", value: "net/./http", wantErr: true},
		{name: "reject parent directory element", value: "net/../http", wantErr: true},
		{name: "reject quote punctuation", value: `example.com/project"next`, wantErr: true},

		{name: "historical size cap remains absent", value: longPath + strings.Repeat("a", longPathFixtureBytes), wantErr: false},
		{name: "boundary one-character import accepted", value: "x"},
		{name: "boundary leading dot import accepted", value: ".x"},
		{name: "boundary leading dash refused", value: "-x", wantErr: true},
		{name: "boundary leading underscore accepted", value: "_x"},
		{name: "boundary trailing dash accepted", value: "x-"},
		{name: "boundary Windows con element refused", value: "con", wantErr: true},
		{name: "boundary Windows console element accepted", value: "console"},
		{name: "boundary Windows com zero accepted", value: "com0"},
		{name: "boundary Windows com one refused", value: "com1", wantErr: true},
		{name: "boundary uppercase package accepted", value: "Example"},
		{name: "boundary tilde package accepted", value: "project~next"},
		{name: "boundary percent punctuation refused", value: "project%next", wantErr: true},
		{name: "boundary NUL refused", value: "project\x00next", wantErr: true},
		{name: "boundary invalid UTF-8 refused", value: "project\xffnext", wantErr: true},
	}

	for _, tc := range cases {
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
		var source string
		sourceErr := jsonv2.Unmarshal(data, &source)
		wantAccepted := sourceErr == nil && source != "" && module.CheckPath(source) == nil
		got := seed
		gotErr := got.UnmarshalJSON(data)
		if (gotErr == nil) != wantAccepted {
			t.Fatalf("JSON admission = %v, want accepted %t for source %q", gotErr, wantAccepted, source)
		}
		if gotErr != nil {
			var fresh gomodule.Path
			freshErr := fresh.UnmarshalJSON(data)
			if !errors.Is(freshErr, core.ErrGoModuleContract) || fresh != (gomodule.Path{}) {
				t.Fatalf("fresh receiver refusal = (%v, %v), want zero and %v", fresh, freshErr, core.ErrGoModuleContract)
			}
			if !errors.Is(gotErr, core.ErrGoModuleContract) || got != seed {
				t.Fatalf("Path.UnmarshalJSON(rejected) = (%v, %v), want preserved seed and typed rejection", got, gotErr)
			}
			return
		}
		if got.String() != source {
			t.Fatalf("accepted source facts = %q, want %q", got.String(), source)
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("Path.UnmarshalJSON(accepted).Validate() error = %v, want nil", err)
		}
		encoded, err := got.MarshalJSON()
		if err != nil {
			t.Fatalf("Path.MarshalJSON(accepted) = (%d bytes, %v), want nil", len(encoded), err)
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
		var source string
		sourceErr := jsonv2.Unmarshal(data, &source)
		wantAccepted := sourceErr == nil && source != "" && module.CheckImportPath(source) == nil
		got := seed
		gotErr := got.UnmarshalJSON(data)
		if (gotErr == nil) != wantAccepted {
			t.Fatalf("JSON admission = %v, want accepted %t for source %q", gotErr, wantAccepted, source)
		}
		if gotErr != nil {
			var fresh gomodule.ImportPath
			freshErr := fresh.UnmarshalJSON(data)
			if !errors.Is(freshErr, core.ErrGoModuleContract) || fresh != (gomodule.ImportPath{}) {
				t.Fatalf("fresh receiver refusal = (%v, %v), want zero and %v", fresh, freshErr, core.ErrGoModuleContract)
			}
			if !errors.Is(gotErr, core.ErrGoModuleContract) || got != seed {
				t.Fatalf("ImportPath.UnmarshalJSON(rejected) = (%v, %v), want preserved seed and typed rejection", got, gotErr)
			}
			return
		}
		if got.String() != source {
			t.Fatalf("accepted source facts = %q, want %q", got.String(), source)
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("ImportPath.UnmarshalJSON(accepted).Validate() error = %v, want nil", err)
		}
		encoded, err := got.MarshalJSON()
		if err != nil {
			t.Fatalf("ImportPath.MarshalJSON(accepted) = (%d bytes, %v), want nil", len(encoded), err)
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
