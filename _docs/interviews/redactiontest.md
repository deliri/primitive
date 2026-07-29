# Redactiontest package interview

Status: `COMPLETE` | Decision: `REDESIGN`

This is the sole reconstruction report for archived package `redactiontest`.
The archive is evidence, not authority. No archived production or test source
was copied.

The package boundary is justified. Four archived Primitive packages import the
same test helper to prove that private signing keys, generic secret material,
Garble custody/build seeds, signed URL bearers, and workload identity tokens do
not escape through generic `fmt` formatting. A fifth Primitive package,
Receipt, depends on the accompanying structural ratchet because it holds a
private signing key in an unexported field. That is a repeated cross-package
security proof, not product behavior and not a reason for those packages to
import one another.

The archive nevertheless overstates what it proves:

- the owned `%.3v` and `%-+#20.4v` surfaces can expose three or four secret
  bytes, while the helper searches no fragment shorter than eight bytes;
- the package's supposed non-vacuity test reimplements disclosure detection
  instead of invoking the helper's rejecting path, so the helper can regress
  while that test remains green;
- its exported `Containment` is a loose `any`/string/bool carrier with no
  `Validate()`;
- its exported `SecretFragments` accepts the empty string, for which every
  rendering contains the returned fragment;
- callers manually remember which raw, hexadecimal, or base64 representations
  of a secret to supply;
- its containment ratchet pins only a count floor, not the exact closed shape
  domain;
- its AST ratchet treats every exact-shape `fmt.Formatter` as secret-bearing
  and resolves only shallow direct or pointer fields; and
- the package declares itself at the Primitive doctrine layer even though Core
  owns an explicit TestSupport layer.

The 2026 result must therefore be a clean, typed reconstruction. Core should
own the shared marker, closed formatting-surface and containment identities,
secret-witness requirements, redaction error identities, and compiler-visible
redaction-witness interface. `redactiontest` should own only the pure evaluator
and the thin `testing.TB` adapter. Production packages must not import it.

## Evidence boundary


### Source revisions and exact Primitive pins

| Source | Exact revision or Primitive pin | Archived `redactiontest` availability | Working-tree qualification |
| --- | --- | --- | --- |
| Archived Primitive | HEAD `d046f7b675fcb797398d7cdc87b5504f43978056` (`2026-07-27T03:35`, `2026-07-27T03:41-04`, `2026-07-27T03:00`, `Harden capability inventory evidence`); package tree `6763dac3decd299682ef44998592e002d0f47fe9` | Present. The complete package and its Primitive integrations were introduced by `40ded9c104a99cbc4b0b672cd7392901b468d1eb` (`2026-07-26T23:14`, `2026-07-26T23:02-04`, `2026-07-26T23:00`, `Harden Primitive comparative contracts`). | One unrelated untracked file exists at `core/api_http_boundary_hostile_test.go`. Every cited `redactiontest`, redaction-Core, Garble, Objectstore, Workloadidentity, and Receipt file is clean against HEAD. |
| Kernel | HEAD `fec28ef7c9c0ab7e31bfa72127053f96deefcb59` (`2026-07-23T04:01`, `2026-07-23T04:52-04`, `2026-07-23T04:00`, `Forbid disabled CSP in production`) | Committed `kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:go.mod:76` pins `0df2954a2d911a5d7d775691d023d569affa2c20` (`2026-07-22T21:25`, `2026-07-22T21:01-04`, `2026-07-22T21:00`), where the package is absent. Dirty `kernel@working-tree:go.mod:76` pins `e8b7172161a4994efcb7f092113e23c28928da43` (`2026-07-27T00:33`, `2026-07-27T00:47-04`, `2026-07-27T00:00`), where the package tree is exactly `6763dac3decd299682ef44998592e002d0f47fe9`. No Kernel production or test source imports it. | Broad pre-existing dirty migration. The committed and dirty pins are separate evidence. Cited Compass/Core files are clean committed source. |
| Witness | HEAD `b9629af57b7058b68982be5d3b282be440b1e76e` (`2026-07-24T15:52`, `2026-07-24T15:58-04`, `2026-07-24T15:00`, `protocol: preserve extracted Primitive contracts`) | `witness@b9629af57b7058b68982be5d3b282be440b1e76e:go.mod:17` pins `773add8ba0fc1a9453cc06c8558b8541c1fc8ce9` (`2026-07-22T07:30`, `2026-07-22T07:53-04`, `2026-07-22T07:00`), where the package is absent. No production or test source imports it. | Only the pre-existing untracked `.ledger_pending.md` was observed. |
| Bug | HEAD `39ce96242240d7174d562c90bb255860946595dc` (`2026-07-24T15:52`, `2026-07-24T15:54-04`, `2026-07-24T15:00`, `protocol: preserve extracted Primitive contracts`) | `bug@39ce96242240d7174d562c90bb255860946595dc:go.mod:9` pins `388e593231a28434f6faae9f0ab9dffcf332dfc3` (`2026-07-20T10:59`, `2026-07-20T10:21-04`, `2026-07-20T10:00`), where the package is absent. No production or test source imports it. | Only the pre-existing untracked `.ledger_pending.md` was observed. |
| Peachfuzz | HEAD `2b2d080c455edaadf88502c1c253845605a4336a` (`2026-07-24T15:52`, `2026-07-24T15:50-04`, `2026-07-24T15:00`, `protocol: preserve extracted Primitive contracts`) | `peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:go.mod:5` pins `3f74d8fc35b4f0f1ddd65ec0e626ee1e06060d75` (`2026-07-23T17:51`, `2026-07-23T17:17-04`, `2026-07-23T17:00`), where the package is absent. No production or test source imports it. | Only the pre-existing modified `.ledger_pending.md` was observed. |

