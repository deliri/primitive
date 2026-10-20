package gotoolchain_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/capabilities"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/gomodule"
	"github.com/deliri/primitive/v2026/gotoolchain"
)

const nativeSymbolFixture = `package sample
import platform "os"
type alias = platform.File
type promoted struct { *platform.File }
type local struct{}
func (local) Write([]byte) (int, error) { return 0, nil }
//line invented.go:900
func use(file *platform.File, a *alias, p promoted, own local) {
 file.Write(nil)
 a.Sync()
 p.Close()
 read := file.Read
 read(nil)
 _ = (*platform.File).Seek
 own.Write(nil)
 platform.Open("unused")
}
func opaque(reader interface { Read([]byte) (int, error) }) { reader.Read(nil) }
`

func TestSymbolReferencesUseNativeDeclaringIdentityAndPhysicalSource(t *testing.T) {
	t.Parallel()
	analysis := nativeSymbolAnalysis(t, t.TempDir())
	want := []struct{ owner, receiver, selector string }{
		{"os", "File", "Write"},
		{"os", "File", "Sync"},
		{"os", "File", "Close"},
		{"os", "File", "Read"},
		{"os", "File", "Seek"},
		{"example.com/native-symbols", "local", "Write"},
		{"os", "", "Open"},
		{"example.com/native-symbols", "", "Read"},
	}
	count := 0
	for reference, err := range analysis.SymbolReferences(t.Context()) {
		if err != nil {
			t.Fatalf("native reference %d error = %v, want nil", count, err)
		}
		if count >= len(want) {
			t.Fatalf("native references exceed %d, got %+v", len(want), reference)
		}
		receiver := ""
		if reference.Symbol.Receiver != nil {
			receiver = reference.Symbol.Receiver.String()
		}
		if reference.Symbol.ImportPath.String() != want[count].owner || receiver != want[count].receiver || reference.Symbol.Selector.String() != want[count].selector {
			t.Fatalf("reference %d = %+v receiver %q, want %+v", count, reference, receiver, want[count])
		}
		if reference.Line >= 900 || filepath.Base(reference.File.String()) != "source.go" || reference.Offset >= uint64(len(nativeSymbolFixture)) {
			t.Fatalf("physical reference coordinates = %+v, want real source.go position independent of line directive", reference)
		}
		name := reference.Symbol.Selector.String()
		if !strings.HasPrefix(nativeSymbolFixture[reference.Offset:], name) {
			t.Fatalf("reference at byte %d = %q, want %q", reference.Offset, nativeSymbolFixture[reference.Offset:], name)
		}
		if count == 7 && reference.Kind != gotoolchain.SymbolReferenceAnonymousMethod {
			t.Fatalf("opaque interface reference kind = %v, want anonymous method", reference.Kind)
		}
		fact, err := capabilities.ResolveStandardSymbol(reference.Symbol)
		wantEffect := want[count].owner == "os"
		if err != nil || (fact.Disposition == capabilities.StandardSymbolEffect) != wantEffect {
			t.Fatalf("catalog ownership for reference %d = %+v/%v, want effect %t", count, fact, err, wantEffect)
		}
		count++
	}
	if count != len(want) {
		t.Fatalf("native reference count = %d, want %d", count, len(want))
	}
}

func TestSymbolReferenceStreamRefusesCancellationAndIncompleteAnalysis(t *testing.T) {
	t.Parallel()
	analysis := nativeSymbolAnalysis(t, t.TempDir())
	for _, tc := range []struct {
		name       string
		cancelled  bool
		incomplete bool
		want       error
	}{
		{name: "cancelled owner publishes no reference", cancelled: true, want: context.Canceled},
		{name: "partial compiler analysis publishes no reference", incomplete: true, want: core.ErrGoToolchainOutput},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			candidate := analysis
			candidate.Incomplete = tc.incomplete
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if tc.cancelled {
				cancel()
			}
			count := 0
			for reference, err := range candidate.SymbolReferences(ctx) {
				count++
				if !errors.Is(err, tc.want) || reference.File != (core.AbsolutePath{}) || reference.Line != 0 || reference.Symbol.Receiver != nil {
					t.Fatalf("refused reference = %+v/%v, want zero reference and %v", reference, err, tc.want)
				}
			}
			if count != 1 {
				t.Fatalf("refusal events = %d, want exactly one", count)
			}
		})
	}
	count := 0
	for _, err := range analysis.SymbolReferences(t.Context()) {
		if err != nil {
			t.Fatal(err)
		}
		count++
		break
	}
	if count != 1 {
		t.Fatalf("early consumer deliveries = %d, want one", count)
	}
	count = 0
	for _, err := range analysis.SymbolReferences(t.Context()) {
		if err != nil {
			t.Fatal(err)
		}
		count++
	}
	if count != 8 {
		t.Fatalf("replayed native references after early stop = %d, want all eight", count)
	}
}

func nativeSymbolAnalysis(t *testing.T, directory string) gotoolchain.PackageAnalysis {
	t.Helper()
	return nativeSymbolSourceAnalysis(t, directory, nativeSymbolFixture)
}

func nativeSymbolSourceAnalysis(t *testing.T, directory, source string) gotoolchain.PackageAnalysis {
	t.Helper()
	for _, file := range []struct{ name, body string }{{"go.mod", "module example.com/native-symbols\n\ngo 1.27.2\n"}, {"source.go", source}} {
		if err := os.WriteFile(filepath.Join(directory, file.name), []byte(file.body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	limits, err := gotoolchain.DefaultLimits()
	if err != nil {
		t.Fatal(err)
	}
	capability, err := gotoolchain.Open(t.Context(), gotoolchain.Configuration{Workspace: gotoolchain.WorkspaceModeDisabled, Limits: limits})
	if err != nil {
		t.Fatal(err)
	}
	path, err := core.ParseAbsolutePath(directory)
	if err != nil {
		t.Fatal(err)
	}
	imported, err := gomodule.ParseImportPath("example.com/native-symbols")
	if err != nil {
		t.Fatal(err)
	}
	analysis, err := capability.AnalyzePackage(t.Context(), gotoolchain.AnalysisRequest{WorkingDirectory: path, Package: imported})
	if err != nil {
		t.Fatal(err)
	}
	return analysis
}
