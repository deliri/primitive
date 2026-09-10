package objectstore

import (
	"embed"
	"errors"
	"reflect"

	"github.com/deliri/primitive/v2026/core"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"slices"
	"strconv"
	"strings"
	"testing"
)

//go:embed *.go
var objectstoreContractSources embed.FS

func TestObjectstoreHTTPDoorArchitectureLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive real production delegates every HTTP execution door", func(t *testing.T) {
		t.Parallel()

		files, err := fs.Glob(objectstoreContractSources, "*.go")
		if err != nil {
			t.Fatalf("fs.Glob(embedded sources) error = %v, want nil", err)
		}
		var got []string
		for _, path := range files {
			if strings.HasSuffix(path, "_test.go") {
				continue
			}
			source, err := objectstoreContractSources.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			imports, err := objectstoreHTTPImports(path, source)
			if err != nil {
				t.Fatalf("objectstoreHTTPImports(%q) error = %v, want nil", path, err)
			}
			got = append(got, imports...)
		}
		if len(got) != 0 {
			t.Fatalf("Objectstore production net/http doors = %q, want none because Exchange owns execution", got)
		}
	})

	t.Run("negative direct aliased and subpackage HTTP mutations are all visible", func(t *testing.T) {
		t.Parallel()

		cases := []struct {
			name string
			code string
			want []string
		}{
			{name: "direct net http import", code: "package synthetic\nimport \"net/http\"\n", want: []string{"net/http"}},
			{name: "aliased net http import", code: "package synthetic\nimport wire \"net/http\"\n", want: []string{"net/http"}},
			{name: "net http subpackage", code: "package synthetic\nimport \"net/http/httptest\"\n", want: []string{"net/http/httptest"}},
		}
		for _, tc := range cases {
			got, err := objectstoreHTTPImports("synthetic.go", []byte(tc.code))
			if err != nil || !slices.Equal(got, tc.want) {
				t.Fatalf("objectstoreHTTPImports(%s) = (%q, %v), want (%q, nil)", tc.name, got, err, tc.want)
			}
		}
	})

	t.Run("neutral neighboring typed and MIME imports create no execution finding", func(t *testing.T) {
		t.Parallel()

		for _, code := range []string{
			"package synthetic\nimport \"github.com/deliri/primitive/v2026/exchange\"\n",
			"package synthetic\nimport \"net/textproto\"\n",
		} {
			got, err := objectstoreHTTPImports("synthetic.go", []byte(code))
			if err != nil || len(got) != 0 {
				t.Fatalf("objectstoreHTTPImports(neutral) = (%q, %v), want (nil, nil)", got, err)
			}
		}
	})
}

func objectstoreHTTPImports(filename string, source []byte) ([]string, error) {
	file, err := parser.ParseFile(token.NewFileSet(), filename, source, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}
	var imports []string
	for _, specification := range file.Imports {
		path, err := strconv.Unquote(specification.Path.Value)
		if err != nil {
			return nil, err
		}
		if path == "net/http" || strings.HasPrefix(path, "net/http/") {
			imports = append(imports, path)
		}
	}
	slices.Sort(imports)
	return imports, nil
}

func TestProviderOperationsAreCompilerSelectedEntryPoints(t *testing.T) {
	t.Parallel()

	set := token.NewFileSet()
	var got []string
	for _, path := range []string{"client.go", "capability_execution.go", "transfer_evidence.go"} {
		source, readErr := objectstoreContractSources.ReadFile(path)
		if readErr != nil {
			t.Fatal(readErr)
		}
		file, gotParseErr := parser.ParseFile(set, path, source, 0)
		if gotParseErr != nil {
			t.Fatalf("parser.ParseFile(%s) error = %v, want nil", path, gotParseErr)
		}
		for _, declaration := range file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Recv != nil || !ast.IsExported(function.Name.Name) {
				continue
			}
			got = append(got, function.Name.Name)
		}
	}
	slices.Sort(got)
	want := []string{
		"Download",
		"DownloadGCS",
		"DownloadS3",
		"NewClient",
		"NewStandardClient",
		"Upload",
		"UploadCloudflareImages",
		"UploadGCS",
		"UploadS3",
		"VerifyProviderUpload",
	}
	if !slices.Equal(got, want) {
		t.Fatalf(
			"exported client operations = %q, want exactly %q with generic dispatch confined to received capabilities",
			got,
			want,
		)
	}
}

