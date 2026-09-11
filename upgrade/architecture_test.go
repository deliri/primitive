package upgrade

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
)

func TestProductionImportsAreExactAndNoWorldModelExists(t *testing.T) {
	t.Parallel()

	var imports []string
	for _, source := range upgradeProductionFiles(t) {
		for _, imported := range source.syntax.Imports {
			path, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				t.Fatalf("strconv.Unquote(%s) error = %v", imported.Path.Value, err)
			}
			if !slices.Contains(imports, path) {
				imports = append(imports, path)
			}
		}
	}
	sort.Strings(imports)
	want := []string{
		"bytes",
		"context",
		"encoding/json/v2",
		"errors",
		"fmt",
		"github.com/deliri/primitive/v2026/core",
		"github.com/deliri/primitive/v2026/filestore",
		"github.com/deliri/primitive/v2026/hostfacts",
		"github.com/deliri/primitive/v2026/objectstore",
		"github.com/deliri/primitive/v2026/release",
		"github.com/deliri/primitive/v2026/temporal",
		"hash/crc32",
		"io",
		"io/fs",
		"math/bits",
		"os",
	}
	if !slices.Equal(imports, want) {
		t.Fatalf("production imports = %v, want %v", imports, want)
	}
}

// TestProductionCarriesNoGoroutineWorldModelOrProcessExit matches syntax rather
// than prose. A substring scan over the raw file cannot tell a comment from
// code, misses "go\tf()", and reports a false violation for the word "go" in a
// sentence. Every forbidden package (os/exec, runtime, sync, syscall, time) is
// already refused by the exact production import set above, so the shapes that
// remain unprovable from the import list are the ones matched here.
func forbiddenUpgradeShape(node ast.Node) string {
	switch typed := node.(type) {
	case *ast.GoStmt:
		return "goroutine"
	case *ast.MapType:
		return "map"
	case *ast.ChanType:
		return "channel"
	case *ast.SelectStmt:
		return "select"
	case *ast.CallExpr:
		selector, ok := typed.Fun.(*ast.SelectorExpr)
		if !ok {
			return ""
		}
		pkg, ok := selector.X.(*ast.Ident)
		if ok && pkg.Name == "os" && selector.Sel.Name == "Exit" {
			return "process exit"
		}
	}
	return ""
}

func TestProductionCarriesNoGoroutineWorldModelOrProcessExit(t *testing.T) {
	t.Parallel()
	for _, source := range upgradeProductionFiles(t) {
		ast.Inspect(source.syntax, func(node ast.Node) bool {
			if kind := forbiddenUpgradeShape(node); kind != "" {
				t.Errorf("%s:%d contains %s, want caller-owned synchronous typed mechanics", source.name, source.set.Position(node.Pos()).Line, kind)
			}
			return true
		})
	}
}