The exact tree checks establish:

- every committed consumer pin predates `redactiontest`;
- Kernel's dirty dependency frontier contains exactly the archive-head package;
- no external consumer currently imports even the available dirty-pin package;
- archive-head Primitive itself has exactly four direct test imports and no
  production import; and
- the external repositories are requirement and gap evidence, not adoption
  evidence.

The four consumer pins also contain no exact-shape
`func (T) Format(fmt.State, rune)` declarations in Primitive. In particular,
Peachfuzz's pin has an `Ed25519SigningKey` with private bytes and no `Format`
method (`archive@3f74d8fc35b4f0f1ddd65ec0e626ee1e06060d75:core/signing_private.go:12-16`, `archive@3f74d8fc35b4f0f1ddd65ec0e626ee1e06060d75:core/signing_private.go:18-39`) and a
`workloadidentity.Token` with an unexported assertion and no `Format` method
(`archive@3f74d8fc35b4f0f1ddd65ec0e626ee1e06060d75:workloadidentity/identity.go:19-21`, `archive@3f74d8fc35b4f0f1ddd65ec0e626ee1e06060d75:workloadidentity/identity.go:36-64`). The archive's
generic-formatting closure is a later proposed upgrade, not a safety property
that can be projected backward onto existing consumer pins.

### Archive census and fresh focused proof

At archive HEAD, `redactiontest` contains:

- 130 lines of non-test Go;
- 146 lines of Go tests;
- a 19-line specification;
- 295 total lines across four files;
- four named deterministic tests;
- no fuzz target; and
- no benchmark.

Fresh focused package gates passed:

```text
go test ./redactiontest
go vet ./redactiontest
gocyclo -over 10 redactiontest
```

The gocyclo command emitted no finding, so no production or test function in
the package exceeds 10.

A fresh focused dependent run produced:

```text
go test ./garble ./objectstore ./workloadidentity ./receipt
```

All four packages passed. A simultaneous `./core` run did not compile because
the archive contains the unrelated untracked
`core/api_http_boundary_hostile_test.go`, which references an undefined
`HTTPSURLMaxRunes`. That dirty-file failure is not attributed to
`redactiontest`, but it prevents this interview from claiming a fresh green
archive Core suite. No file was removed, hidden, or rewritten to bypass it.

The green focused `redactiontest` result proves that its current positive helper
path compiles, its four deterministic tests pass, vet is clean, and complexity
is below the production ceiling. It does not prove that the rejecting helper
path is non-vacuous, that its partial-disclosure claim is true, or that its
public contracts are compiler-owned.

## Capability ownership

The admissible capability is:

> A test-only, deterministic evaluator that applies the complete Core-owned
> generic-formatting surface and containment domains to one validated set of
> secret witnesses, then returns a typed violation when formatting omits the
> redaction marker, violates an exact-marker expectation, or discloses a
> witness fragment covered by the declared detection contract.

The package should own:

- the pure execution of one `fmt` rendering probe;
- the standard sequence that probes each typed containment without erasing all
  values into a public `any` carrier;
- a generic API for a consumer-owned custom containment;
- the thin `testing.TB` adapter that turns a typed violation into a useful test
  failure;
- test-only hostile probes that directly prove each violation identity; and
- bounded diagnostic projection of the surface, containment identity, witness
  identity, and violated rule.

The helper must remain honest about its scope. It proves the behavior of the
specific type, value, witness representations, surfaces, and containments
supplied. It does not prove that every secret-bearing field in a repository was
found. That inventory is a separate compiler/AST ratchet.

### Non-ownership

`redactiontest` must not own:

- any private key, bearer, seed, token, or secret representation used in
  production;
- production `Format`, `String`, marshaling, logging, telemetry, HTTP, command,
  or wire behavior;
- a registry of product packages or product secret types;
- consumer-specific explicit capability crossings such as `Token.Assertion`,
  `Token.BearerValue`, `Seed.String`, or `signedTarget.String`;
- configuration-field redaction, path redaction, evidence sanitization, or
  sidecar transformation;
- a repository scanner, import graph, or production doctrine engine;
- a string-matched error protocol;
- arbitrary formatter correctness for non-secret types;
- exhaustive proof over all possible future `fmt` behavior;
- compatibility with the archived `Containment`, `SecretFragments`, or
  variadic-extra APIs; or
- aliases, wrappers, shims, or copied constants for v2026 call sites.

Core owns the facts shared by production types, test support, structural
ratchets, and doctrine lint. Consumers own the real values and their deliberate
declassification/capability crossings.

## Archive evidence

### One shared generic-format surface

