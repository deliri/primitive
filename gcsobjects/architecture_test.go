package gcsobjects

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"testing"
)

// TestAuthenticatedGCSOperationsAreCompilerSelectedEntryPoints pins the exact
// exported operation set. UploadMedia and UploadFile are two compiler-selected
// entry points, not one call behind a served-or-stored mode flag: the caller
// makes the served-versus-stored decision by naming the function, so a single
// Upload with a boolean would be exactly the coupling this ratchet forbids.
func TestAuthenticatedGCSOperationsAreCompilerSelectedEntryPoints(t *testing.T) {
	t.Parallel()

	set := token.NewFileSet()
	var got []string
	for _, path := range []string{"client.go", "upload_capability.go"} {
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
		"CreateBucket",
		"DeleteGCSObject",
		"DeleteGCSObjects",
		"IssueGCSUploadCapability",
		"NewGCSClient",
		"NewGCSUploadCapabilityIssuer",
		"ObserveGCSUpload",
		"ParseGCSServiceAccount",
		"ReadGCSObject",
		"UploadFile",
		"UploadMedia",
	}
	if !slices.Equal(got, want) {
		t.Fatalf(
			"exported gcsobjects operations = %q, want exactly %q with served and stored as distinct entry points",
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
	case "GCSClient", "GCSUploadCapabilityIssuer":
		return "authenticated provider capability wrapper", true
	case "GCSClientConfig":
		return "authenticated provider construction ingress", true
	case "GCSBucket", "GCSObjectName", "GCSObjectPrefix", "GCSCacheControl",
		"GCSGeneration", "GCSProjectID", "GCSLocation", "GCSObjectSegment", "GCSServiceAccount":
		return "opaque validated provider value", true
	case "GCSMediaUpload", "GCSFileUpload", "GCSReadRequest", "GCSUploadObservationRequest", "GCSDeleteRequest",
		"GCSDeleteObjectRequest", "GCSBucketCreateRequest", "GCSRootPrefixRequest",
		"GCSChildPrefixRequest", "GCSObjectInPrefixRequest", "GCSUploadCapabilityRequest":
		return "authenticated provider execution ingress", true
	case "gcsWrite":
		return "internal owner-only write projection", true
	case "gcsUploadURLRequest":
		return "internal official SDK signing projection", true
	case "GCSObjectMetadata", "VerifiedGCSUpload", "GCSDeleteResult", "GCSDeleteObjectResult", "GCSBucketProvisioning":
		return "sealed authenticated provider evidence", true
	case "gcsObjectIdentity", "gcsObjectProperties", "gcsObjectTimes":
		return "internal authenticated provider metadata projection", true
	default:
		return "", false
	}
}