// The live repository scan and synthetic fixtures use exactly one matcher.
func TestTheForbiddenShapeMatcherActuallyMatches(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, source string
		want         []string
	}{
		{name: "each forbidden syntax has one match", source: `package upgrade
import "os"
type worldModel map[string]int
func offend(events chan int) { go offend(events); select { case <-events: }; os.Exit(1) }
`, want: []string{"channel", "goroutine", "map", "process exit", "select"}},
		{name: "synchronous struct and return remain admitted", source: `package upgrade
type fact struct { value int }
func value(v fact) int { return v.value }
`},
		{name: "comments and strings do not become executable shapes", source: `package upgrade
// go worker(); os.Exit(1); map[string]int
const diagnostic = "select channel"
`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			file, err := parser.ParseFile(token.NewFileSet(), "fixture.go", tc.source, 0)
			if err != nil {
				t.Fatalf("ParseFile error = %v, want nil", err)
			}
			var got []string
			ast.Inspect(file, func(node ast.Node) bool {
				if kind := forbiddenUpgradeShape(node); kind != "" {
					got = append(got, kind)
				}
				return true
			})
			sort.Strings(got)
			if !slices.Equal(got, tc.want) {
				t.Fatalf("forbidden syntax = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestEverySettledRemovalCarriesTheRecoveryContext locks the compiler-visible
// half of a contract no in-package behavioural test can reach: a removal that
// settles an effect the same call already committed must survive the caller's
// cancellation, or a cancelled Promote strands the former slot and wedges the
// next candidate. The exception is named, not silent.
func TestEverySettledRemovalCarriesTheRecoveryContext(t *testing.T) {
	t.Parallel()

	// A removal is unsettled only when the caller still owns the bytes and a
	// cancelled caller must therefore be obeyed.
	unsettled := map[string]string{
		"DiscardTrial": "removes a candidate the caller still owns, so cancellation must stop it",
	}
	const wantSettled = 5

	settled := 0
	for _, source := range upgradeProductionFiles(t) {
		var enclosing string
		ast.Inspect(source.syntax, func(node ast.Node) bool {
			if declaration, ok := node.(*ast.FuncDecl); ok {
				enclosing = declaration.Name.Name
				return true
			}
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}
			name, ok := call.Fun.(*ast.Ident)
			if !ok || name.Name != "removeArtifact" || len(call.Args) == 0 {
				return true
			}
			line := source.set.Position(call.Pos()).Line
			if reason, exempt := unsettled[enclosing]; exempt {
				t.Logf("%s:%d %s removal is unsettled: %s",
					source.name, line, enclosing, reason)
				return true
			}
			wrapper, ok := call.Args[0].(*ast.CallExpr)
			if !ok {
				t.Errorf("%s:%d removeArtifact in %s passes %T, want recoveryContext(...)",
					source.name, line, enclosing, call.Args[0])
				return true
			}
			wrapperName, ok := wrapper.Fun.(*ast.Ident)
			if !ok || wrapperName.Name != "recoveryContext" {
				t.Errorf("%s:%d removeArtifact in %s does not settle its context",
					source.name, line, enclosing)
				return true
			}
			settled++
			return true
		})
	}
	if settled != wantSettled {
		t.Fatalf("settled removals = %d, want %d; update the count with the reason "+
			"when a settled removal is added or retired", settled, wantSettled)
	}
}

func TestEveryProductionStructHasADataFlowRole(t *testing.T) {
	t.Parallel()

	roles := map[string]string{
		"AttemptError":        "typed externally returned failure record",
		"BootstrapRequest":    "typed public ingress",
		"DiscardTrialRequest": "typed public ingress",
		"DownloadSource":      "provider capability wrapper",
		"Primary":             "validated sealed projection",
		"PromoteRequest":      "typed public ingress",
		"Promotion":           "validated promotion authority",
		"ResolveRequest":      "typed public ingress",
		"StagePolicy":         "typed caller policy",
		"StageRequest":        "typed public ingress",
		"TrialReport":         "typed product observation",
		"TrialTarget":         "validated trial capability",
		"bootstrapWrite":      "internal ownership receipt",
		"candidateDownload":   "internal ownership receipt",
		"metadataBuffer":      "fixed storage for one bounded canonical persistence document",
		"selectionDocument":   "durable primary fact",
		"selectionWire":       "canonical persistence wire",
		"stageAuthorityFacts": "authenticated release-to-selector projection",
		"trialDocument":       "durable trial ownership fact",
		"trialWire":           "canonical persistence wire",
		"attemptErrorRequest": "internal failure projection request",
	}
	seen := make(map[string]bool, len(roles))
	for _, source := range upgradeProductionFiles(t) {
		for _, declaration := range source.syntax.Decls {
			general, ok := declaration.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, specification := range general.Specs {
				named, ok := specification.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if _, ok := named.Type.(*ast.StructType); !ok {
					continue
				}
				role, classified := roles[named.Name.Name]
				if !classified || role == "" {
					t.Errorf("%s declares unclassified production struct %s",
						source.name, named.Name.Name)
					continue
				}
				seen[named.Name.Name] = true
			}
		}
	}
	for name, role := range roles {
		if !seen[name] {
			t.Errorf("data-flow inventory classifies absent struct %s as %q",
				name, role)
		}
	}
}

type upgradeProductionFile struct {
	set    *token.FileSet
	syntax *ast.File
	name   string
}

func upgradeProductionFiles(t *testing.T) []upgradeProductionFile {
	t.Helper()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("os.ReadDir(.) error = %v", err)
	}
	files := make([]upgradeProductionFile, 0, len(entries))
	fileSet := token.NewFileSet()
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") ||
			strings.HasSuffix(name, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fileSet, name, nil, 0)
		if err != nil {
			t.Fatalf("parser.ParseFile(%q) error = %v", name, err)
		}
		files = append(files, upgradeProductionFile{
			name: name, syntax: file, set: fileSet,
		})
	}
	return files
}
