package gotoolchain

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/scanner"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func sourceFileRequestForTest(t testing.TB, directory, body string) FileParseRequest {
	t.Helper()
	path := filepath.Join(directory, "source.go")
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	absolute, err := core.ParseAbsolutePath(path)
	if err != nil {
		t.Fatal(err)
	}
	return FileParseRequest{Path: absolute, FileSet: token.NewFileSet()}
}

func TestSourceFileParsingAgreesWithNativeGo(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, body string }{
		{name: "package only", body: "package sample\n"},
		{name: "typed constant", body: "package sample\nconst Value uint8 = 1\n"},
		{name: "declared struct", body: "package sample\ntype Value struct{ Count int }\n"},
		{name: "receiver method", body: "package sample\ntype Value int\nfunc (v Value) Read() int {return int(v)}\n"},
		{name: "type alias", body: "package sample\ntype Value = uint8\n"},
		{name: "generic declaration", body: "package sample\nfunc Identity[T any](v T) T{return v}\n"},
		{name: "comments retain source ownership", body: "// Package sample documents its source.\npackage sample\n// Value is observed.\nconst Value = 1\n"},
		{name: "build directive remains source data", body: "//go:build not_this_host\n\npackage sample\n"},
		{name: "foreign line directive cannot hide physical position", body: "package sample\n//line forged.go:900\nconst Value = 1\n"},
		{name: "type errors remain compiler work", body: "package sample\nvar Value int = \"wrong type\"\n"},
		{name: "empty source"},
		{name: "missing package", body: "var Value = 1\n"},
		{name: "truncated declaration", body: "package sample\nvar =\n"},
		{name: "truncated function", body: "package sample\nfunc Broken( {\n"},
		{name: "unterminated string", body: "package sample\nvar Text = \"unfinished\n"},
		{name: "unterminated comment", body: "package sample\n/* unfinished"},
		{name: "nul source", body: "package sample\n\x00"},
		{name: "invalid utf8", body: "package sample\n\xff"},
		{name: "missing struct delimiter", body: "package sample\ntype Value struct{Count int\n"},
		{name: "trailing foreign token", body: "package sample\n}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request := sourceFileRequestForTest(t, t.TempDir(), tc.body)
			proveSourceFileNativeAgreement(t, request, tc.body)
		})
	}
}

func proveSourceFileNativeAgreement(t testing.TB, request FileParseRequest, body string) {
	t.Helper()
	oracleSet := token.NewFileSet()
	oracle, oracleErr := parser.ParseFile(oracleSet, request.Path.String(), body, parser.ParseComments)
	got, err := ParseFile(t.Context(), request)
	if (err == nil) != (oracleErr == nil) {
		t.Fatalf("ParseFile() error=%v, want native outcome %v", err, oracleErr)
	}
	if oracleErr != nil {
		var wantErrors, gotErrors scanner.ErrorList
		if got != nil || !errors.Is(err, core.ErrGoToolchainOutput) || !errors.As(oracleErr, &wantErrors) || !errors.As(err, &gotErrors) || !slices.EqualFunc(gotErrors, wantErrors, func(a, b *scanner.Error) bool { return *a == *b }) {
			t.Fatalf("ParseFile() rejected syntax=%v error=%v, want zero syntax and every native error %v", got, err, oracleErr)
		}
		return
	}
	if got == nil || got.Name.Name != oracle.Name.Name || len(got.Decls) != len(oracle.Decls) || len(got.Comments) != len(oracle.Comments) {
		t.Fatalf("ParseFile() syntax=%+v, want native package/declarations/comments %+v", got, oracle)
	}
	var actual, expected bytes.Buffer
	if err := format.Node(&actual, request.FileSet, got); err != nil {
		t.Fatal(err)
	}
	if err := format.Node(&expected, oracleSet, oracle); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actual.Bytes(), expected.Bytes()) {
		t.Fatalf("ParseFile() rendered syntax=%q, want native %q", actual.Bytes(), expected.Bytes())
	}
	for i, declaration := range got.Decls {
		gotPosition := request.FileSet.PositionFor(declaration.Pos(), false)
		wantPosition := oracleSet.PositionFor(oracle.Decls[i].Pos(), false)
		if gotPosition != wantPosition {
			t.Fatalf("ParseFile() physical position=%v, want %v", gotPosition, wantPosition)
		}
	}
}

