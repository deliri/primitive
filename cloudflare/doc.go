// Package cloudflare owns Cloudflare Images, Stream, and R2 provider mechanics.
// Server capabilities hold credentials; client capabilities consume narrowly
// issued upload or object authorities. Exchange owns HTTP, Temporal owns time,
// and callers own policy, retries, idempotency, and resource lifetimes.
//
// Media transfers stream with bounded working memory. Authentication, upload
// acceptance, processing readiness, and durable application accounting are
// separate facts; an upload URL is never evidence that media was uploaded.
package cloudflare