Core owns a closed `RedactionSurface uint8` enum rather than asking each package
to remember a string list. It includes default, detailed, Go-syntax, text,
quoted, integer, floating-point, boolean, width, precision, flag, and unknown
verb surfaces (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/security_constants.go:11-44`).

`Verb`, `IsValid`, `Validate`, `OffWireEnum`, and
`AllRedactionSurfaces` make the surface compiler-visible and explicitly
off-wire (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/security_constants.go:46-64`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/security_constants.go:67-167`).

That centralization is correct. A consumer-local list would permit a new verb
or decorated surface to enter some packages but not others. The switches are
also split into focused functions, and the fresh gocyclo gate confirms the
implementation remains within the complexity ceiling.

### Unconditional production formatters

The archive's protected production types write the Core-owned marker without
switching on the caller's verb:

- `core.Ed25519SigningKey`
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/ed25519_signing_key.go:12-20`);
- `core.SecretMaterial`
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/keygen.go:139-147`);
- `garble.CustodySeed`
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/values.go:15-22`);
- `garble.Seed`
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/values.go:140-147`);
- `objectstore.SignedURL`
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:objectstore/values.go:18-27`);
- Objectstore's private `signedTarget`
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:objectstore/client.go:37-49`); and
- `workloadidentity.Token`
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/identity.go:44-50`).

The verb-switching hostile probe demonstrates why unconditional output matters:
it redacts only `%v` and `%s`, then emits material for every forgotten verb
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:redactiontest/redaction_test.go:23-34`).

### Generic formatting is separated from explicit crossings

The archive does not confuse accidental generic rendering with intentional
capability use.

Garble proves `Seed.String()` remains the command projection and is neither
empty nor the marker after generic formatting is closed
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/secret_format_hostile_test.go:52-68`).

Workloadidentity proves `Token.BearerValue()` and `Token.Assertion()` still
return their exact explicit outputs
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/token_format_hostile_test.go:20-45`).

Objectstore proves `signedTarget.String()` still returns the exact signed URL
used to build the request line
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:objectstore/signed_url_format_hostile_test.go:34-47`).

This is an important 2026 distinction: generic formatting should fail closed;
explicit, validated declassification remains available only through the
production type that owns it.

### Multiple actual containments are exercised

`StandardContainments` constructs ten shapes: bare value, pointer, interface,
exported field, nested exported field, slice, array, map value, slice of
pointers, and map of pointers
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:redactiontest/redaction.go:28-48`).

The generic parameter preserves the actual value type while those shapes are
constructed. That is better than converting the input once to `any` and then
reusing only the erased representation.

The helper also distinguishes whole-operand projections, which must equal the
marker exactly, from container projections, which may include `fmt`'s own
container syntax but must contain the marker
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:redactiontest/redaction.go:73-99`).

The exact/non-exact distinction should survive as a typed Core-owned projection
expectation, not as an unvalidated bool.

### Consumer-owned custom containment is a real requirement

Objectstore supplies the production `signedTarget` and a request-shaped struct
containing it because that unexported bearer wrapper is the actual Exchange
crossing and cannot be inferred by a generic helper
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:objectstore/signed_url_format_hostile_test.go:9-31`).

That is the strongest archived consumer proof. It attacks the real wrapper,
not merely `SignedURL` in isolation, while separately proving that the wrapper's
explicit wire projection still works.

The 2026 helper therefore needs a generic custom-containment entry point. It
does not need a variadic slice of untyped `Containment` bags.

### Raw, encoded, and zero-value attacks

Core tests a private signing key's raw bytes, base64, and hexadecimal text, and
tests both the signing key and secret material at zero value
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/secret_format_hostile_test.go:13-67`).

Garble tests raw and hexadecimal custody/build seed material, the nested
`SecretMaterial`, and zero values
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/secret_format_hostile_test.go:10-50`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/secret_format_hostile_test.go:71-85`).

Workloadidentity tests both a parsed token and the zero token
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/token_format_hostile_test.go:10-18`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/token_format_hostile_test.go:48-55`).

These attacks correctly recognize that:

- a secret can leak in more than one textual representation;
- a zero value can take a distinct buggy path; and
- wrapping one protected type inside another needs its own proof.

The representations currently rely on caller memory, but the attack dimensions
are worth preserving in a typed witness set.

### The archive recognizes the unexported-field trap

The package specification correctly warns that `fmt` cannot call a value's own
formatter when reflection reaches that value through an unexported field
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:redactiontest/SPEC.md:12-15`).

Receipt demonstrates the production consequence. `issueFacts` holds
`core.Ed25519SigningKey` in an unexported `key` field and therefore declares its
own unconditional `Format`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/documents.go:208-224`).

Core's structural test explains the reflection/`CanInterface` failure mode and
requires an enclosing formatter for a directly contained protected type
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/redaction_architecture_test.go:37-79`).

The architectural concern is valid. The scanner that attempts to enforce it is
not complete enough for 2026.

### Production-import exclusion exists

Core keeps `PrimitiveRedactionTestPackagePath` as a compiler-owned path
constant (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/governance_constants.go:61-72`) and scans production Go
files for imports of that path
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/test_support_architecture_test.go:13-67`).

The scan has a non-vacuity floor of 100 production files
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/test_support_architecture_test.go:60-72`).

This is better than relying only on the package suffix or a comment. The 2026
contract must move from a Primitive-local path list to a doctrine rule that
also runs in each consuming repository.

### Primitive dependents at archive HEAD

### Core

`core/secret_format_hostile_test.go` imports `redactiontest` and proves:

- parsed private Ed25519 signing keys do not disclose raw, base64, or hex
  representations;
- zero signing keys use the same generic marker;
- parsed `SecretMaterial` does not disclose raw or hexadecimal material; and
- zero `SecretMaterial` also uses the marker
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/secret_format_hostile_test.go:13-67`).

Core also owns the marker, surface enum, nominal fragment width, architecture
ratchet, and production-import exclusion. Those shared contracts remain Core
responsibilities in 2026.

### Garble

Garble imports the helper for `CustodySeed`, `Seed`, and their shared
`core.SecretMaterial`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/secret_format_hostile_test.go:10-50`).

