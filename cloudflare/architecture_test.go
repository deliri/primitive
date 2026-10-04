package cloudflare

import (
	"embed"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"slices"
	"strconv"
	"strings"
	"testing"
)

// Cloudflare owns provider policy and wire facts. The effect packages alone
// own Go's HTTP, clock, filesystem and process handles. Scanning imports also
// prevents aliases, dot imports and function-value indirection hiding a bypass.
func TestCloudflareEffectsRemainBehindPrimitiveCapabilities(t *testing.T) {
	t.Parallel()
	files, err := fs.Glob(cloudflareSource, "*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		data, err := cloudflareSource.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		file, err := parser.ParseFile(token.NewFileSet(), name, data, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		if got := cloudflareForbiddenImports(file); len(got) != 0 {
			t.Errorf("%s direct effect/provider imports = %q, want none", name, got)
		}
	}
}

func cloudflareForbiddenImports(file *ast.File) []string {
	var forbidden []string
	for _, specification := range file.Imports {
		name, err := strconv.Unquote(specification.Path.Value)
		if err != nil {
			forbidden = append(forbidden, specification.Path.Value)
			continue
		}
		switch name {
		case "os", "os/exec", "syscall", "net", "net/http", "time", "crypto/rand":
			forbidden = append(forbidden, name)
		default:
			if strings.HasPrefix(name, "github.com/aws/") || strings.HasPrefix(name, "cloud.google.com/") {
				forbidden = append(forbidden, name)
			}
		}
	}
	return forbidden
}

func TestCloudflareEffectMatcherLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, source string
		want         []string
	}{
		{name: "typed Exchange dependency preserves HTTP ownership", source: `import "github.com/deliri/primitive/v2026/exchange"`},
		{name: "aliased HTTP import cannot bypass Exchange", source: `import hidden "net/http"`, want: []string{"net/http"}},
		{name: "dot clock import cannot bypass Temporal", source: `import . "time"`, want: []string{"time"}},
		{name: "filesystem import cannot bypass Filestore", source: `import "os"`, want: []string{"os"}},
		{name: "foreign provider SDK cannot introduce hidden transport", source: `import "github.com/aws/aws-sdk-go-v2/service/s3"`, want: []string{"github.com/aws/aws-sdk-go-v2/service/s3"}},
		{name: "pure URL representation has no effects", source: `import "net/url"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			file, err := parser.ParseFile(token.NewFileSet(), "fixture.go", "package fixture\n"+tc.source, parser.ImportsOnly)
			if err != nil {
				t.Fatal(err)
			}
			if got := cloudflareForbiddenImports(file); !slices.Equal(got, tc.want) {
				t.Fatalf("forbidden imports=%q, want %q", got, tc.want)
			}
		})
	}
}

//go:embed *.go
var cloudflareSource embed.FS

func TestCloudflareProductionStructsHaveCompilerVisibleDataFlowRoles(t *testing.T) {
	t.Parallel()

	files, err := fs.Glob(cloudflareSource, "*.go")
	if err != nil {
		t.Fatalf("fs.Glob(cloudflare production source) error = %v, want nil", err)
	}
	structs := make(map[string]struct{})
	classified := make(map[string]struct{})
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		source, readErr := cloudflareSource.ReadFile(name)
		file, parseErr := parser.ParseFile(token.NewFileSet(), name, source, 0)
		if readErr != nil || parseErr != nil {
			t.Fatalf("parse embedded cloudflare source %s errors = (%v, %v), want nil", name, readErr, parseErr)
		}
		collectCloudflareStructRoles(file, structs, classified)
	}
	missing := missingCloudflareStructRoles(structs, classified)
	if len(missing) != 0 {
		t.Fatalf("cloudflare production structs missing data-flow role = %q, want every protocol, flow, or capability struct classified", missing)
	}
}

func collectCloudflareStructRoles(file *ast.File, structs, classified map[string]struct{}) {
	for _, declaration := range file.Decls {
		switch value := declaration.(type) {
		case *ast.GenDecl:
			for _, specification := range value.Specs {
				typeSpec, ok := specification.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if _, ok := typeSpec.Type.(*ast.StructType); ok {
					structs[typeSpec.Name.Name] = struct{}{}
				}
			}
		case *ast.FuncDecl:
			if value.Recv == nil || !cloudflareRoleMethod(value.Name.Name) {
				continue
			}
			if receiver := cloudflareReceiverName(value.Recv.List[0].Type); receiver != "" {
				classified[receiver] = struct{}{}
			}
		}
	}
}

func missingCloudflareStructRoles(structs, classified map[string]struct{}) []string {
	missing := make([]string, 0)
	for name := range structs {
		if _, ok := classified[name]; !ok {
			missing = append(missing, name)
		}
	}
	slices.Sort(missing)
	return missing
}

func cloudflareRoleMethod(name string) bool {
	return name == "cloudflareProtocolFact" || name == "cloudflareInternalFlow" || name == "cloudflareCapabilityWrapper"
}

func cloudflareReceiverName(expression ast.Expr) string {
	if pointer, ok := expression.(*ast.StarExpr); ok {
		expression = pointer.X
	}
	if generic, ok := expression.(*ast.IndexExpr); ok {
		expression = generic.X
	}
	if identifier, ok := expression.(*ast.Ident); ok {
		return identifier.Name
	}
	return ""
}
