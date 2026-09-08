package gotoolchain

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"testing"
)

// These cases exercise the actual Go checker on authored syntax. No C types
// are invented: a reference requiring absent generated definitions must still
// fail. Hammer's integration tests separately drive real cmd/cgo definitions.
func TestConfigureCgoPreservesGoChecking(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, source                                         string
		enable, repeat, fake, ignoreBodies, nilConfiguration bool
		wantError                                            bool
	}{
		{name: "nil checker configuration is refused", nilConfiguration: true},
		{name: "ordinary Go retains its constant", source: "package p; const Value = 7", enable: true},
		{name: "ordinary Go needs no cgo configuration", source: "package p; const Value = 7"},
		{name: "authored foreign import uses the real checker mode", source: "package p; import \"C\"; const Value = 7", enable: true},
		{name: "missing mode cannot pretend the foreign import resolves", source: "package p; import \"C\"; const Value = 7", wantError: true},
		{name: "repeated enabling is idempotent", source: "package p; import \"C\"; const Value = 7", enable: true, repeat: true},
		{name: "missing generated type remains an error", source: "package p; import \"C\"; var Value C.missing", enable: true, wantError: true},
		{name: "missing generated function remains an error", source: "package p; import \"C\"; func Value() { C.missing() }", enable: true, wantError: true},
		{name: "ordinary undefined symbol remains an error", source: "package p; import \"C\"; var Value = missing", enable: true, wantError: true},
		{name: "ordinary assignment mismatch remains an error", source: "package p; import \"C\"; var Value int = true", enable: true, wantError: true},
		{name: "fake and real modes cannot silently combine", source: "package p; import \"C\"; const Value = 7", enable: true, fake: true, wantError: true},
		{name: "caller body policy survives configuration", source: "package p; import \"C\"; func Value() { missing() }", enable: true, ignoreBodies: true},
		{name: "body errors remain visible when checking is requested", source: "package p; import \"C\"; func Value() { missing() }", enable: true, wantError: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.nilConfiguration {
				if got := configureCgo(nil); got {
					t.Fatalf("configureCgo(nil) = %t, want false", got)
				}
				return
			}
			positions := token.NewFileSet()
			file, err := parser.ParseFile(positions, "source.go", tc.source, 0)
			if err != nil {
				t.Fatal(err)
			}
			var diagnostics []error
			configuration := types.Config{
				Context: types.NewContext(), GoVersion: "go1.27", Sizes: types.SizesFor("gc", "arm64"),
				FakeImportC: tc.fake, IgnoreFuncBodies: tc.ignoreBodies,
				Error: func(err error) { diagnostics = append(diagnostics, err) },
			}
			beforeContext, beforeSizes := configuration.Context, configuration.Sizes
			if tc.enable && !configureCgo(&configuration) {
				t.Fatalf("configureCgo() = false, want true for Go version %q", configuration.GoVersion)
			}
			if tc.repeat && !configureCgo(&configuration) {
				t.Fatalf("repeated configureCgo() = false, want true for Go version %q", configuration.GoVersion)
			}
			if configuration.Context != beforeContext || configuration.Sizes != beforeSizes || configuration.GoVersion != "go1.27" || configuration.FakeImportC != tc.fake || configuration.IgnoreFuncBodies != tc.ignoreBodies {
				t.Fatalf("checker options changed: GoVersion=%q FakeImportC=%t IgnoreFuncBodies=%t; want caller options preserved", configuration.GoVersion, configuration.FakeImportC, configuration.IgnoreFuncBodies)
			}
			information := types.Info{Defs: make(map[*ast.Ident]types.Object)}
			pkg, err := configuration.Check("example.com/p", positions, []*ast.File{file}, &information)
			if (err != nil) != tc.wantError || (len(diagnostics) != 0) != tc.wantError {
				t.Fatalf("Go checker error=%v, diagnostics=%d, want error=%t", err, len(diagnostics), tc.wantError)
			}
			if tc.wantError {
				var typed types.Error
				if !errors.As(err, &typed) || typed.Fset != positions {
					t.Fatalf("Go checker error lost its type or source coordinates: %v", err)
				}
				return
			}
			if pkg == nil || !pkg.Complete() || pkg.Scope().Lookup("Value") == nil {
				t.Fatalf("checked package = %v, want complete package retaining Value", pkg)
			}
		})
	}
}