It additionally proves that generic redaction does not corrupt the explicit
command projection and attacks zero values
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/secret_format_hostile_test.go:52-85`).

Garble's local `hexadecimalText` manually copies the lowercase hexadecimal
digit alphabet and nibble conversion
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/secret_format_hostile_test.go:96-104`). That is an avoidable
local duplicate of a standard encoding contract. A typed secret witness should
be created from actual source material by the owner that knows its legitimate
representations; tests should not hand-code an encoding algorithm or duplicated
digits.

### Objectstore

Objectstore imports the helper for `SignedURL` and, uniquely, supplies two
custom production containments for `signedTarget`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:objectstore/signed_url_format_hostile_test.go:9-31`).

This use establishes the need for a custom typed containment API and for
separate positive proof of an intentional `String()` crossing
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:objectstore/signed_url_format_hostile_test.go:34-47`).

### Workloadidentity

Workloadidentity imports the helper for parsed and zero `Token` values
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/token_format_hostile_test.go:10-18`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/token_format_hostile_test.go:48-55`).

It separately proves both explicit bearer projections
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:workloadidentity/token_format_hostile_test.go:20-45`).

### Receipt's indirect structural dependency

Receipt does not import `redactiontest`. Its private `issueFacts` type is
protected by the Core structural ratchet because it contains a private signing
key in an unexported field
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:receipt/documents.go:208-224`).

That distinction matters:

- `redactiontest` dynamically probes known values and known containments;
- the repository ratchet discovers structurally risky containment sites; and
- Receipt owns the production formatter on its own enclosing type.

Neither proof replaces the other.

## Consumer evidence

### Kernel: a distinct field-projection problem, not a package use

Kernel has no direct import of `redactiontest`.

Its local redaction capability is `compass.Config.Redacted`, a test-only method
that copies the config and manually replaces each known credential field with
`core.RedactedValue`
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:compass/test_support_test.go:5-32`).

That helper is a different concern: it projects an entire configuration for
test comparison rather than proving production generic formatting. Its
field-by-field list has already drifted; `BUG-182` added a regression test for
two credentials that had been missed
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:compass/compass_test.go:395-409`).

The ordinary redaction test also duplicates the raw literal `"[REDACTED]"`
instead of using Core's constant
(`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:compass/compass_test.go:371-393`), although Kernel Core owns
`RedactedValue` at
`kernel@fec28ef7c9c0ab7e31bfa72127053f96deefcb59:core/structured_log_constants.go:10-14`.

This is useful consumer evidence for a structural inventory of secret config
fields, but `redactiontest` must not absorb Config projection or Kernel's
product-owned field policy.

### Witness: byte/path sanitization is downstream evidence behavior

Witness has no direct import of `redactiontest`.

Witness redacts operator roots and the home directory from bounded sidecar
bytes before hashing or persistence. The production boundary records original
byte count and whether bytes changed
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/exec/checks.go:2207-2230`), and the transformation replaces
working roots with `$WORKDIR` and the home with `$HOME`
(`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/exec/checks.go:2291-2305`).

Its hostile data-flow test proves that:

- the recorded byte count reflects redacted bytes;
- the SHA-256 digest reflects redacted bytes;
- persisted bytes are redacted; and
- the original home path is absent
  (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/exec/exec_test.go:7117-7155`).

Witness also projects a redacted hostname and reduced kernel class in its
machine profile (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/plan/plan.go:46-97`) with dedicated hostile
tests (`witness@b9629af57b7058b68982be5d3b282be440b1e76e:internal/plan/plan_test.go:404-484`).

These are production transformation and evidence-layer contracts, not generic
`fmt` formatting. They stay in Witness. A future Witness secret-bearing type
that implements Core's redaction-witness contract could import
`redactiontest` from tests, but no current source does.

### Bug: `fmt.Formatter` is not a secret marker

Bug has no direct import of `redactiontest` and no located secret-redaction
duplicate.

Bug's `GitProofCount` implements the exact `Format(fmt.State, rune)` shape to
preserve width and flags for a public integer count
(`bug@39ce96242240d7174d562c90bb255860946595dc:internal/run/git_proof_count.go:79-88`).

This is direct counterevidence to the archive scanner's central inference. An
exact-shape `fmt.Formatter` is not necessarily redacted or secret-bearing. A
2026 structural ratchet must discover types through an explicit Core-owned
redaction-witness interface or declaration, not by method signature alone.

### Peachfuzz: prospective demand after cutover

Peachfuzz has no direct import of `redactiontest`, because its committed
Primitive pin predates the package.

It does hold a Primitive `Ed25519SigningKey` in the daemon
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/app/daemon.go:35-49`) and a Primitive
`workloadidentity.Token` in `IdentitySource`
(`peachfuzz@2b2d080c455edaadf88502c1c253845605a4336a:internal/googleauth/identity.go:30-41`).

At Peachfuzz's exact Primitive pin, neither type implements generic-format
redaction, as shown in the pin evidence above. Peachfuzz therefore cannot claim
the later archive protection today. After a clean Primitive 2026 cutover,
Peachfuzz should add package-local hostile tests for the real daemon and
identity-source containment shapes if those types can reach generic formatting.
That is a prospective admission case, not current adoption.