// All named production types are covered, including defined projections,
// enums, and consumer interfaces. This avoids reimplementing Go type resolution
// just to discover whether a named type eventually underlies a struct.
type inventoryRole uint8

const (
	inventoryProtocol inventoryRole = iota + 1
	inventoryWire
	inventoryFlow
	inventoryCapability
)

type inventoryBinding struct {
	typ  reflect.Type
	role inventoryRole
}

func objectstoreTypeBindings() []inventoryBinding {
	return []inventoryBinding{
		{typ: reflect.TypeFor[BLAKE3Digest](), role: inventoryCapability},
		{typ: reflect.TypeFor[UploadCapabilityRequest](), role: inventoryProtocol},
		{typ: reflect.TypeFor[DownloadCapabilityRequest](), role: inventoryProtocol},
		{typ: reflect.TypeFor[Client](), role: inventoryCapability},
		{typ: reflect.TypeFor[Transfer](), role: inventoryCapability},
		{typ: reflect.TypeFor[preparedDownload](), role: inventoryFlow},
		{typ: reflect.TypeFor[exactDownloadWriter](), role: inventoryCapability},
		{typ: reflect.TypeFor[preparedUpload](), role: inventoryFlow},
		{typ: reflect.TypeFor[requestBody](), role: inventoryFlow},
		{typ: reflect.TypeFor[uploadObservedSource](), role: inventoryFlow},
		{typ: reflect.TypeFor[framedUploadSource](), role: inventoryFlow},
		{typ: reflect.TypeFor[uploadConfirmation](), role: inventoryFlow},
		{typ: reflect.TypeFor[transferConfirmation](), role: inventoryFlow},
		{typ: reflect.TypeFor[exchangeTarget](), role: inventoryFlow},
		{typ: reflect.TypeFor[streamDigests](), role: inventoryFlow},
		{typ: reflect.TypeFor[DownloadCapability](), role: inventoryCapability},
		{typ: reflect.TypeFor[DownloadCapabilityProjection](), role: inventoryCapability},
		{typ: reflect.TypeFor[DownloadCapabilityCommitment](), role: inventoryCapability},
		{typ: reflect.TypeFor[ExactReader](), role: inventoryCapability},
		{typ: reflect.TypeFor[exactRemainingSource](), role: inventoryFlow},
		{typ: reflect.TypeFor[exactLengthSource](), role: inventoryFlow},
		{typ: reflect.TypeFor[exactFileSource](), role: inventoryFlow},
		{typ: reflect.TypeFor[signedHeaderDeclaration](), role: inventoryFlow},
		{typ: reflect.TypeFor[callerSignedHeaderValidation](), role: inventoryFlow},
		{typ: reflect.TypeFor[sentHeaderNames](), role: inventoryFlow},
		{typ: reflect.TypeFor[providerHeader](), role: inventoryFlow},
		{typ: reflect.TypeFor[amazonS3ChecksumType](), role: inventoryFlow},
		{typ: reflect.TypeFor[InspectionRequest](), role: inventoryProtocol},
		{typ: reflect.TypeFor[Inspection](), role: inventoryCapability},
		{typ: reflect.TypeFor[inspectionCopier](), role: inventoryFlow},
		{typ: reflect.TypeFor[TransferProgress](), role: inventoryProtocol},
		{typ: reflect.TypeFor[ProgressObserver](), role: inventoryProtocol},
		{typ: reflect.TypeFor[progressWriter](), role: inventoryFlow},
		{typ: reflect.TypeFor[transferEvidenceWire](), role: inventoryWire},
		{typ: reflect.TypeFor[TransferEvidence](), role: inventoryCapability},
		{typ: reflect.TypeFor[TransferEvidenceProjection](), role: inventoryCapability},
		{typ: reflect.TypeFor[ProviderUploadObservationRequest](), role: inventoryProtocol},
		{typ: reflect.TypeFor[VerifiedProviderUpload](), role: inventoryCapability},
		{typ: reflect.TypeFor[UploadCapability](), role: inventoryCapability},
		{typ: reflect.TypeFor[UploadCapabilityProjection](), role: inventoryCapability},
		{typ: reflect.TypeFor[UploadCapabilityCommitment](), role: inventoryCapability},
		{typ: reflect.TypeFor[uploadCapabilityWire](), role: inventoryWire},
		{typ: reflect.TypeFor[uploadCapabilityHeaderWire](), role: inventoryWire},
		{typ: reflect.TypeFor[UploadHTTPProjection](), role: inventoryCapability},
		{typ: reflect.TypeFor[uploadHTTPProjectionWire](), role: inventoryWire},
		{typ: reflect.TypeFor[Direction](), role: inventoryProtocol},
		{typ: reflect.TypeFor[Commitment](), role: inventoryProtocol},
		{typ: reflect.TypeFor[ProviderVersion](), role: inventoryCapability},
		{typ: reflect.TypeFor[SignedURL](), role: inventoryCapability},
		{typ: reflect.TypeFor[SignedHeader](), role: inventoryCapability},
		{typ: reflect.TypeFor[SignedHeaders](), role: inventoryCapability},
		{typ: reflect.TypeFor[UploadTarget](), role: inventoryProtocol},
		{typ: reflect.TypeFor[DownloadTarget](), role: inventoryProtocol},
		{typ: reflect.TypeFor[Integrity](), role: inventoryProtocol},
		{typ: reflect.TypeFor[Policy](), role: inventoryProtocol},
		{typ: reflect.TypeFor[UploadRequest](), role: inventoryProtocol},
		{typ: reflect.TypeFor[DownloadRequest](), role: inventoryProtocol},
		{typ: reflect.TypeFor[Provider](), role: inventoryProtocol},
		{typ: reflect.TypeFor[VendorAPI](), role: inventoryProtocol},
		{typ: reflect.TypeFor[DirectionCapability](), role: inventoryProtocol},
		{typ: reflect.TypeFor[UploadEncoding](), role: inventoryProtocol},
		{typ: reflect.TypeFor[ProviderIntegrity](), role: inventoryProtocol},
		{typ: reflect.TypeFor[WritePreference](), role: inventoryProtocol},
		{typ: reflect.TypeFor[VendorSpec](), role: inventoryProtocol},
	}
}

