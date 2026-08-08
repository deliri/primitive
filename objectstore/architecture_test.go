package objectstore

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"testing"
)

func TestProviderOperationsAreCompilerSelectedEntryPoints(t *testing.T) {
	t.Parallel()

	set := token.NewFileSet()
	file, gotParseErr := parser.ParseFile(set, "client.go", nil, 0)
	if gotParseErr != nil {
		t.Fatalf(
			"parser.ParseFile(client.go) error = %v, want nil",
			gotParseErr,
		)
	}
	var got []string
	for _, declaration := range file.Decls {
		function, ok := declaration.(*ast.FuncDecl)
		if !ok || function.Recv != nil || !ast.IsExported(function.Name.Name) {
			continue
		}
		got = append(got, function.Name.Name)
	}
	slices.Sort(got)
	want := []string{
		"DownloadGCS",
		"DownloadS3",
		"NewClient",
		"UploadCloudflareImages",
		"UploadGCS",
		"UploadS3",
	}
	if !slices.Equal(got, want) {
		t.Fatalf(
			"exported client operations = %q, want exactly %q with no generic provider dispatcher",
			got,
			want,
		)
	}
}

func TestProductionStructDataFlowInventory(t *testing.T) {
	t.Parallel()

	files, gotGlobErr := filepath.Glob("*.go")
	if gotGlobErr != nil {
		t.Fatalf("filepath.Glob() error = %v, want nil", gotGlobErr)
	}
	set := token.NewFileSet()
	found := 0
	for _, path := range files {
		if filepath.Ext(path) != ".go" ||
			len(path) >= len("_test.go") &&
				path[len(path)-len("_test.go"):] == "_test.go" {
			continue
		}
		file, gotParseErr := parser.ParseFile(set, path, nil, 0)
		if gotParseErr != nil {
			t.Fatalf(
				"parser.ParseFile(%q) error = %v, want nil",
				path,
				gotParseErr,
			)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			spec, ok := node.(*ast.TypeSpec)
			if !ok {
				return true
			}
			if _, ok := spec.Type.(*ast.StructType); !ok {
				return true
			}
			role, classified := productionStructRole(spec.Name.Name)
			if !classified || role == "" {
				t.Errorf(
					"production struct %s has role %q classified %t, want a precise data-flow role",
					spec.Name.Name,
					role,
					classified,
				)
			}
			found++
			return false
		})
	}
	if found == 0 {
		t.Fatalf(
			"production struct inventory found %d structs, want at least %d",
			found,
			1,
		)
	}
}

func productionStructRole(name string) (string, bool) {
	switch name {
	case "VendorSpec":
		return "published compiler-owned vendor contract", true
	case "ProviderVersion", "SignedURL", "SignedHeader", "SignedHeaders":
		return "opaque validated capability value", true
	case "UploadTarget", "DownloadTarget", "Integrity", "Policy":
		return "public protocol fact", true
	case "UploadRequest", "DownloadRequest":
		return "public execution ingress", true
	case "Client":
		return "capability wrapper", true
	case "GCSClient":
		return "authenticated provider capability wrapper", true
	case "GCSClientConfig":
		return "authenticated provider construction ingress", true
	case "GCSBucket", "GCSObjectName", "GCSObjectPrefix", "GCSCacheControl",
		"GCSGeneration":
		return "opaque validated provider value", true
	case "GCSWriteRequest", "GCSReadRequest", "GCSDeleteRequest", "GCSDeleteObjectRequest":
		return "authenticated provider execution ingress", true
	case "GCSObjectMetadata", "GCSDeleteResult", "GCSDeleteObjectResult":
		return "sealed authenticated provider evidence", true
	case "gcsObjectIdentity", "gcsObjectProperties", "gcsObjectTimes":
		return "internal authenticated provider metadata projection", true
	case "Transfer":
		return "sealed transfer evidence", true
	case "exactReader", "preparedUpload", "preparedDownload",
		"exchangeTarget", "streamDigests", "requestBody":
		return "internal streaming flow", true
	case "providerHeader":
		return "internal protocol field projection", true
	case "signedHeaderDeclaration", "sentHeaderNames":
		return "bounded internal protocol name set", true
	case "UploadCapability":
		return "received wire projection of a capability value", true
	case "UploadCapabilityProjection":
		return "external output projection of an already-issued capability value", true
	case "UploadCapabilityCommitment":
		return "sealed non-secret capability binding", true
	case "uploadCapabilityWire", "uploadCapabilityHeaderWire":
		return "internal exact wire temporary", true
	default:
		return "", false
	}
}