## Strong mechanics and proof

The archive demonstrates a valuable test-only proof boundary: hostile values
flow through real generic formatting surfaces, and the test fails when secret
material reaches output. The focused proof, dependent inventory, and consumer
evidence above establish the capability. They do not justify the archive's
reflection-heavy and stringly typed implementation.

## Defects and blockers

### Blocker 1: the fragment-width claim is false for owned surfaces

Core says eight bytes is short enough that precision flags cannot hide a leak
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/security_constants.go:5-9`).

The same file owns:

- `%.3v`, which can truncate output to three units; and
- `%-+#20.4v`, which carries precision four
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/security_constants.go:128-140`).

`SecretFragments` checks only:

- the whole secret when it is shorter than sixteen bytes; or
- the whole secret, its two halves, its first eight bytes, and its last eight
  bytes when it is longer
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:redactiontest/redaction.go:101-117`).

`RequireRedacted` rejects witnesses shorter than eight bytes
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:redactiontest/redaction.go:53-58`).

Therefore a non-exact containment rendering that contains the redaction marker
and also leaks a three-, four-, five-, six-, or seven-byte secret fragment can
pass:

1. the exact-marker rule does not apply to a container;
2. the marker-presence rule passes; and
3. none of the searched fragments is short enough to match the leak.

This is a code-level contradiction, not merely missing test volume. The 2026
detection contract must be designed together with the smallest owned
precision. It must explicitly state which disclosure lengths it can prove and
include marker-plus-partial-leak hostile cases on every non-exact containment.

### Blocker 2: the rejection helper is not proved non-vacuous

The package says its defective probes prove the helper is not vacuous
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:redactiontest/redaction_test.go:44-47`).

The test never invokes `RequireContainmentRedacted` or `RequireRedacted` on a
defective probe. It independently calls `fmt.Sprintf`, independently loops over
`SecretFragments`, independently calls `strings.Contains`, and compares a
local bool
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:redactiontest/redaction_test.go:48-80`).

If the production helper stopped scanning fragments, stopped checking the
marker, inverted `Exact`, or returned before rendering, that test would still
pass. This violates the protocol requirement that a fixture force the target
behavior and fail when the production behavior regresses
(`foundation@working-tree:_docs/testing_protocol.md:157-170`, `foundation@working-tree:_docs/testing_protocol.md:837-850`).

The clean design needs a pure evaluator returning a stable typed violation.
Hostile tests can then call that evaluator directly and use `errors.Is` and
`errors.As`. The `testing.TB` wrapper remains a thin adapter over the already
proved evaluator.

### Blocker 3: the public contract is an untyped carrier

`Containment` exports:

- `Value any`;
- `Name string`; and
- `Exact bool`
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:redactiontest/redaction.go:16-26`).

It has no constructor and no `Validate()`. Empty names, nil values, contradictory
expectations, and arbitrary informal name conventions compile.

This violates the required compiler-owned direction of typed structs, closed
enums, explicit validation, and no untyped helper carriers
(`foundation@working-tree:_docs/testing_protocol.md:118-147`, `foundation@working-tree:_docs/testing_protocol.md:1087-1092`).

The unavoidable final call to Go's `fmt` package accepts an interface value,
but that does not justify exposing an untyped public protocol. Standard
containments should be invoked as separate generic calls. Custom containments
should use a generic typed entry point plus a Core-owned `ContainmentKind` and
typed projection expectation.

### Blocker 4: exported fragment construction accepts a vacuous witness

`SecretFragments("")` returns `[]string{""}`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:redactiontest/redaction.go:105-108`).

The package test explicitly ratchets that behavior as valid
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:redactiontest/redaction_test.go:82-110`).

Every Go string contains the empty string. A direct caller of the stable public
`SecretFragments` API therefore gets a vacuous disclosure match, while only
the separate `RequireRedacted` wrapper owns the minimum-length check.

Validation belongs to the type that owns the witness rule. The 2026 package
needs a typed, nonempty, validated secret-witness set. No public helper should
produce a semantically valid empty fragment collection.

### Blocker 5: representation completeness is caller memory

Core callers manually enumerate raw, base64, and hexadecimal private-key text
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/secret_format_hostile_test.go:31-43`) and raw/hex secret material
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/secret_format_hostile_test.go:50-61`).

Garble separately remembers raw/hex representations and implements its own hex
encoder (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/secret_format_hostile_test.go:27-49`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:garble/secret_format_hostile_test.go:96-104`).

Nothing in the helper's type system says:

- which representation kinds exist;
- which are required for a specific secret source;
- that each witness is nonempty and distinct;
- that the witness belongs to the source material; or
- that adding a new canonical encoding breaks all affected call sites.

The 2026 contract should accept a typed validated witness set with a closed
representation kind. The production type or a test fixture adjacent to its
actual constructor should derive representations through the real standard
encoder. `redactiontest` should evaluate witnesses, not invent secret encoding
policy.

### Blocker 6: the containment ratchet can silently exchange shapes

`TestStandardContainmentsEnumeratesDistinctShapes` requires:

- at least ten containments;
- distinct string names;
- non-nil values; and
- at least one exact containment
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:redactiontest/redaction_test.go:113-141`).

The count is pinned by a package-local `standardContainmentMinimum = 10`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:redactiontest/redaction_test.go:143-146`).

