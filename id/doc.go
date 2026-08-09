// Package id owns the canonical time-ordered identifiers both ends of the
// stack persist and exchange: UUIDv7 and ULID. Construction is pure. The
// instant arrives as one temporal observation and the entropy as caller-drawn
// secret material, so the clock and entropy effects stay with the doors that
// own them and no identity in the fleet asks a third-party library for
// either.
package id
