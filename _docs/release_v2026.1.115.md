# Primitive v2026.1.115 — native calendar text and append ownership

The Kernel consumer run exposed two separate extents: a calendar filename may
validly use years 1000–9999, while an `Instant` is exact signed nanoseconds.
`ParseTimeRequest.Validate` now owns canonical UTC calendar text admission under
Go's complete calendar parsing semantics. `ParseTimeUTC` additionally admits
the parsed value as an Instant. The same private parser owns both contracts;
no second date parser or fallback exists.

`AppendTime` preserves caller-owned storage and its prefix on refusal, allowing
hash and ledger timestamp projections to retain Go's append mechanics.
`FormatNativeTime` admits a typed layout for an SDK's native `time.Time`,
preserving its actual calendar and location, including Go's zero timestamp.
It does not invent a current time or narrow native calendar years to Instant's
extent. These request types are classified in Temporal's inventory and their
public boundaries share the differential semantic fuzz proof.

Retained development proof under `_docs/work/2026-10-06/calendar-text/`
records the exact commands, revisions, cache posture, attempts and artifacts.
It does not issue independent acceptance or claim repository-wide release
verification. Kernel's failed consumer attempt remains visible in its work
queue and must be followed by current contract proof before Kernel publication.