func TestSourceFileParsingPreservesNativeScopeAndWindowCrossings(t *testing.T) {
	t.Parallel()
	for _, size := range []int{(32 << 10) - 1, 32 << 10, (32 << 10) + 1, (64 << 10) - 1, 64 << 10, (64 << 10) + 1, 8 << 20} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			t.Parallel()
			body := "package sample\n/*" + strings.Repeat("x", size) + "*/\ntype Record int\nvar Value Record\n"
			request := sourceFileRequestForTest(t, t.TempDir(), body)
			proveSourceFileNativeAgreement(t, request, body)
			request.FileSet = token.NewFileSet()
			file, err := ParseFile(t.Context(), request)
			if err != nil {
				t.Fatal(err)
			}
			declaration := file.Decls[len(file.Decls)-1].(*ast.GenDecl).Specs[0].(*ast.ValueSpec)
			reference := declaration.Type.(*ast.Ident)
			if reference.Obj == nil || reference.Obj != file.Scope.Lookup("Record") {
				t.Fatalf("ParseFile() discarded native object identity: %+v", reference)
			}
		})
	}
}

func TestSourceFileParsingRefusesUnownedAndCancelledRequests(t *testing.T) {
	t.Parallel()
	request := sourceFileRequestForTest(t, t.TempDir(), "package sample\n")
	for _, tc := range []struct {
		name    string
		request FileParseRequest
	}{
		{name: "missing position owner", request: FileParseRequest{Path: request.Path}},
		{name: "missing path owner", request: FileParseRequest{FileSet: token.NewFileSet()}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			file, err := ParseFile(t.Context(), tc.request)
			if file != nil || !errors.Is(err, core.ErrGoToolchainContract) {
				t.Fatalf("ParseFile()=%v/%v, want zero and contract refusal", file, err)
			}
		})
	}
	ctx, cancel := context.WithCancelCause(t.Context())
	cancel(fs.ErrPermission)
	file, err := ParseFile(ctx, request)
	if file != nil || !errors.Is(err, context.Canceled) || !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("ParseFile() cancelled=%v/%v, want owning causes", file, err)
	}
	file, err = ParseFile(nil, request)
	if file != nil || !errors.Is(err, core.ErrGoToolchainContract) {
		t.Fatalf("ParseFile() nil context=%v/%v", file, err)
	}
	missing, err := core.ParseAbsolutePath(filepath.Join(t.TempDir(), "absent.go"))
	if err != nil {
		t.Fatal(err)
	}
	file, err = ParseFile(t.Context(), FileParseRequest{Path: missing, FileSet: token.NewFileSet()})
	var nativePath *fs.PathError
	if file != nil || !errors.Is(err, fs.ErrNotExist) || !errors.Is(err, core.ErrGoToolchainExecution) || !errors.As(err, &nativePath) {
		t.Fatalf("ParseFile() missing=%v/%v, want native missing path", file, err)
	}
}

func FuzzSourceFileParsingPreservesNativeGrammarAndPositions(f *testing.F) {
	seed := sourceFileRequestForTest(f, f.TempDir(), "package sample\nconst Value = 1\n")
	file, err := ParseFile(f.Context(), seed)
	if err != nil {
		f.Fatal(err)
	}
	var canonical bytes.Buffer
	if err := format.Node(&canonical, seed.FileSet, file); err != nil {
		f.Fatal(err)
	}
	f.Add(canonical.String())
	f.Add("package sample\nvar =\n")
	f.Add("")
	f.Fuzz(func(t *testing.T, body string) {
		request := sourceFileRequestForTest(t, t.TempDir(), body)
		proveSourceFileNativeAgreement(t, request, body)
	})
}
