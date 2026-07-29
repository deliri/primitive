# Core package recon

Status: `COMPLETE` | Decision: `REDESIGN`

This is the sole recon report for archived package `core`. Production,
consumer, ownership, and independent gap-review evidence is integrated below.

## Evidence boundary

- Archive: `d046f7b675fcb797398d7cdc87b5504f43978056`.
- Kernel HEAD: `fec28ef7c9c0ab7e31bfa72127053f96deefcb59`;
  committed pin `0df2954a2d91`, dirty pin `e8b7172161a4`.
- Witness HEAD: `b9629af57b7058b68982be5d3b282be440b1e76e`;
  pin `773add8ba0fc`.
- Bug HEAD: `39ce96242240d7174d562c90bb255860946595dc`;
  pin `388e593231a2`.
- Peachfuzz HEAD: `2b2d080c455edaadf88502c1c253845605a4336a`;
  pin `3f74d8fc35b4`.

Committed source and worktree state are separated. Witness and Bug each have
only an untracked `.ledger_pending.md`; Peachfuzz has only a modified
`.ledger_pending.md`. Their production source and module files are clean.
Kernel's committed and dirty pins are recorded separately above. The archive
Core tree is clean at HEAD; an untracked
`core/api_http_boundary_hostile_test.go` remains separate dirty-tree evidence.

## Capability ownership

Archived Core admits only product-neutral contracts needed by at least two
independent products. Use by two Primitive packages is necessary integration
evidence but is not sufficient admission evidence. Shared facts cannot be
copied into sibling packages
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/SPEC.md:17`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/SPEC.md:20`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/SPEC.md:30`).

Core owns:

- dependency-free nominal values and closed enums;
- shared stable error identities;
- checked and saturating numeric primitives;
- bounded strict JSON and canonical structural validation;
- typed byte, digest, filesystem-path, HTTP, and protocol primitives; and
- validation rules genuinely shared by multiple Primitive packages.

It does not own product schemas, commercial policy, providers, consumer paths,
consumer errors, or application workflow decisions.

The archive explicitly retires old Core time/duration, money, Garble, storage
transfer, and product contracts after moving them to owning packages
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/SPEC.md:100`).

## Archive evidence

### Primitive-internal demand

Core is not ceremonial. Current archived packages consume it directly, but
every retained semantic fact still needs the independent-product admission bar:

- checked signed arithmetic:
  `archive@d046f7b675fcb797398d7cdc87b5504f43978056:currency/amount.go:68`;
- ceiling-percent arithmetic:
  `archive@d046f7b675fcb797398d7cdc87b5504f43978056:hostresource/memory.go:43`;
- validation, HTTP route semantics, and strict encode/decode:
  `archive@d046f7b675fcb797398d7cdc87b5504f43978056:exchange/server.go:43`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:exchange/server.go:149`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:exchange/server.go:272`;
- typed absolute paths:
  `archive@d046f7b675fcb797398d7cdc87b5504f43978056:filestore/types.go:113`;
- hostile-safe error identity observation:
  `archive@d046f7b675fcb797398d7cdc87b5504f43978056:contextstate/classify.go:10`; and
- provider constants:
  `archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/source.go:47`.

Lifecycle identity types are also used by `submission`, `receipt`, `register`,
and `controlstate`. Wrapper-specific independent-product evidence and the
resulting Core disposition are recorded under `Lifecycle admission`.

## Consumer evidence

### Kernel

Committed Kernel uses:

- typed absolute paths:
  `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:alfred/artifact.go:41`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:alfred/artifact.go:61`;
- HTTP route semantics:
  `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:api/route.go:59`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:api/route.go:72`;
- stable error identity:
  `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:api/errors.go:157`; and
- `Validatable` package-boundary witnesses:
  `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:appboot/config.go:227`.

It also still consumes retired old Core types such as `UnixNanoTime`,
`MoneyPennies`, and product URL/Peach contracts. Kernel's dirty repin removes
some Peach/product coupling but does not complete its time/money migration.
Those call sites are migration evidence, not reasons to restore retired Core
types (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/user/snapshot.go:49`,
`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/order_contracts.go:296`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/order_contracts.go:383`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:pay/pay.go:66`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:pay/pay.go:76`,
`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:adminapi/types.go:317`).

### Witness

Witness consumes strict JSON, bytes, hashes, and doctrine declarations:

