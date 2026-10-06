// Package jsonio consumes sequential JSON documents or lexical tokens through
// typed requests, Go-owned parsing and synchronous backpressure. Document
// admission declares explicit extents; token streams avoid document and array
// aggregation and keep scalar-token and parser-state memory visible.
// The caller owns source lifetime, application policy and overall work budget.
package jsonio
