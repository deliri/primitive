package cloudflare

import _ "embed"

//go:embed durable_wake.mjs
var durableWakeModule string

// DurableWakeModule emits a native Cloudflare Worker and SQLite Durable Object.
// The application configures the one HTTPS destination, scope header and secret
// bindings. The executor stores only a scope key and Cloudflare's own alarm;
// the destination remains authoritative for work and the next due instant.
func DurableWakeModule() string { return durableWakeModule }