type sourceTypeInventory struct {
	names            []string
	local, anonymous int
}

func TestProductionTypesHaveCompilerOwnedRoles(t *testing.T) {
	t.Parallel()
	paths, err := fs.Glob(objectstoreContractSources, "*.go")
	if err != nil {
		t.Fatal(err)
	}
	var files []*ast.File
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		data, err := objectstoreContractSources.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		file, err := parser.ParseFile(token.NewFileSet(), path, data, 0)
		if err != nil {
			t.Fatal(err)
		}
		files = append(files, file)
	}
	got := inventorySourceTypes(files)
	want, err := inventoryBoundNames(objectstoreTypeBindings())
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got.names, want) || got.local != 0 || got.anonymous != 0 {
		t.Fatalf("production types=%v, local=%d anonymous=%d; want exactly bound types=%v and no unowned carriers", got.names, got.local, got.anonymous, want)
	}
}

func inventorySourceTypes(files []*ast.File) sourceTypeInventory {
	var got sourceTypeInventory
	top := make(map[*ast.TypeSpec]bool)
	named := make(map[*ast.StructType]bool)
	for _, file := range files {
		for _, declaration := range file.Decls {
			group, ok := declaration.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, raw := range group.Specs {
				if spec, ok := raw.(*ast.TypeSpec); ok {
					top[spec] = true
					got.names = append(got.names, spec.Name.Name)
				}
			}
		}
		ast.Inspect(file, func(node ast.Node) bool {
			if spec, ok := node.(*ast.TypeSpec); ok {
				if !top[spec] {
					got.local++
				}
				if literal, ok := ast.Unparen(spec.Type).(*ast.StructType); ok {
					named[literal] = true
				}
			}
			if literal, ok := node.(*ast.StructType); ok && !named[literal] {
				got.anonymous++
			}
			return true
		})
	}
	slices.Sort(got.names)
	return got
}

