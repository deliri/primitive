// Package webmcp publishes Primitive's browser-native WebMCP executor.
// Applications retain tool metadata, schemas, input/result admission and actions.
package webmcp

import _ "embed"

//go:embed browser_module.js
var browserModule string

// BrowserModule returns the immutable browser module. The native document owns
// discovery, registration, serialization and invocation. This executor carries
// registration lifetime into each invocation; it does not provide authentication.
func BrowserModule() string { return browserModule }