- `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/license/store.go:389`;
- `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/updatecmd/store.go:45`;
- `witness@b9629af57b7058b68982be5d3b282be440b1e76e:cmd/witness/license.go:513`; and
- `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/toolbundlearchive/archive.go:236`.

Its `protocol/` and `_foundation_source` trees are preservation copies, not
live Primitive authority
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/FOUNDATION_EXTRACTION.md:1`).

### Bug

Bug consumes Primitive strict JSON, hashes, and bytes:

- `bug@39ce96242240d7174d562c90bb255860946595dc:cli/license.go:518`;
- `bug@39ce96242240d7174d562c90bb255860946595dc:internal/core/strict_json.go:5`;
- `bug@39ce96242240d7174d562c90bb255860946595dc:cli/update.go:453`; and
- `bug@39ce96242240d7174d562c90bb255860946595dc:internal/release/pipeline.go:356`.

Its local strict wrapper validates a Bug-owned authority claim and delegates to
Primitive strict decoding; it is not a second decoder.

### Peachfuzz

Peachfuzz consumes validated encoding/decoding, byte/path values, and HTTP
facts:

- `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/daemon_identity.go:40`, `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/daemon_identity.go:48`, `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/daemon_identity.go:70`;
- `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/archive/run_evidence.go:32`, `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/archive/run_evidence.go:68`;
- `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/archive/gcs.go:438`.

It also directly converts raw strings into exported old path types
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/daemon.go:64`, `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/daemon.go:84`, `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/daemon.go:200`), exposing a
compiler-ownership defect in the archived path API.
Its `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/process/result.go:26` still uses retired
`foundationcore.NanosecondsDuration`, which is migration debt rather than
positive Core admission evidence.

## Strong mechanics and proof

### Numeric safety

`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/numeric_contracts.go:5-85` owns checked narrowing, signed arithmetic,
saturating `int64`/`uint64`, and ceiling-percent projection.

Kernel duplicates unsigned-to-signed narrowing at
`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/numeric_contracts.go:18`. Peachfuzz duplicates saturating
arithmetic at `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/core/saturating.go:5`. These are clean
migration targets, not new competing contracts.

### Strict structure boundary

`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/strict_json.go:14-63` validates output and closes it through bounded
strict decoding. It rejects invalid UTF-8, duplicate/unknown fields, trailing
data, and oversized input; depth, field, and cardinality bounds are enforced at
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/strict_json.go:86`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/strict_json.go:140`, and `archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/strict_json.go:278`.

Witness's canonical signed-ledger JSON is a separate byte protocol and should
not be mistaken for a competing generic strict decoder.

### Hostile error graph

`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/error_contracts.go:5-118` contains panicking error methods, bounds
traversal, avoids unbounded child materialization, and distinguishes matched,
unmatched, and unobservable identity. `contextstate` consumes this primitive.

Consumer product errors correctly remain in each consumer Core.

### Typed bytes, digests, and HTTP facts

Useful primitives include:

- `ByteCount`: `archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/byte_count.go:8`;
- `ByteLength`: `archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/byte_length.go:12`;
- exported `SHA256Hex` with private representation:
  `archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/crypto_hex.go:12`;
- route relation: `archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/exchange_contracts.go:289`; and
- bounded header/query collections:
  `archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/exchange_contracts.go:336`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/exchange_contracts.go:356`.

These have direct Primitive and consumer use.

### Product policy correctly removed

Older pins placed consumer facts in Primitive Core:

- Peachfuzz product/package tokens:
  `archive@0df2954a2d911a5d7d775691d023d569affa2c20:core/product.go:8`;
- Bug repository identity:
  `archive@388e593231a28434f6faae9f0ab9dffcf332dfc3:core/bug_repository_id.go:10`;
- combined Bug/Witness/custody/release schema tokens:
  `archive@388e593231a28434f6faae9f0ab9dffcf332dfc3:core/schema.go:12`;
- Witness machine, retention, payment, and deletion policy:
  `archive@773add8ba0fc1a9453cc06c8558b8541c1fc8ce9:core/witness_policy.go:12`; and
- Peachfuzz evidence limits:
  `archive@3f74d8fc35b4f0f1ddd65ec0e626ee1e06060d75:core/peachfuzz_contracts.go:3`.

The archive removed these. Consumers pinned before the split must migrate their
owned contracts; Primitive v2026 must not restore them.

## Defects and blockers

### Exported string path types

`AbsoluteFilePath` and `AbsoluteDirectoryPath` remain exported defined strings
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/filesystem_contracts.go:10`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/filesystem_contracts.go:12`). Callers can
construct invalid values without a parser, as Peachfuzz does.

The missing contract is a private nominal representation with validated
construction. Lexical path identity may remain Core-owned when shared;
containment, traversal, filesystem observation, and effects belong
`filestore`.

### Raw HTTP field values

`HTTPHeader` and `HTTPQueryParameter` expose raw `Name` and `Value` strings
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/exchange_contracts.go:320`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/exchange_contracts.go:340`).

Kernel already demonstrates private validated header-name/value types at
`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/http_header_contracts.go:22`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/http_header_contracts.go:47`. Generic syntax can
become compiler-owned, while stricter product-specific opaque-value policy
requires independent evidence.

