package objectstore

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
)

func TestObjectstoreHTTPDoorArchitectureLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive real production delegates every HTTP execution door", func(t *testing.T) {
		t.Parallel()

		files, err := filepath.Glob("*.go")
		if err != nil {
			t.Fatalf("filepath.Glob() error = %v, want nil", err)
		}
		var got []string
		for _, path := range files {
			if strings.HasSuffix(path, "_test.go") {
				continue
			}
			imports, err := objectstoreHTTPImports(path, nil)
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
			got, err := objectstoreHTTPImports("synthetic.go", tc.code)
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
			got, err := objectstoreHTTPImports("synthetic.go", code)
			if err != nil || len(got) != 0 {
				t.Fatalf("objectstoreHTTPImports(neutral) = (%q, %v), want (nil, nil)", got, err)
			}
		}
	})
}

func objectstoreHTTPImports(filename string, source any) ([]string, error) {
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
		file, gotParseErr := parser.ParseFile(set, path, nil, 0)
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
	case "ProviderVersion", "SignedURL", "SignedHeader", "SignedHeaders", "BLAKE3Digest":
		return "opaque validated capability value", true
	case "UploadTarget", "DownloadTarget", "Integrity", "Policy", "TransferProgress":
		return "public protocol fact", true
	case "UploadRequest", "DownloadRequest", "UploadCapabilityRequest", "DownloadCapabilityRequest", "InspectionRequest":
		return "public execution ingress", true
	case "Inspection":
		return "sealed stream integrity", true
	case "Client":
		return "capability wrapper", true
	case "Transfer":
		return "sealed transfer evidence", true
	case "TransferEvidence":
		return "received wire projection of confirmed transfer evidence", true
	case "TransferEvidenceProjection":
		return "issue-only projection of confirmed transfer evidence", true
	case "ProviderUploadObservationRequest":
		return "provider-neutral exact-upload observation ingress", true
	case "VerifiedProviderUpload":
		return "sealed provider-neutral exact-upload evidence", true
	case "ExactReader", "preparedUpload", "preparedDownload", "progressWriter",
		"exchangeTarget", "streamDigests", "requestBody", "uploadObservedSource", "framedUploadSource", "inspectionCopier", "uploadConfirmation",
		"transferConfirmation", "callerSignedHeaderValidation":
		return "internal streaming flow", true
	case "providerHeader":
		return "internal protocol field projection", true
	case "signedHeaderDeclaration", "sentHeaderNames":
		return "bounded internal protocol name set", true
	case "UploadCapability":
		return "received wire projection of a capability value", true
	case "UploadCapabilityProjection":
		return "external output projection of an already-issued capability value", true
	case "UploadHTTPProjection":
		return "external output projection of one browser-spendable upload request", true
	case "UploadCapabilityCommitment":
		return "sealed non-secret capability binding", true
	case "DownloadCapability":
		return "received wire projection of a retrieval capability value", true
	case "DownloadCapabilityProjection":
		return "external output projection of an already-issued retrieval capability value", true
	case "DownloadCapabilityCommitment":
		return "sealed non-secret retrieval capability binding", true
	case "uploadCapabilityWire", "uploadCapabilityHeaderWire", "uploadHTTPProjectionWire":
		return "internal exact wire temporary", true
	case "transferEvidenceWire":
		return "internal exact wire temporary", true
	default:
		return "", false
	}
}
