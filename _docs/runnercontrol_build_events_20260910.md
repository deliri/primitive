# Runnercontrol interleaved Go build events — 2026-09-10

v2026.1.53, based on 7be8289. This closes build-event ingestion and selected-unit
accounting for real Go compilation failures. It is not the JSON streaming repair
or completion of the runnercontrol sweep.

## Exact boundary

Go 1.27.1 emits BuildEvent and TestEvent on the same go test -json stream.
BuildEvent has ImportPath/Action/Output; TestEvent has a different Package
identity. The installed toolchain documents that ImportPath does not match
TestEvent.Package and implements this in cmd/go/internal/load/printer.go and
cmd/go/internal/help/helpdoc.go. Previously the test-only strict wire decoder
rejected ImportPath, losing legitimate compilation-failure accounting.

An action-only typed discriminator now selects a separate strict build-event
or test-event decoder. Both new internal wire carriers declare data-flow roles
checked by the package inventory ratchet. Duplicate discriminators, cross-wire
members, unknown members, and wrong JSON types remain refusals. Build diagnostic
output never becomes a test unit or benchmark measurement. Dependency build IDs
are not added to the selected-package maps. Only a scalar build-failure flag is
retained; a zero process exit after a build failure is a typed contradiction.
No product completion, authority, or acceptance policy is inferred.

## Evidence

SSH server furnace, repository /work/code/primitive, Go 1.27.1 linux/amd64.
Evidence root:
/work/engineering-evidence/primitive/runnercontrol-build-events-20260910.
Each run records revision/dirty source facts, exact arguments, source snapshots,
environment/toolchain, stdout/stderr hashes and byte counts, exit, and source
stability. These are local author-run facts, not independent acceptance receipts.

Two real-toolchain regressions failed on unchanged production: selected-package
compilation failure, and failure in its dependency. The fixtures create isolated
temporary modules, execute go test -json, require emitted build-fail/ImportPath
members and exit 1, then feed those exact bytes through the public compiler.
After the fix, both produce one failed selected package with no unavailable or
invented dependency unit. The dependency output also seeds the existing semantic
Go-event fuzz target; no build-event seed is fabricated as valid wire JSON.

A local layer triad checks diagnostic neutrality, failure accounting, and typed
rejection. Additional cases reject mixed wire identities, duplicate discriminator,
wrong-type identity, empty diagnostic events, and output on failure events.
The contradiction mutation disables the process-exit check; the corresponding
test fails. Its identity and failure remain retained after restoring production.

Package tests, two race/shuffle repetitions, vet, staticcheck, witness-lint,
module-wide build, and one 30-second four-worker Go-event fuzz phase passed.
The initial witness wording finding was fixed and its failure retained. Strict
errcheck's 19 previously recorded findings in unchanged tests remain open. No
benchmark or performance improvement is claimed by this wire-admission slice.
The discriminator adds a JSON scan; its cost has not been benchmarked here.

## Streaming remains open

GoTestObservationCompiler still accumulates a whole line, retains selected-package
maps and benchmark aggregates, and has the old line extent limit. Go 1.27.1's
encoding/json/jsontext SkipValue still reads individual scalar values through
ReadValue; simply replacing decoding with SkipValue would not establish a fixed
memory bound for arbitrary JSON strings. This slice therefore does not remove the
limit and substitute unbounded allocation, nor claim constant-memory JSON input.

Next proof surface: a fixed-window JSON-string projection that preserves strict
grammar, escaped Unicode, duplicate/unknown-field refusal, exact package identity,
and benchmark facts without retaining arbitrary diagnostic text. Package identity
and benchmark aggregates must have explicit ownership rather than being described
as O(1) in total cardinality. JUnit/profile ingress, lifecycle checks, and broader
producer/accounting review also remain open. The coverage stream repaired in
v2026.1.52 is a separate completed input-extent slice.
