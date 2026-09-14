# Decoded request ownership

This slice keeps destructible decoded values reachable until the HTTP boundary
has either transferred custody or released them. Exchange owns body close;
Controlwire owns route, nonce and replay binding. The typed release callback
does not carry product policy. No product namespace dispatch was added.

The real regression was a valid registration token decoded before a body-close
error: the old zero-result path discarded its destruction handle. A second
regression was a successful decode followed by a route or nonce refusal.
Both now return zero results, release once, and preserve cleanup errors alongside
the original typed refusal. Success transfers custody to the caller.

Local diagnostics are retained under
`/private/tmp/peachfuzz-state-readonly-evidence/`. Each named directory contains
execution.json, exact argument vector, source snapshots and artifact digests.
These are dirty-source development facts, not independent acceptance receipts.

- `primitive-owned-socket-close-red`: real close-failure regression reproduced.
- `primitive-owned-socket-close-green`: lower receiver retains release custody.
- `primitive-owned-route-release-mutation`: deliberately disabled post-binding
  release failed; the mutation was discarded.
- `primitive-owned-restored-and-seeds`: restored release and semantic seeds pass.
- `primitive-owned-package-regressions`: missing Exchange ingress inventory
  entry failed; that failed attempt remains retained.
- `primitive-owned-inventory-complete`: complete controlwire, exchange,
  controlplane and permit selection passed, 20,225 test events, zero skips.
  Parent events are included; this is not an earned-row count.
- `primitive-owned-exchange-fuzz-live`: the six-door semantic oracle passed its
  three-second fuzz phase. It checks exact decoded release identity and count.
- `primitive-owned-staticcheck` and `primitive-owned-vet`: passed before the
  final test-only ingress inventory/oracle additions.
- `primitive-owned-controlwire-doctrine-local`: passed.
- `primitive-owned-race`: focused ownership and ingress inventory tests passed
  twice with race detection and shuffled ordering, 240 events and zero skips.
  The runner's generic cache label for count=2 is imprecise: this command also
  bypasses Go test result caching. The exact argv and shuffle seeds are retained.
- `primitive-owned-token-fuzz-live`: real token custody and canonical closure
  passed a three-second fuzz phase across both owned receiver doors.
- `primitive-owned-exchange-doctrine-local` and
  `primitive-owned-exchange-doctrine-baseline`: both exit 3 with
  `run layer lint: invalid doctrine report`. The latter uses clean base commit
  7fdfa72d341f74e9c58f527808db87e2da9e466e; this is an unresolved baseline
  tool failure, not a clean lint result.

Local Exchange and Controlwire LayerTriad tests exercise real standard-library
HTTP decoding and the real typed AccessRegistrationRequest decoder. No live
provider is contacted. Durable receipt, ledger and provider layers are unchanged
and are not claimed by this ownership slice. Registration handler wiring and
both clients' connected execution remain downstream proof work.

## Version 2026.1.89

The release includes the published v2026.1.88 transport shutdown changes and
declares patch 89 in compass/config.json. `primitive-v89-combined-tests` passed
21,023 test events across controlwire, exchange, controlplane, permit and
gcsobjects. Three live GCS tests were skipped: authenticated deletion retention,
authenticated lifecycle, and private object retrieval. Those are unavailable
provider proofs, not passes. `primitive-v89-combined-vet` and
`primitive-v89-combined-staticcheck` passed over the same package selection.
Publication does not resolve those gaps or issue independent acceptance.
