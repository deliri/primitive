package objectstore_test

import (
	"embed"
	"encoding"
	json "encoding/json/v2"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/objectstore"
)

//go:embed *.go
var ingressSources embed.FS

type ingressFuzzBinding struct {
	receiver reflect.Type
	method   string
	campaign func(*testing.F)
}

func jsonIngressBinding[T any, P interface {
	*T
	json.Unmarshaler
}](campaign func(*testing.F)) ingressFuzzBinding {
	return ingressFuzzBinding{receiver: reflect.TypeFor[T](), method: reflect.TypeFor[json.Unmarshaler]().Method(0).Name, campaign: campaign}
}

func textIngressBinding[T any, P interface {
	*T
	encoding.TextUnmarshaler
}](campaign func(*testing.F)) ingressFuzzBinding {
	return ingressFuzzBinding{receiver: reflect.TypeFor[T](), method: reflect.TypeFor[encoding.TextUnmarshaler]().Method(0).Name, campaign: campaign}
}

func TestPublicDecoderFuzzInventory(t *testing.T) {
	t.Parallel()
	bindings := []ingressFuzzBinding{
		jsonIngressBinding[objectstore.BLAKE3Digest](FuzzBLAKE3DigestJSONSemanticClosure),
		textIngressBinding[objectstore.BLAKE3Digest](FuzzBLAKE3DigestTextSemanticClosure),
		jsonIngressBinding[objectstore.UploadCapability](objectstore.FuzzUploadCapabilityAdmitsOnlyTransferableCapabilities),
		jsonIngressBinding[objectstore.UploadCapabilityCommitment](objectstore.FuzzUploadCapabilityCommitmentJSONSemanticClosure),
		jsonIngressBinding[objectstore.DownloadCapability](objectstore.FuzzDownloadCapabilityJSONSemanticClosure),
		jsonIngressBinding[objectstore.DownloadCapabilityCommitment](objectstore.FuzzDownloadCapabilityCommitmentJSONSemanticClosure),
		jsonIngressBinding[objectstore.TransferEvidence](objectstore.FuzzTransferEvidenceJSONSemanticClosure),
	}
	var want []string
	for _, binding := range bindings {
		if binding.campaign == nil {
			t.Fatalf("%v.%s has no compiler-bound campaign", binding.receiver, binding.method)
		}
		want = append(want, binding.receiver.Name()+"."+binding.method)
	}
	paths, err := fs.Glob(ingressSources, "*.go")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		data, err := ingressSources.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, data, 0)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, publicDecoderNames(file)...)
	}
	slices.Sort(got)
	slices.Sort(want)
	if !slices.Equal(got, want) {
		t.Fatalf("public decoder methods=%v, compiler-bound fuzz inventory=%v", got, want)
	}

}

func publicDecoderNames(file *ast.File) []string {
	jsonMethod := reflect.TypeFor[json.Unmarshaler]().Method(0).Name
	textMethod := reflect.TypeFor[encoding.TextUnmarshaler]().Method(0).Name
	var names []string
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Recv == nil || (function.Name.Name != jsonMethod && function.Name.Name != textMethod) {
			continue
		}
		receiver := ast.Unparen(function.Recv.List[0].Type)
		if pointer, ok := receiver.(*ast.StarExpr); ok {
			receiver = ast.Unparen(pointer.X)
		}
		switch indexed := receiver.(type) {
		case *ast.IndexExpr:
			receiver = ast.Unparen(indexed.X)
		case *ast.IndexListExpr:
			receiver = ast.Unparen(indexed.X)
		}
		if identifier, ok := receiver.(*ast.Ident); ok && identifier.IsExported() {
			names = append(names, identifier.Name+"."+function.Name.Name)
		}
	}
	slices.Sort(names)
	return names
}

func TestPublicDecoderInventoryShapeTable(t *testing.T) {
	t.Parallel()
	jsonName := reflect.TypeFor[json.Unmarshaler]().Method(0).Name
	textName := reflect.TypeFor[encoding.TextUnmarshaler]().Method(0).Name
	for _, tc := range []struct {
		name, source string
		want         []string
	}{
		{name: "new public JSON decoder is discovered", source: "package p;func (*Added) " + jsonName + "([]byte)error{return nil}", want: []string{"Added." + jsonName}},
		{name: "value receiver remains visible", source: "package p;func (Added) " + textName + "([]byte)error{return nil}", want: []string{"Added." + textName}},
		{name: "parenthesized receiver cannot hide", source: "package p;func (x *(Added)) " + jsonName + "([]byte)error{return nil}", want: []string{"Added." + jsonName}},
		{name: "generic receiver remains visible", source: "package p;func (*Added[T]) " + jsonName + "([]byte)error{return nil}", want: []string{"Added." + jsonName}},
		{name: "multiple type parameters cannot hide", source: "package p;func (*Added[K,V]) " + jsonName + "([]byte)error{return nil}", want: []string{"Added." + jsonName}},
		{name: "private internal decoder creates no public door", source: "package p;func (*local) " + jsonName + "([]byte)error{return nil}"},
		{name: "ordinary method does not invent an ingress", source: "package p;func (*Added) Validate()error{return nil}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			file, err := parser.ParseFile(token.NewFileSet(), "fixture.go", tc.source, 0)
			if err != nil {
				t.Fatal(err)
			}
			got := publicDecoderNames(file)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("decoder discovery=%v,want %v", got, tc.want)
			}
		})
	}
}

// Bind the public textual parser and its campaign through compiler signatures.
var (
	_ func(string) (objectstore.SignedURL, error) = objectstore.ParseSignedURL
	_ func(*testing.F)                            = FuzzParseSignedURL
)
