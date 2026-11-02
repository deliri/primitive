# TimeMatch SDK hardening checkpoint — 2026-11-02

This is the Primitive SDK checkpoint, not a complete Kernel booking product or
live-calendar acceptance. Source derives from published460 (862b414b), retaining
its StableValue capability. Provider fixes are tested through Exchange against
hostile native HTTP fixtures; no live provider was contacted.

## Calendar safety

- Apple mutations reconstruct the exact event URL, verify its UID and use one
  observed strong ETag. No collection deletion, wildcard overwrite, implicit
  recreation or mutation during recovery. The original unsafe code genuinely
  failed its native fixture before the repair.
- Microsoft records first dispatch separately from recovery. Recovery never
  blindly POSTs; collection lookup supplies candidates only, followed by each
  candidate's documented single-resource ownership expansion. Missing metadata
  remains unknown; contrary ownership conflicts. Immutable IDs survive ordinary
  same-mailbox calendar moves; archive/export absence stays indeterminate.
- Google calendar manager access alone does not establish data ownership.
  Secondary ownership is bound to the authenticated primary calendar identity.
  Updates patch owned fields and preserve user-added notes. Cancelled placeholders
  retire their event identity even when Google omits times. A new appointment
  needs a new identity, never resurrection of an old deleted appointment.
- Google/Apple writes require whole seconds; Microsoft requires100ns precision.
  Unsupported instants are refused before HTTP, never rounded. Stored time and
  browser comparison still preserve exact nanoseconds through Temporal.
- Availability never promotes incomplete/error/null collections to free time.
  Provider403 alone does not establish revoked consent. OAuth omitted scope
  inherits the requested purpose; explicit insufficient scope is refused.

## Evidence

Each retained run has an independently read receipt and execution; those details
agree. Counts below are per-run nodes, not a sum of unique product scenarios.
Failures and semantic controls are retained rather than erased by retries.

| Focused run | Result |
| --- | --- |
| e964f10b Apple original unsafe behavior |7 expected failing nodes |
| a1471f0a Apple repaired behavior |7 passing nodes |
| aea2227e combined provider run |224 pass,3 failing entries in2 Google fixture checks; Apple68 nodes pass |
| e366c30e new Google ownership/precision and documented Graph lookup |6 pass,10 failing nodes before repair |
|63e7d3aa corrected affected provider scope |99 passing nodes, including51 Microsoft nodes |
|2335a68f partial Google busy answer/deleted placeholder |3 failing nodes before repair |
|533825dd corresponding repaired scope |3 passing nodes |
|b70bf332 six independent Google/OAuth/ICS semantic regressions |19 failing nodes,16 passing nodes |
|03f23d41 restored failed leaves only |19 passing nodes,13 requested retry leaves passed |
|e6a1226a two new enum domains |3 passing nodes; all256 values of each domain checked |

Five changed Root parser fuzz targets each passed serial explicit3s searches with
minimization disabled: token588 reported executions, calendar-page1027,
free/busy770, iCalendar free/busy2049 and event parser2350. Apple resource and
specific-ETag targets passed2243/2476 reported executions; Microsoft ownership,
recovery, paging and missing-collection cases have separate retained proof.
Reported fuzz executions are not unique semantic cases.

The final TimeMatch vet/staticcheck/Witness runs at73553e2d all pass with zero
findings. Earlier final selected Core/Temporal vet and staticcheck are clean;
31 source-qualified existing Witness findings in those two packages remain
open, not waived. Firestore's correct native SDK value guard has a separate
owning analyzer request; Instagram's unchanged baseline findings are recorded
separately. No all-Primitive clean-lint claim is made.

## Remaining integration limits

- iCloud free-busy-query support is unverified and reportedly unavailable.
  Unsupported replies remain unknown. No full-event fallback is implemented.
  RFC4791 property projection exists, but actual iCloud privacy behavior is not
  proved. Apple passwords have provider-level access broader than these requests.
- Single-directory Microsoft tenant IDs are not configurable; the SDK exposes
  common, consumers and organizations account-audience modes.
- No live Google/Microsoft/iCloud account, iOS/Android/Windows/Firefox/Edge matrix
  or complete Kernel booking journey is certified by these SDK tests.
- Kernel must atomically reserve time and all-party notices, distinguish provider
  observations from durable bookings, reconcile uncertain changes visibly and
  prevent indefinite retries or silent release after an unknown cancellation.

Raw receipts and source controls remain in the retained private evidence roots,
including timematch-review-followup-PROOF, primitive-apple-mutation-proof,
primitive-microsoft-proof-index and timematch-root-fuzz. They are not checked in.
