// Package exchange adds typed, bounded operation policy to Go's real net/http
// client and server boundaries.
//
// Exchange owns HTTP transmission: request validation, total and per-attempt
// budgets, replay permission, retries, backoff, jitter, redirect confinement,
// bounded JSON and byte documents, and O(1)-memory streams. The caller owns the
// authority policy for projecting one canonical upstream client address from
// a real request; Exchange owns the bounded peer and forwarding mechanics. The
// caller owns the meaning and custody of every payload. In particular, object
// names, digests, cloud generation conditions, verification, and commitment
// belong to an object-store package, not Exchange.
//
// DNS, TLS, proxies, connections, framing, pooling, and scheduling remain with
// net/http and the operating system.
package exchange
