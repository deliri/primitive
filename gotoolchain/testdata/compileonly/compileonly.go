// Package compileonly is a production-path fixture whose test binary must
// compile but must never be started by gotoolchain.CompilePackage.
package compileonly

// Value keeps the production package nonempty.
const Value = 1
