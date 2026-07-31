package shutdown

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

func TestProductionArchitectureHasOneOwnedGoroutineAndExactImports(t *testing.T) {
	t.Parallel()

	files := shutdownProductionFiles(t)
	imports := make(map[string]struct{})
	var goroutines []string
	for name, file := range files {
		for _, imported := range file.Imports {
			path, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				t.Fatalf("strconv.Unquote(%s) error = %v", imported.Path.Value, err)
			}
			imports[path] = struct{}{}
		}
		ast.Inspect(file, func(node ast.Node) bool {
			if _, ok := node.(*ast.GoStmt); ok {
				goroutines = append(goroutines, name)
			}
			return true
		})
	}
	gotImports := make([]string, 0, len(imports))
	for path := range imports {
		gotImports = append(gotImports, path)
	}
	sort.Strings(gotImports)
	wantImports := []string{
		"context",
		"errors",
		"fmt",
		"github.com/deliri/primitive/v2026/contextstate",
		"github.com/deliri/primitive/v2026/core",
		"github.com/deliri/primitive/v2026/temporal",
		"os",
		"os/signal",
		"strconv",
		"strings",
		"sync",
		"syscall",
		"time",
		"unicode/utf8",
	}
	if !slices.Equal(gotImports, wantImports) {
		t.Fatalf("production imports = %v, want %v", gotImports, wantImports)
	}
	if len(goroutines) != 1 || goroutines[0] != "signal.go" {
		t.Fatalf("production goroutines = %v, want [signal.go]", goroutines)
	}
}

func TestProductionRejectsProcessExitAndWorldBuildingRatchet(t *testing.T) {
	t.Parallel()

	for name := range shutdownProductionFiles(t) {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("os.ReadFile(%q) error = %v", name, err)
		}
		source := string(data)
		for _, forbidden := range []string{
			"os.Exit(",
			"runtime.Goexit(",
			"exec.Command",
			"syscall.Kill(",
			"runtime.NumGoroutine",
			"encoding/json",
			"map[",
		} {
			if strings.Contains(source, forbidden) {
				t.Fatalf("%s contains %q, want no process policy or world model",
					name, forbidden)
			}
		}
	}
}

func shutdownProductionFiles(t *testing.T) map[string]*ast.File {
	t.Helper()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("os.ReadDir(.) error = %v", err)
	}
	files := make(map[string]*ast.File)
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
		files[name] = file
	}
	return files
}
