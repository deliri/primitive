// Package requestbudget consumes durable reservation credits through one
// bounded, synchronized local executor. Consumers own scope identities,
// authoritative windows, daily limits, batch sizes, route coverage and the
// transactional reservation effect. Primitive never infers them from HTTP.
//
// Each executor allocates its fixed capacity once. Live keys, including known
// exhaustion, are never evicted before their authoritative window ends. Full
// capacity is an error, not budget exhaustion, and never calls the provider.
// The executor reads no clock, starts no goroutine and performs no persistence.
// Process restarts can abandon previously reserved credits; durable counters
// remain authoritative and must never refund an ambiguous reservation.
package requestbudget
