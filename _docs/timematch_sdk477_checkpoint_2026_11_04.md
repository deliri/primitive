# Primitive 477 source checkpoint

This cut integrates the reviewed TimeMatch branch through `978a3e36`, the
Google corrections through `49394211`, the contradictory Apple revision fix
`4924e627`, and the pure invitation disclosure extraction `c1ba939e`.
Kernel adoption, deployment and live provider acceptance remain separate.

The reviewed source is merged into main and its feature branch was pruned
only after its tip became an ancestor of pushed main. Its detached checkout,
commits and original evidence remain retained. The earlier review checkpoint
records the composite source binding of its 1,290 passes and failed-only seed
correction; those tests were not replayed as a whole-package release gate.

Google mutations now request only the fields used to prove the owned effect.
Free/busy requests explicitly limit calendar expansion to the existing batch
size. A definitive duplicate event with no owned metadata match returns a
conflict; a read-only recovery with no match remains indeterminate. Delete
may observe only the exact event's ID and status after an empty ownership
query: only a matching cancelled tombstone proves absence. An active, hidden
or unreadable event is never deleted or reported absent. The remaining Google
JSON encoders use the existing jsonio projection door.

The reviewed changes also preserve provider Retry-After as a typed error,
require explicit disclosure for multi-recipient invitations, qualify invitation
UIDs by organizer domain, refuse internationalized mail domains rather than
invent an encoding, and retain/increment CalDAV SEQUENCE on real revisions.
Conflicting duplicate SEQUENCE properties are refused. Calendar identity
validation uses sorted scratch IDs; group matching avoids expansion of an
already impossible match. Browser and provider JSON use the existing decoder.
The credential limit reuses Secret Manager's custody extent. The 254-byte
Apple account and 4,096-byte summary limits are explicit SDK policies, not
claims about every provider's maximum.

Root's Google RED executed 16 failing nodes and one neutral pass at
`b7356b6a`; only the 16 failures were retried at `49394211`, all passing.
The duplicate Apple sequence leaf and parent failed at `2434b18e`; six other
arithmetic cases passed and were reused. At `4924e627`, the seven affected
selections passed 26 nodes without skips, including the failed-only sequence
case, changed Google contracts, real HTTP rate-limit propagation and zero-byte
invitation disclosure refusal.

Both changed external Google boundaries passed serial three-second fuzz
phases: ownership 209 executions/42 new interesting inputs, conference 699/25.
These are bounded mutation samples, not exhaustive provider acceptance.
Original receipt hashes, execution-detail equality, positive reader joins and
author exits are retained in
`/private/tmp/timematch-sdk-review-verification-20261104.json`, SHA256
`a9d0c8cdbff49714f68188ae41007c5ab827960e4b20458f65c80131e77a8ecf`.
The final index binds 21 original pairs, 42 positive reader joins and six
joined authors, including the failed-only complexity retry and final changed
invitation-package analysis at `c1ba939e`.
Root's RED and GREEN originals remain under the
`timematch-root-google-review-*`, `timematch-root-review-sequence-red03` and
`timematch-root-review-affected04` prefixes in `/private/tmp`.

Final vet, staticcheck and errcheck are clean. The new invitation complexity
finding was corrected by extracting its pure disclosure predicate. The exact
failed gocyclo scope then retained 32 prior identities, with no new identity.
Five existing scores increased during the reviewed changes and remain
explicit, unwaived baseline findings: appleUpdatedResource 31 to 32,
groupSweep 18 to 19, groupPreparePhase 13 to 16, groupForbidden 13 to 16,
and groupPreparePerson 11 to 13. This is not an all-SDK clean lint claim.

All provider tests used controlled peers. No production provider write or
live Workspace/CalDAV/Microsoft acceptance is claimed. SDK476's previously
documented query-reference and live Meet qualifications remain. All failures,
setup refusals and available raw originals are preserved on SSD; new HDD
custody is pending.