### Repeated enum wire mechanics

Canonical string-enum JSON mechanics are repeated in:

- Primitive Objectstore:
  `archive@d046f7b675fcb797398d7cdc87b5504f43978056:objectstore/enums.go:45`;
- Bug:
  `bug@39ce96242240d7174d562c90bb255860946595dc:internal/core/enum_json_contracts.go:7`;
- Peachfuzz:
  `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/core/enum_json.go:8`; and
- Witness:
  `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/core/enum_text.go:5`.

Evidence supports a compiler-owned on-wire enum contract or shared codec
mechanic, but each owning enum must retain parsing, validation, error identity,
and receiver non-mutation.

### Controlstate commercial constants

Do not carry the archived Controlstate commercial constants into Primitive
Core. The archive combines private implementation names and call counts,
aggregate wire vocabulary, commercial plan policy, and schema bounds in one
Core file (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/controlstate_constants.go:5`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/controlstate_constants.go:39`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/controlstate_constants.go:71`).
It also fixes a 30 minute aggregate lifetime and couples that ceiling to
Callbudget through a compile-time assertion
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/controlstate_constants.go:45`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/controlstate_constants.go:206`).

Consumer evidence settles ownership. Kernel owns a one-member `PlanFree`
domain (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/plan.go:10`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/plan.go:68`). Witness deliberately carries no
usage or teaching state (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/license/state.go:12`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/license/state.go:26`).
Bug owns command usage, teaching state, and local check-in policy
(`bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/state.go:12`, `bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/state.go:24`, `bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/state.go:164`, `bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/state.go:176`). Peachfuzz
has no commercial lifecycle use.

Plan members, offer members, prices, display text, billing choices, term
limits, offline entitlement, usage, rating policy, and receipt policy remain
downstream. Private Go names, file names, package names, function names, and
AST call counts belong in package tests as local ratchets, never in production
Core. The 30 minute lifetime is rejected with the deferred aggregate rather
than retained as a generic default.

If a smaller Controlstate package is admitted later, that package may own only
its noncommercial schema discriminants and package-local bounds. Core owns only
stable shared error identities and cross-package facts that independently pass
Core's admission rule. The matching local conclusion appears in
`_docs/interviews/controlstate.md`, section `Commercial vocabulary is consumer
policy, not Primitive Core`.

### Lifecycle admission

The archive implements one private nonzero 128 bit identity value and six
nominal wrappers. It correctly prevents cross-domain conversion through
private domain tags and owns strict canonical parsing, validation, and JSON
behavior (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/lifecycle_identity.go:11`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/lifecycle_identity.go:36`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/lifecycle_identity.go:63`,
`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/lifecycle_identity.go:102`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/lifecycle_identity.go:107`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/lifecycle_identity.go:320`).

Admission must be decided per semantic wrapper. The shared archive
implementation does not make every wrapper generic.

`AccountIdentity` has independent product evidence. Kernel mints one account
identity and reuses it across every target
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:cmd/seedStarterAccount/write.go:37`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:cmd/seedStarterAccount/write.go:44`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:cmd/seedStarterAccount/write.go:74`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:cmd/seedStarterAccount/write.go:89`).
Witness carries a validated customer identity through its custody request and
response boundary (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/custody/scalars.go:20`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/custody/scalars.go:58`;
`witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/custody/models.go:43`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/custody/models.go:60`). The semantic identity
belongs in Core after a clean representation decision. The archive
hexadecimal wire is not authority.

`DeviceIdentity` has stronger independent evidence. Kernel derives a stable
server-owned device identifier from accepted device evidence using injective
framing (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:auth/device_snapshot.go:125`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:auth/device_snapshot.go:156`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:auth/device_snapshot.go:174`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:auth/device_snapshot.go:188`).
Witness and Bug each persist and validate a cryptographically bound
installation identity (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/license/device.go:76`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/license/device.go:95`,
`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/license/device.go:115`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/license/device.go:139`; `bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/device.go:16`, `bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/device.go:31`, `bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/device.go:64`,
`bug@39ce96242240d7174d562c90bb255860946595dc:internal/license/device.go:84`). A product-neutral device identity belongs in Core after redesign. The
evidence does not support copying the archive's arbitrary 128 bit
representation over the demonstrated digest-based identities.