A required shape can be removed and replaced by an unrelated eleventh shape
without failing the test. Names are raw strings and the returned slice is
mutable. The test does not pin the exact expected shape identities or exact
projection expectation of each shape.

Core should own a closed `ContainmentKind` enum because the required shape set
is cross-package. Tests should exhaust every valid enum, sweep the complete
small invalid ordinal domain, and prove the exact projection expectation per
kind.

### Blocker 7: the documented unexported containment is not dynamically tested

The specification says unexported-field containment is what the proof exists
for and that a bare proof is insufficient
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:redactiontest/SPEC.md:12-15`).

`StandardContainments` contains exported fields only
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:redactiontest/redaction.go:32-47`).

That limitation is understandable: an unexported field containing a type with
its own `Format` is precisely the reflection trap, so the correct production
fix belongs to the enclosing type. But the API and specification blur two
different proofs:

- dynamic rendering of standard exported/container shapes; and
- structural discovery of enclosing types that must own a formatter.

The 2026 specification must separate them. Objectstore's explicit custom
`signedTarget` attack is an honest dynamic proof; the repository scanner is the
structural proof.

### Blocker 8: the architecture scanner has both false positives and false negatives

The scanner defines a protected type as any receiver with a method named
`Format`, no results, and parameters whose AST text is `fmt.State` and `rune`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/redaction_architecture_test.go:132-166`).

Bug's public `GitProofCount` formatter proves that signature does not imply
secret ownership (`bug@39ce96242240d7174d562c90bb255860946595dc:internal/run/git_proof_count.go:79-88`).

The field resolver strips at most one pointer and then recognizes only:

- a local identifier; or
- a Primitive-qualified selector
  (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/redaction_architecture_test.go:239-314`).

It does not recursively resolve:

- aliases;
- slices or arrays of protected types;
- maps containing protected keys or values;
- generic wrappers;
- nested composite types;
- interfaces whose dynamic implementations are protected; or
- equivalent consumer-module types.

The scan also uses static floors of twenty packages and six protected types
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/redaction_architecture_test.go:14-20`, `archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/redaction_architecture_test.go:53-65`). Once the repository
is above those floors, losing discovery of a later package or type need not
fail.

The 2026 ratchet should use `go/types`, an explicit Core-owned redaction-witness
interface/declaration, recursive field-type traversal, and an exact
compiler-owned inventory. It must run in each repository that owns protected
types; a Primitive-only scan cannot protect consumer structs.

### Blocker 9: doctrine declares the wrong layer

Core owns `DoctrinePackageLayerTestSupport` as a distinct package role
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/doctrine_contracts.go:8-22`) and gives that role a specific
import policy (`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/doctrine_contracts.go:129-140`).

`redactiontest` instead declares:

```go
doctrinePackageLayer = foundationcore.DoctrinePackageLayerPrimitive
doctrinePackageCapability = foundationcore.DoctrinePackageCapabilityTestSupport
```

(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:redactiontest/doctrine_contract.go:5-8`).

The capability says test support while the layer says Primitive. That weakens
the import graph's compiler-visible statement of the package's role and makes
the separate manual forbidden-import map carry load it should not need to
carry.

The clean package must declare the TestSupport layer and TestSupport
capability. Doctrine lint must reject production imports across Primitive and
consumer repositories.

### Blocker 10: error identity is not represented

`RequireRedacted` and `RequireContainmentRedacted` report failures directly
through formatted `t.Fatalf` strings
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:redactiontest/redaction.go:53-99`).

There is no typed distinction among:

- invalid surface;
- invalid witness;
- invalid containment;
- missing redaction marker;
- non-exact rendering; and
- disclosed secret material.

Even `RedactionSurface.Validate` returns only the broad
`core.ErrPrimitiveContract`
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/security_constants.go:145-153`).

The protocol requires the strongest stable identity and `errors.Is`/`As`
instead of diagnostic strings
(`foundation@working-tree:_docs/testing_protocol.md:644-695`).

Core should own stable redaction contract and violation identities. A typed
violation may carry surface, containment, witness kind, and violated rule for
`errors.As`; it must not retain or print the secret material itself.

### Gap 11: the closed surface test is not exhaustive over its small domain

Core proves the valid surface sequence and checks three invalid values:
unknown, one past the maximum, and 255
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/security_constants_hostile_test.go:5-42`).

`RedactionSurface` is `uint8`, so all 256 ordinals are cheap to exhaust. The
2026 test should verify every ordinal's validity, projection, uniqueness, and
error identity. A three-case invalid sample is weaker than complete proof for a
tiny domain, contrary to the protocol's direction to exhaust trivial closed
spaces (`foundation@working-tree:_docs/testing_protocol.md:474-516`).

### Gap 12: important hostile rendering cases are absent

The package-local suite does not directly prove:

- marker plus a three- through seven-byte disclosure;
- malformed/unknown surface error identity;
- invalid or empty containment identity;
- nil pointer and typed-nil behavior;
- pointer-only formatter method-set behavior;
- map-key containment;
- nested pointer/container combinations;
- custom containment success and failure;
- duplicate or empty witness representations;
- an exact rendering that contains the marker plus extra disclosure; or
- a non-exact rendering that contains no marker but happens not to contain the
  selected witness fragment.

The helper is security test support. Those are hostile contract dimensions,
not optional polish.

### Gap 13: production-import exclusion stops at the archive repository

