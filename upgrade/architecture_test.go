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
		"encoding/json",
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
func TestProductionCarriesNoGoroutineWorldModelOrProcessExit(t *testing.T) {
	t.Parallel()

	for _, source := range upgradeProductionFiles(t) {
		ast.Inspect(source.syntax, func(node ast.Node) bool {
			switch typed := node.(type) {
			case *ast.GoStmt:
				t.Errorf("%s starts a goroutine at %d, want one caller-owned path",
					source.name, source.set.Position(typed.Pos()).Line)
			case *ast.MapType:
				t.Errorf("%s declares a map at %d, want typed structs and closed enums",
					source.name, source.set.Position(typed.Pos()).Line)
			case *ast.ChanType:
				t.Errorf("%s declares a channel at %d, want no private scheduler",
					source.name, source.set.Position(typed.Pos()).Line)
			case *ast.SelectStmt:
				t.Errorf("%s selects at %d, want no private scheduler",
					source.name, source.set.Position(typed.Pos()).Line)
			case *ast.CallExpr:
				selector, ok := typed.Fun.(*ast.SelectorExpr)
				if !ok {
					return true
				}
				pkg, ok := selector.X.(*ast.Ident)
				if ok && pkg.Name == "os" && selector.Sel.Name == "Exit" {
					t.Errorf("%s calls os.Exit at %d, want a returned typed failure",
						source.name, source.set.Position(typed.Pos()).Line)
				}
			}
			return true
		})
	}
}

// TestTheForbiddenShapeMatcherActuallyMatches proves the scan above is not
// vacuously green. It runs the same matcher over synthetic source that contains
// every forbidden shape.
func TestTheForbiddenShapeMatcherActuallyMatches(t *testing.T) {
	t.Parallel()

	const hostile = `package upgrade

import "os"

type worldModel map[string]int

func offend(events chan int) {
	go offend(events)
	select {
	case <-events:
	}
	os.Exit(1)
}
`
	set := token.NewFileSet()
	file, err := parser.ParseFile(set, "hostile.go", hostile, 0)
	if err != nil {
		t.Fatalf("parser.ParseFile(hostile) error = %v", err)
	}
	var goStatements, maps, channels, selects, exits int
	ast.Inspect(file, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.GoStmt:
			goStatements++
		case *ast.MapType:
			maps++
		case *ast.ChanType:
			channels++
		case *ast.SelectStmt:
			selects++
		case *ast.CallExpr:
			selector, ok := typed.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			pkg, ok := selector.X.(*ast.Ident)
			if ok && pkg.Name == "os" && selector.Sel.Name == "Exit" {
				exits++
			}
		}
		return true
	})
	if goStatements != 1 || maps != 1 || channels != 1 || selects != 1 || exits != 1 {
		t.Fatalf("hostile matches = go:%d map:%d chan:%d select:%d exit:%d, "+
			"want 1/1/1/1/1", goStatements, maps, channels, selects, exits)
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
		"selectionDocument":   "durable primary fact",
		"selectionWire":       "canonical persistence wire",
		"stageAuthorityFacts": "authenticated release-to-selector projection",
		"trialDocument":       "durable trial ownership fact",
		"trialWire":           "canonical persistence wire",
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
