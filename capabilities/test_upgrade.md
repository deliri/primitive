# Capabilities production and test upgrade

Scope: `capabilities`. The project-local `_docs/testing_protocol.md` governs
this pass. Repository gates, race checks, and standalone linters remain deferred
at the user's direction. This report records local verification, not independent
acceptance. Sustained fuzzing passed. Final-source profiles and their integrity checks
complete the local measurement record.

## Production findings

The initial package tests passed with 83.3% statement coverage. New regression
tests exposed three categories of mechanical contract failure:

- `Capability.Owns` trusted the package identity while ignoring forged kind and
  role fields. It now requires the complete capability to validate before
  reporting ownership.
- `StandardSymbolFact.Replacement` could advertise an operation whose owner
  contradicted the supplied effect. It now also rejects a retained operation
  that contradicts the symbol's offered operation. Unavailable remains an
  explicit outcome; ownership does not invent an equivalent callable operation.
- Symbol and operation-contract validation sometimes returned only a lower-level
  package error. Refusals now preserve `core.ErrCapabilitiesContract` alongside
  the lower-level error identity.

The original catalog CPU profile attributed about 90% of sampled CPU to
whole-architecture validation. `Catalog.Validate` was validating the full core
architecture again for each derived capability. It now validates the input
catalog once and compares each entry directly with core's authority. It retains
role coverage, exact package membership, and effect-owner checks. No mutable
cache, global registry, duplicated architecture, or execution runtime was added.
The fixed catalog and its lookups remain bounded by the compiled package domain;
this report does not claim constant-time processing of arbitrary input bytes.

## Hostile surfaces

| Surface | Proof |
| --- | --- |
| Capability and requirement | Complete package/scope observations; forged kind/role byte domains for all effect owners; invalid closed unions; exact matches and zero refusal; compiler-derived catalog order and every iterator stopping position |
| Identity, operation, disposition and lexical symbols | Closed byte domains; invalid JSON emits no bytes; exact source-derived JSON decoding; Go identifier lexical boundaries and every keyword; exact nil-receiver refusal |
| Classification | Every secondary subset for every primary owner; operation/disposition/effect combinations; secondary invalid-byte and duplicate refusal; exact ordered round trips; receiver preservation |
| Rule producer → classifier | Complete pairwise domain of 103 valid single-rule outcomes: unresolved, pure, contextual, ten primary-only effects, and ninety distinct primary/secondary pairs; explicit contradiction, neutrality and boundary classes; all 246 invalid effect bytes have the typed-refusal class |
| Handoff invariants | Actual rule producer checked before merge; full retained symbol and classification; reversed order; identical replay; no invented ownership; contradiction returns zero; source refusal retains typed identity |
| JSON ingress | Unknown, duplicate, conflicting and case-drifted fields; absent/null/type-wrong values; contradictory owners/operations; malformed Unicode; truncation; extra document/trailing bytes; exact byte ceiling at maximum−1/maximum/maximum+1 |
| Standard symbols and operations | Exact classifications and replacement operations; unknown namespace/receiver neutrality; public method coverage from Go's compiler; callable function/request/result coordinates bound to real Go functions |
| Structural inventory | Every production struct is discovered and bound through its real marker constraint; all public functions and raw byte/string decoder methods are inventoried; synthetic additions prove discovery cannot hide a new entry or alias |

The handoff matrix has 10,609 valid-outcome pairs plus 246 typed-refusal cases:
10,302 contradiction, 246 typed-refusal, 205 neutral, and 102 boundary cases.
Each case has one primary class. The pairwise domain is exhaustive for the
outcomes one function rule can emit, including its optional single secondary
owner. It does not assert completeness of every possible Go standard-library
symbol or infer behavior for future symbols. Unlisted valid symbols stay
unresolved. Distinct secondary ordering is retained rather than silently sorted.

## Weak tests retired

The existing identity, operation, disposition and classification fuzzers now
compare against source-derived expectations before checking canonical round
trips. A decoder that substitutes another valid value or refuses all valid
seeds fails the oracle. Standard-symbol tables now compare the complete
classification, including the entire secondary slice and offered operation;
the previous single-secondary projection could hide unexpected extra owners.

Compiler-owned structure inventory markers were previously present without a
source-discovery ratchet. The new inventory checks exact discovered membership.
The existing Go method-set inventory remains and checks the installed compiler's
actual exported and promoted methods.

## Execution record

`testdata/test-upgrade-20260907` retains original source/report snapshots, exact
commands, stdout/stderr, source hashes and snapshots, toolchain/environment facts,
coverage, overlays, binaries and profiles. Failed regression runs remain failures
in the record. No failed attempt is replaced with a later passing result.

The final package run, `commit-source-tests.json`, passed 11,426
test/subtest/fuzz-seed events with zero failures and zero skips, at 91.7%
statement coverage. Events include parents and children; they are not counts of
independent bugs or independent proof obligations. Constructor-unreachable
catalog failures are not advertised as public-path coverage.

Fourteen deliberately injected production faults were caught and discarded.
They exercise metadata forgery, contradictory replacement facts, lost error
identity, iterator truncation, JSON value substitution, dropped/duplicated
secondary ownership, hidden catalog contradiction, namespace leakage, and byte
limit bypass. These are mutation checks of test strength, not fourteen bugs in
the original package. Each exact mutation and selected failing test is retained
in `mutations.json`.

The seven original-production profile runs preceded the final source freeze;
test authoring continued during that phase. A further before/after comparison
uses the same frozen test tree and unchanged benchmark fixtures. Its before
side uses a source overlay of the preserved original production. This avoids
claiming that an earlier evolving test-tree snapshot was the final compiled
test tree. Every earlier sample is retained with its historical label.

All nine sustained fuzz targets passed once, requesting 30 seconds each with
one worker: 1,865,855 Go-reported executions in total. The default Go fuzz
cache was used; cache churn is not represented as retained crash evidence. No
new crasher required promotion.

Final review moved a repeated local error-context literal into one package
constant. The literal value, branches, APIs, benchmark fixtures and fuzz
callbacks did not change. The complete package run was repeated on that source,
and `commit-profile-plan.json` captures fresh after profiles for it. Sustained
fuzzing and deliberate mutations retain their original source bindings before
that constant-only correction; they are not relabeled as later executions.
Gates remain deferred.