The archive's Core AST test scans Primitive source and blocks four manually
listed package paths
(`archive@d046f7b675fcb797398d7cdc87b5504f43978056:core/test_support_architecture_test.go:13-67`).

It does not inspect Kernel, Witness, Bug, Peachfuzz, or another future consumer.
A production consumer could import `redactiontest` and Primitive's local test
would remain green.

The Core-owned doctrine identity is the right SSOT. A shared doctrine linter
should inspect each repository's package declaration/import graph without a
copied product registry.

### Testing-protocol assessment

### Compiler-owned contracts: reject archive

The good parts are the Core-owned marker and closed surface enum. The public
`Containment{Value any, Name string, Exact bool}`, mutable containment slice,
raw shape names, separate minimum checks, and caller-invented representation
lists bypass the compiler-visible contract required by
`test/compiler-contracts`
(`foundation@working-tree:_docs/testing_protocol.md:118-147`).

The 2026 public surface must be typed end to end and validate at construction
and evaluation boundaries.

### Real behavior and non-vacuity: partial, blocker remains

Archive consumers use real protected types and Objectstore attacks its actual
wire wrapper. Those are strong direct test slices.

The package's negative probes do not drive the rejecting helper, so the central
test-support path lacks a meaningful red state. This fails `test/evidence` and
`test/fixtures/non-vacuous`
(`foundation@working-tree:_docs/testing_protocol.md:157-170`, `foundation@working-tree:_docs/testing_protocol.md:837-850`).

### Hostile boundaries: reject archive

The suite includes positive, defective, threshold, and shape tables, but it
does not exhaust the small surface domain and misses the marker-plus-short-leak
case that defeats its claim. Serious boundary code should cover valid, invalid,
zero, exact, one-below, one-above, future enum, and malformed inputs or exhaust
the closed space
(`foundation@working-tree:_docs/testing_protocol.md:466-516`).

### Error assertions: reject archive

No pure error-returning evaluator exists, and no test uses `errors.Is` or
`errors.As` for a redaction violation. The helper's diagnostics are useful for
humans but cannot substitute for stable error identity
(`foundation@working-tree:_docs/testing_protocol.md:644-695`).

### Structural ratchets: preserve the intent, replace the implementation

The unexported-field concern genuinely requires an AST/type-system ratchet
because a dynamic standard helper cannot discover every enclosing type. The
protocol explicitly permits structural guards where runtime tests cannot
cheaply prove architecture
(`foundation@working-tree:_docs/testing_protocol.md:202-214`, `foundation@working-tree:_docs/testing_protocol.md:625-638`).

The current scanner's signature inference and shallow field resolution are not
strong enough. Use explicit compiler-owned witness identity and `go/types`.

### Production-path honesty: mostly good

The package is a direct helper by design. Its own tests should be described as
direct helper proof, not integration proof. Consumer tests then connect it to
real production types and, for Objectstore, the real wrapper. That layering is
consistent with the protocol's requirement to name synthetic/direct helper
proof honestly (`foundation@working-tree:_docs/testing_protocol.md:862-875`).

### Fuzzing: not required for admission

The helper accepts already-typed values and operates over small closed enums.
The protocol classifies pure functions over already-validated structs and tiny
input spaces as poor fuzz targets
(`foundation@working-tree:_docs/testing_protocol.md:1210-1249`).

Exhaustive deterministic tests are stronger here. A future parser for an
external witness format would create a separate fuzz boundary, but the package
should not invent such a parser.

### Benchmarks and streaming

No benchmark is required to admit this finite test helper. The standard shape
and surface counts are fixed, and the evaluator should retain only a bounded
diagnostic plus the caller's validated witness set. It must not build a
repository model or accumulate rendered output across cases.

The structural ratchet is test-only and may build the bounded source/type model
needed for doctrine enforcement; production code remains unaffected, which is
consistent with the protocol's structural-test rule
(`foundation@working-tree:_docs/testing_protocol.md:1040-1044`).

### Layer triads and data-flow inventories

`redactiontest` does not capture, persist, replay, verify, or project evidence,
so evidence layer triads do not apply
(`foundation@working-tree:_docs/testing_protocol.md:518-543`).

Its public test carriers still need compiler-visible classification. The 2026
design should avoid an untyped carrier entirely and declare the remaining
test-only structs in the package inventory. Dynamic tests remain necessary;
an inventory is wiring proof, not behavioral proof
(`foundation@working-tree:_docs/testing_protocol.md:1046-1097`).

## Primitive 2026 ownership and DAG

### Potential shared contracts

The corrected candidate does not add Core contracts solely to support a
deferred Redactiontest package. If multiple production packages independently
require the same redaction facts, a future topology review may consider:

- `RedactedValueText`;
- `RedactionSurface` and its exact closed iterator/array;
- `RedactionContainmentKind`;
- `RedactionProjectionExpectation`;
- `RedactionWitnessRepresentationKind`;
- `RedactionViolationKind`;
- `ErrRedactionContract`;
- `ErrRedactionDisclosure`;
- `ErrRedactionMarkerMissing`;
- the smallest detectable fragment/precision relationship;
- the Primitive `redactiontest` package path;
- the compiler-visible redaction-witness interface/declaration; and
- validation rules shared by production types, test support, doctrine lint, and
  tests.

The exact names are reconstruction decisions, not compatibility obligations.
What matters is one typed owner and direct call-site migration.

Core must not retain raw duplicate verb lists in tests. The closed surface
projection should be tested exhaustively against the enum's compiler-visible
domain.

