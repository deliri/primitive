// Package gotoolchain performs bounded typed observations through cmd/go.
// Products declare the module, build-context, package-list, or compilation
// fact they need; this package owns command resolution, execution, bounds, and
// the conversion of cmd/go output into validated Primitive contracts.
// ParseFile also owns native Go source parsing and its file lifetime. Its
// whole-file AST retains Go's comments, object links and physical positions;
// compilation and build-file selection remain separate observations.
package gotoolchain
