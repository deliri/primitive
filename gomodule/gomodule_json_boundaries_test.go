package gomodule_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
)

func TestJSONRepresentationLayerTriad(t *testing.T) {
	t.Parallel()
	seed, err := gomodule.ParsePath("example.com/project/v2")
	if err != nil {
		t.Fatalf("ParsePath(seed) error = %v, want nil", err)
	}
	canonical, err := seed.MarshalJSON()
	if err != nil {
		t.Fatalf("Path.MarshalJSON(seed) error = %v, want nil", err)
	}
	cases := []struct {
		name    string
		input   string
		wantErr error
	}{
		{name: "canonical typed document", input: string(canonical)},
		{name: "surrounding JSON whitespace", input: " \n\t" + string(canonical) + "\r "},
		{name: "escaped path separator preserves identity", input: strings.ReplaceAll(string(canonical), "/", `\/`)},
		{name: "escaped ASCII preserves identity", input: strings.Replace(string(canonical), "e", `\u0065`, 1)},
		{name: "absent document preserves receiver", wantErr: core.ErrJSONContract},
		{name: "null does not erase identity", input: "null", wantErr: core.ErrJSONContract},
		{name: "whitespace null does not erase identity", input: " \nnull\t", wantErr: core.ErrJSONContract},
		{name: "unterminated typed string", input: string(canonical[:len(canonical)-1]), wantErr: core.ErrJSONContract},
		{name: "second document is refused", input: string(canonical) + string(canonical), wantErr: core.ErrJSONContract},
		{name: "object cannot replace scalar", input: "{}", wantErr: core.ErrJSONContract},
		{name: "array cannot replace scalar", input: "[]", wantErr: core.ErrJSONContract},
		{name: "number cannot replace scalar", input: "1", wantErr: core.ErrJSONContract},
		{name: "boolean cannot replace scalar", input: "true", wantErr: core.ErrJSONContract},
		{name: "empty decoded identity is refused", input: `""`, wantErr: core.ErrGoModuleContract},
		{name: "escaped parent traversal is refused", input: `"example.com/\u002e\u002e/project"`, wantErr: core.ErrGoModuleContract},
		{name: "unpaired surrogate is refused", input: `"example.com/\ud800"`, wantErr: core.ErrJSONContract},
		{name: "raw invalid UTF8 is refused", input: "\"example.com/\xff\"", wantErr: core.ErrJSONContract},
		{name: "embedded NUL is refused by path grammar", input: `"example.com/\u0000"`, wantErr: core.ErrGoModuleContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := seed
			gotErr := got.UnmarshalJSON([]byte(tc.input))
			if !errors.Is(gotErr, tc.wantErr) || got != seed {
				t.Fatalf("Path.UnmarshalJSON() = (%v, %v), want (%v, %v)", got, gotErr, seed, tc.wantErr)
			}
			if tc.wantErr != nil && !errors.Is(gotErr, core.ErrGoModuleContract) {
				t.Fatalf("Path.UnmarshalJSON() error = %v, want %v", gotErr, core.ErrGoModuleContract)
			}
			wantImport, err := gomodule.ParseImportPath(seed.String())
			if err != nil {
				t.Fatalf("ParseImportPath(seed) error = %v, want nil", err)
			}
			gotImport := wantImport
			gotErr = gotImport.UnmarshalJSON([]byte(tc.input))
			if !errors.Is(gotErr, tc.wantErr) || gotImport != wantImport {
				t.Fatalf("ImportPath.UnmarshalJSON() = (%v, %v), want (%v, %v)", gotImport, gotErr, wantImport, tc.wantErr)
			}
			if tc.wantErr != nil && !errors.Is(gotErr, core.ErrGoModuleContract) {
				t.Fatalf("ImportPath.UnmarshalJSON() error = %v, want %v", gotErr, core.ErrGoModuleContract)
			}
		})
	}
}

func TestNilJSONReceiversRefuseWithoutPanic(t *testing.T) {
	t.Parallel()
	var path *gomodule.Path
	if err := path.UnmarshalJSON(nil); !errors.Is(err, core.ErrGoModuleContract) {
		t.Fatalf("nil Path.UnmarshalJSON() error = %v, want %v", err, core.ErrGoModuleContract)
	}
	var imported *gomodule.ImportPath
	if err := imported.UnmarshalJSON(nil); !errors.Is(err, core.ErrGoModuleContract) {
		t.Fatalf("nil ImportPath.UnmarshalJSON() error = %v, want %v", err, core.ErrGoModuleContract)
	}
}
