// Package gotoolchain performs typed native observations through cmd/go.
// Products declare the module, module-file, build-context, package-list, or
// compilation fact they need; this package owns command resolution, execution,
// the conversion of cmd/go output into validated Primitive contracts.
// ParseFile also owns native Go source parsing and its file lifetime. Its
// whole-file AST retains Go's comments, object links and physical positions;
// compilation and build-file selection remain separate observations.
// WithSSAUnit can opt into Go-owned source/value references without a product
// symbol table. Source metadata and its additional allocation stay opt-in.
// ObserveModuleFile delegates go.mod grammar to Go's read-only mod edit -json
// command. It retains only a declared-version and replacement-count projection;
// native Go owns its file parser, and Go's JSON cursor owns scalar buffers.
package gotoolchain
