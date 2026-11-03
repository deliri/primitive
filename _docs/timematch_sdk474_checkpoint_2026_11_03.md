# Primitive 474 source and verification checkpoint

This release collects the typed integer-JSON query, JSONIO native codecs and
Go-owned SDK object projection, Plunk filename hardening and current-process
cgroup ancestor headroom. It does not complete or deploy the TimeMatch product.

## Collected source

- Query/typed-codec branch through `76408b78b`; both source histories are merged.
- Direct Go object projection replaces the unpublished handwritten JSON walker;
  the removed source remains in the merged immutable parent, not in the release.
- Headroom six-file source `37ee4c36`, delivery SHA256
  `59e79f5747bd06811aeff12b7359fee6259ff45867424c46962e0ce8b9a4b42d`.
  Headroom observes current membership; it is not future job admission or a lease.
- Plunk attachment hardening through `3f0de34e`.

## Retained native proof

Original receipts, executions, positive reader joins, request scope, source and
failure attempts are retained under `/private/tmp/timematch-sdk474-evidence` and
its referenced prefixes; raw proof is intentionally outside this repository.

- Query original: `0a46cc8c` (41 passing nodes). Collected caller-codec proof
  `149205ba` (528 passing, three failed), corrected failed-only `c8abf431`
  (7 passing); the failures were explicit new fixture expectations.
- Changed Go integer projection / real Firestore selected reads, writes and
  query: `7922eced` (141 passing, no skips).
- New object boundary: `546f7fa9` retained the nil-map refusal failure; Go
  otherwise spells nil maps as `{}`. Explicit nil admission was corrected;
  changed-only `7287218f` passes, retaining the other original passing rows.
- Opaque native-root refusal and independently owned parallel restore counters:
  `f16d0cae` passes the changed boundary selection.
- Meaningful omission controls: `2f5aacbd` fails on lost integer precision;
  `abd69c27` fails on loss of the owner cause. Private control commits are
  non-ancestors of the release; proper source was restored at first invocation.
- Serial semantic fuzz: `e916e27e`, 3 seconds, 27611 reported executions / 13
  interesting; `631d4357`, 3 seconds, 13288 / 23. These are execution facts,
  not unique-case coverage or performance claims.
- Compass version and authored-note source `671c559f` passed its selected
  configuration roots. Later source changes leave those metadata bytes equal.
- Plunk originals: `b1dc1cd2` intended refusal control, `1687e66b` with one
  obsolete boundary row, corrected-only `2e6ae550`, semantic `6ea95804`.
- Headroom: source proof index SHA256
  `2f6475bca7d61eb67fa91ac01b93c26cfbd33f9be956a807cb8a0d831b210e96`
  retains native observations, six intended omission failures and five safe
  neighbors, semantic counter fuzz, authors and original-reader joins.

## Analysis qualifications

Final changed JSONIO / Firestore vet and staticcheck are clean. Plunk vet,
staticcheck and errcheck are clean. Existing errcheck remains open at five
positions in four files byte-identical to published 473. Witness retains 71
JSONIO/Firestore findings and 61 Plunk findings: these are not green, excluded,
suppressed or waived. They include the native Go `*any` decoder metadata hook
relocated from the published integer writer to its JSONIO owner; no native
map escapes Primitive into Kernel. New test-context and parallel-ownership
findings were corrected, not suppressed. Headroom's exact inherited findings
and original failed analyses remain qualified in its source delivery.

There is no live provider/account, inbox/import, every-platform, global SDK
lint or whole-product completion claim. Kernel adopts only the actual published
module after source, ZIP and module sums have been independently checked.
