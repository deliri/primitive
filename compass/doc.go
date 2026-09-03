// Package compass owns the reusable, compiler-visible boundary between one
// project's human-authored config/config.json document and Go code.
//
// Each project owns its complete local configuration struct and validation.
// Compass owns the common project declaration and the bounded strict decoder.
// Consumers receive validated structs; they do not decode JSON themselves.
package compass