### Package-local test mechanics

Each owning package should use a small direct design:

1. a pure generic evaluator for one typed value, surface, containment kind,
   projection expectation, and validated witness set;
2. a generic standard-containment runner that constructs each shape and calls
   the evaluator immediately, without returning a heterogeneous public slice;
3. a generic custom-containment runner for a consumer's real wrapper;
4. a typed violation that retains Core identities and never includes secret
   bytes in `Error()`; and
5. a thin `testing.TB` requirement helper over that pure evaluator.

Every public struct and enum must own `Validate()` where it owns a rule.
Ingress validation occurs before formatting. Typed violations validate before
external diagnostic output.

### Compiler-owned structural witness

Production secret-bearing types should satisfy an explicit Core-owned
redaction-witness contract in addition to `fmt.Formatter`. Compiler witness
assignments should make method-set changes break the build.

The repository ratchet should:

- load packages with `go/types`;
- discover only explicit redaction witnesses;
- recursively inspect aliases, pointers, arrays, slices, maps, embedded fields,
  and generic wrappers;
- identify the exact enclosing type and field path;
- require an explicit enclosing redaction witness where reflection can bypass
  the inner formatter;
- use an exact compiler-owned inventory rather than static minimum floors; and
- run in Primitive and every consumer repository.

The dynamic helper and the structural ratchet must remain separate named
capabilities.

### Consumer ownership after cutover

Primitive packages should migrate directly:

- Core derives raw/base64/hex witnesses from actual key material through
  standard encoders;
- Garble deletes the handwritten hexadecimal encoder and uses typed witnesses;
- Objectstore keeps its real `signedTarget` custom-containment attack;
- Workloadidentity keeps explicit bearer/assertion projection tests; and
- Receipt remains protected by the recursive structural ratchet.

Peachfuzz should add real wrapper tests once it adopts Primitive 2026 secret
types. Kernel's Config projection and Witness's sidecar/path redaction remain
product-owned and must not be routed through this helper.

## Decision rationale and conditions

### Conditions for future reconsideration

The corrected `v2026.0.0` topology keeps redaction helpers package-local.
A shared `redactiontest` package may be reconsidered only when all of these are
true:

1. The package declares both
   `DoctrinePackageLayerTestSupport` and
   `DoctrinePackageCapabilityTestSupport`.
2. Core owns one closed surface domain, one closed containment identity domain,
   shared projection expectations, redaction error identities, and the explicit
   redaction-witness contract.
3. No exported helper contract contains `any`, a loose map, an informal raw
   name, or an unvalidated bool protocol.
4. Every public type validates at its ownership boundary.
5. A pure evaluator returns typed violations, and the `testing.TB` wrapper is
   proved to delegate directly to it.
6. Hostile tests directly prove, with `errors.Is` and `errors.As`, invalid
   surface, invalid witness, invalid containment, missing marker, non-exact
   marker, and disclosure identities.
7. Marker-plus-partial-leak cases cover every fragment length relevant to the
   smallest owned precision; the implementation and documentation make no
   stronger claim than the evaluator can prove.
8. The complete `uint8` surface and containment ordinal domains are exhausted.
9. The standard containment set and exact/non-exact expectation for every kind
   are pinned exactly, not by a minimum count or raw name.
10. Nil, typed-nil, pointer-only method set, map key/value, nested container,
    zero value, unknown verb, width, flags, and precision behavior are hostile
    test cases.
11. Witness representations are typed, validated, nonempty, distinct, and
    derived by the source owner through real encoders.
12. The structural ratchet uses explicit witness identity and recursive
    `go/types` resolution rather than assuming every `fmt.Formatter` is secret.
13. Primitive and consumer doctrine gates reject production imports of the
    test-support package.
14. Core, Garble, Objectstore, Workloadidentity, and Receipt focused suites
    pass in a clean archive-free Primitive 2026 tree.
15. `go vet`, `staticcheck`, relevant doctrine/sentinel gates, and
    `gocyclo <= 10` are green.
16. No v2026 alias, wrapper, variadic compatibility adapter, copied literal, or
    retired call path remains.

### Current rationale

The capability is justified, but the archived implementation and API are not.

The package has one exclusive owner-worthy reason to exist: reusable hostile
proof that Primitive secret-bearing types fail closed across one shared,
closed generic-formatting contract. Four direct Primitive test dependents and
Objectstore's real wrapper attack establish genuine demand.

Do not copy `Containment`, `StandardContainments`, `SecretFragments`, or the
variadic-extra API. Do not preserve their names through shims. Rebuild the
capability around Core-owned enums, validated witness structures, stable typed
violations, a pure evaluator, and a separate recursive structural ratchet.

The archive's strongest ideas are the centralized surface set, unconditional
formatters, real containment attacks, zero/encoded witness cases, explicit
capability-crossing tests, and production-import prohibition. Its blockers are
contractual, not cosmetic: a security proof that misses its own truncated leak
and cannot prove its own rejecting path is not admissible unchanged.

The recon report is complete. Implementation readiness remains blocked until
the typed 2026 contract is implemented, the conditions above are green, the
four Primitive call sites are migrated directly, Peachfuzz's post-cutover
containment demand is assessed, and fresh independent review accepts the
result.

## Independent review

Fresh independent review of this current report is pending. Earlier drafting or
review statements do not count as independent current-file evidence. This
report is complete as recon; it is not independently approved.