func inventoryBoundNames(bindings []inventoryBinding) ([]string, error) {
	names := make([]string, 0, len(bindings))
	for _, binding := range bindings {
		if binding.typ == nil || binding.typ.PkgPath() != reflect.TypeFor[Client]().PkgPath() || binding.typ.Name() == "" {
			return nil, core.ErrObjectStoreContract
		}
		switch binding.role {
		case inventoryProtocol, inventoryWire, inventoryFlow, inventoryCapability:
		default:
			return nil, core.ErrObjectStoreContract
		}
		names = append(names, binding.typ.Name())
	}
	slices.Sort(names)
	for i := 1; i < len(names); i++ {
		if names[i] == names[i-1] {
			return nil, core.ErrObjectStoreContract
		}
	}
	return names, nil
}

func TestInventoryScannerHostileShapeTable(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, source     string
		names            []string
		local, anonymous int
	}{
		{name: "named struct is visible", source: "package p; type A struct{}", names: []string{"A"}},
		{name: "defined projection cannot hide behind another name", source: "package p; type A struct{}; type B A", names: []string{"A", "B"}},
		{name: "alias remains visible for exact compiler binding refusal", source: "package p; type A struct{}; type B = A", names: []string{"A", "B"}},
		{name: "local struct cannot masquerade as package-owned", source: "package p; func f(){type A struct{}}", local: 1},
		{name: "anonymous carrier inside named container remains unowned", source: "package p; type A struct { Inner struct{} }", names: []string{"A"}, anonymous: 1},
		{name: "anonymous array element cannot disappear", source: "package p; type A []struct{}", names: []string{"A"}, anonymous: 1},
		{name: "scalar and interface contracts require ownership too", source: "package p; type A uint8; type B interface{ Read() }", names: []string{"A", "B"}},
		{name: "ordinary function adds no invented type", source: "package p; func f(){}"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			file, err := parser.ParseFile(token.NewFileSet(), "fixture.go", tc.source, 0)
			if err != nil {
				t.Fatal(err)
			}
			got := inventorySourceTypes([]*ast.File{file})
			if !slices.Equal(got.names, tc.names) || got.local != tc.local || got.anonymous != tc.anonymous {
				t.Fatalf("inventory=%+v, want names=%v local=%d anonymous=%d", got, tc.names, tc.local, tc.anonymous)
			}
		})
	}
}

func TestInventoryBindingHostileTable(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		bindings []inventoryBinding
		want     []string
		wantErr  error
	}{
		{name: "compiler-owned public type", bindings: []inventoryBinding{{reflect.TypeFor[Client](), inventoryCapability}}, want: []string{"Client"}},
		{name: "compiler-owned private flow", bindings: []inventoryBinding{{reflect.TypeFor[inspectionCopier](), inventoryFlow}}, want: []string{"inspectionCopier"}},
		{name: "empty inventory does not invent ownership"},
		{name: "absent type is refused", bindings: []inventoryBinding{{role: inventoryFlow}}, wantErr: core.ErrObjectStoreContract},
		{name: "foreign type cannot satisfy local contract", bindings: []inventoryBinding{{reflect.TypeFor[core.ByteLength](), inventoryProtocol}}, wantErr: core.ErrObjectStoreContract},
		{name: "pointer is not a named local declaration", bindings: []inventoryBinding{{reflect.TypeFor[*Client](), inventoryCapability}}, wantErr: core.ErrObjectStoreContract},
		{name: "missing role cannot classify a type", bindings: []inventoryBinding{{typ: reflect.TypeFor[Client]()}}, wantErr: core.ErrObjectStoreContract},
		{name: "unknown future role is refused", bindings: []inventoryBinding{{reflect.TypeFor[Client](), inventoryRole(255)}}, wantErr: core.ErrObjectStoreContract},
		{name: "duplicate types cannot inflate ownership", bindings: []inventoryBinding{{reflect.TypeFor[Client](), inventoryCapability}, {reflect.TypeFor[Client](), inventoryFlow}}, wantErr: core.ErrObjectStoreContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := inventoryBoundNames(tc.bindings)
			if !errors.Is(err, tc.wantErr) || !slices.Equal(got, tc.want) {
				t.Fatalf("bound names=(%v,%v), want (%v,%v)", got, err, tc.want, tc.wantErr)
			}
		})
	}
}
