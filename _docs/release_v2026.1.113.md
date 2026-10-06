# Primitive v2026.1.113 — exact UTC calendar mechanics

Temporal owns typed UTC date admission, ISO week and weekday observations, and
Go calendar shifts. `UTCDateTime.Instant` refuses dates or clocks that Go would
normalize and preserves signed-nanosecond representability. `Instant.AddCalendar`
deliberately uses Go's documented calendar normalization. Neither operation
reads a clock or retains calendar state. Kernel selects reporting periods.

Month and weekday meanings are closed compiler-visible enums. Input, observation
and delta structures are classified in Temporal's inventory; admitted date
coordinates are tied to their semantic fuzz target.

Development proof is retained under `_docs/work/2026-10-06/utc-calendar/`.
A mutation removing exact date admission produced seven failed tests, with the
original source retained. The race suite passed 935 tests without skips across
Temporal, Compass and Version. Three-second fuzz runs with two workers executed
114,659 coordinate admissions and 91,551 native calendar parity checks.
Repository-wide errcheck and staticcheck passed. Targeted vet and alignment
passed; the alignment failure and repair are retained. This records development
proof, not independent acceptance or repository-wide release verification.

Compass's inherited release coordinate was still 110 after the published 111
and 112 tags. This release sets the authoritative coordinate to 113. Kernel
must consume the published tag with GOWORK disabled and verify its source SHA.
