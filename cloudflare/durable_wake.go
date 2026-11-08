package cloudflare

import _ "embed"

//go:embed durable_wake.mjs
var durableWakeModule string

// DurableWakeGuardHeader carries a positive relative guard duration as exact
// decimal nanoseconds. A guarded reservation requires HTTP 201; ordinary wake
// acknowledgment is HTTP 204. Callers must cover their full writer lifetime.
const DurableWakeGuardHeader = "X-Wake-Guard-Nanoseconds"

// DurableWakeModule emits a native Cloudflare Worker and SQLite Durable Object.
// The application configures the one HTTPS destination, scope header and secret
// bindings. The executor stores only a scope key, an optional writer guard
// deadline, and Cloudflare's own alarm;
// the destination remains authoritative for work and the next due instant.
func DurableWakeModule() string { return durableWakeModule }
