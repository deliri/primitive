package timematch

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

	"github.com/deliri/primitive/v2026/core"
)

//go:embed *.go browser_module.js
var timematchSource embed.FS

func productionFiles(t *testing.T) map[string]*ast.File {
	t.Helper()
	names, err := fs.Glob(timematchSource, "*.go")
	if err != nil {
		t.Fatalf("fs.Glob() error = %v, want nil", err)
	}
	files := map[string]*ast.File{}
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		source, readErr := timematchSource.ReadFile(name)
		file, parseErr := parser.ParseFile(token.NewFileSet(), name, source, 0)
		if readErr != nil || parseErr != nil {
			t.Fatalf("parse %s = (%v, %v), want nil", name, readErr, parseErr)
		}
		files[name] = file
	}
	return files
}

func TestTimeMatchProductionStructsHaveDataFlowRoles(t *testing.T) {
	t.Parallel()
	structs, classified := map[string]bool{}, map[string]bool{}
	for _, file := range productionFiles(t) {
		for _, declaration := range file.Decls {
			switch value := declaration.(type) {
			case *ast.GenDecl:
				for _, specification := range value.Specs {
					if typeSpec, ok := specification.(*ast.TypeSpec); ok {
						if _, ok := typeSpec.Type.(*ast.StructType); ok {
							structs[typeSpec.Name.Name] = true
						}
					}
				}
			case *ast.FuncDecl:
				name := value.Name.Name
				if value.Recv == nil || name != "timematchProtocolFact" && name != "timematchInternalFlow" && name != "timematchCapabilityWrapper" {
					continue
				}
				expression := value.Recv.List[0].Type
				if star, ok := expression.(*ast.StarExpr); ok {
					expression = star.X
				}
				if identifier, ok := expression.(*ast.Ident); ok {
					classified[identifier.Name] = true
				}
			}
		}
	}
	var missing []string
	for name := range structs {
		if !classified[name] {
			missing = append(missing, name)
		}
	}
	slices.Sort(missing)
	if len(missing) != 0 || len(structs) == 0 {
		t.Fatalf("unclassified production structs = %q of %d, want every struct classified", missing, len(structs))
	}
}

// All HTTP crosses Exchange: no Go production file may import a vendor SDK
// or construct or call a standard-library HTTP client.
func TestTimeMatchHTTPOnlyThroughExchange(t *testing.T) {
	t.Parallel()
	for name, file := range productionFiles(t) {
		for _, spec := range file.Imports {
			path, _ := strconv.Unquote(spec.Path.Value)
			if strings.HasPrefix(path, "golang.org/x/oauth2") || strings.HasPrefix(path, "google.golang.org/api") || strings.Contains(path, "msgraph") {
				t.Fatalf("%s imports vendor SDK %q, want Exchange-only HTTP", name, path)
			}
		}
		ast.Inspect(file, func(node ast.Node) bool {
			selector, ok := node.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if identifier, ok := selector.X.(*ast.Ident); ok && identifier.Name == "http" {
				switch selector.Sel.Name {
				case "Client", "DefaultClient", "Get", "Post", "Head", "PostForm", "NewRequest", "NewRequestWithContext", "DefaultTransport", "Transport":
					t.Fatalf("%s uses net/http.%s, want Exchange-only HTTP", name, selector.Sel.Name)
				}
			}
			return true
		})
	}
	module, err := timematchSource.ReadFile("browser_module.js")
	if err != nil {
		t.Fatalf("read browser module error = %v, want nil", err)
	}
	for _, forbidden := range []string{"fetch(", "XMLHttpRequest", "localStorage", "sessionStorage", "indexedDB", "document.cookie", "sendBeacon", "WebSocket", "import("} {
		if strings.Contains(string(module), forbidden) {
			t.Fatalf("browser module contains %q, want HTTP only through the injected Exchange executor and no storage", forbidden)
		}
	}
}

func TestBrowserModuleEmitsCoreIdentities(t *testing.T) {
	t.Parallel()
	module := BrowserModule()
	want := []string{"contract: " + strconv.Itoa(int(core.ErrTimeMatchContract)) + ",", "response: " + strconv.Itoa(int(core.ErrTimeMatchResponse)) + ","}
	for _, fragment := range want {
		if !strings.Contains(module, fragment) {
			t.Fatalf("BrowserModule() lacks %q", fragment)
		}
	}
	if strings.Contains(module, "__") || !strings.Contains(module, "export function createTimeMatchClient") {
		t.Fatalf("BrowserModule() has unreplaced placeholders or no client export")
	}
}