`SubmissionIdentity` also has independent evidence. Kernel binds retry
identity to the complete evidence projection
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:store/firestore/client_evidence_identity.go:13`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:store/firestore/client_evidence_identity.go:35`).
Witness carries a nominal session identity in its upload grant
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/custody/scalars.go:60`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/custody/scalars.go:138`;
`witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/custody/models.go:90`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/custody/models.go:110`). Bug carries one
validated deploy request identity through prepare and finalize
(`bug@39ce96242240d7174d562c90bb255860946595dc:internal/release/deploy.go:55`, `bug@39ce96242240d7174d562c90bb255860946595dc:internal/release/deploy.go:78`, `bug@39ce96242240d7174d562c90bb255860946595dc:internal/release/deploy.go:185`, `bug@39ce96242240d7174d562c90bb255860946595dc:internal/release/deploy.go:202`;
`bug@39ce96242240d7174d562c90bb255860946595dc:vendor/github.com/deliri/primitive/v2026/release/deploy_transport.go:23`,
`bug@39ce96242240d7174d562c90bb255860946595dc:vendor/github.com/deliri/primitive/v2026/release/deploy_transport.go:49`). The semantic operation identity belongs in Core after redesign.

The archived `OfferingIdentity`, `ObjectIdentity`, and `PlanIdentity` do not
belong in Core for the current design. Current consumers do not demonstrate
one shared opaque identity contract for those domains. Kernel owns its own
product and plan vocabulary (`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/pay_product.go:40`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/pay_product.go:99`,
`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/pay_product.go:116`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/pay_product.go:150`; `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/plan.go:10`, `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/plan.go:68`). Witness and Bug apply
different commercial plan and state policy. Their storage objects are
identified by validated product and provider paths, not by a second shared
opaque identity (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/custody/scalars.go:240`, `witness@b9629af57b7058b68982be5d3b282be440b1e76e:protocol/custody/scalars.go:288`;
`bug@39ce96242240d7174d562c90bb255860946595dc:vendor/github.com/deliri/primitive/v2026/release/scalars.go:252`,
`bug@39ce96242240d7174d562c90bb255860946595dc:vendor/github.com/deliri/primitive/v2026/release/scalars.go:289`).

The ownership disposition is explicit. Account, device, and submission
identities require clean Core designs. The archived offering, object, and plan
wrappers leave Core. A later package may seek admission for one of those
domains only with new evidence from two independent products that need the
same semantic fact.

## Primitive 2026 ownership and DAG

Primitive Core remains the dependency-free owner of genuinely shared value
contracts. The redesigned account, device, and submission identities may sit
in Core because independent consumers need those semantic facts and multiple
Primitive packages must share them without copying.

Offering, object, and plan identities do not enter the current Core node.
Commercial values remain downstream. Package-local wire tokens and
implementation ratchets remain with their owning packages. This keeps the
import direction one way: all Primitive packages may import Core, and no
package imports a sibling merely to share a value contract.

## Decision rationale and conditions

Archived Core already contains important high-quality generic mechanics:
strict JSON, numeric safety, hostile error traversal, bytes/digests, and typed
HTTP semantics. Old product contracts were correctly removed.

The strongest unfinished work is:

- make shared path and HTTP field values impossible to construct from raw
  strings;
- reconcile repeated enum wire mechanics;
- preserve the settled downstream ownership of Controlstate commercial values;
  and
- redesign only the independently proven lifecycle identities.

No consumer constants, paths, errors, schemas, or policy should move into
Primitive Core merely to ease migration.

Every mechanic retained in Core must pass the same two-independent-products
test. Primitive-internal reuse is evidence of placement and coupling, never a
waiver of that admission rule.

## Independent review

Fresh independent review of this current report is pending. Earlier drafting or
review statements do not count as independent current-file evidence. This
report is complete as recon; it is not independently approved.
