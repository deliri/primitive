# Compass upgrade — reviewed, 2026-09-10

Submission is published as v2026.1.45, commit 204e07d7cf9a9270a0d9a03e101b43de1823527d. The Compass slice was reviewed and approved for release v2026.1.46.

## Production changes

- Remove Compass's 1 MiB document ceiling and inherited 1,024-element array ceiling by using Core.ExtensibleJSONLimits. Rejected configurations remain exactly zero with native, JSON and Compass error identities preserved.
- Remove the 128-byte ProjectName ceiling. Nonempty, valid UTF-8, no surrounding whitespace and no controls remain the admission grammar. Names are preserved exactly, including Unicode representation.
- ProjectName.String returns its immutable private value directly. Public construction and JSON admission establish validity; the zero value contains empty text. The measured redundant Validate scan is removed without adding caching, mutable globals, interfaces or a second parser.
- Remove the two obsolete public size constants and their test references. Correct stale decoder and release-coordinate comments.

No new effect implementation, provider, workflow, state machine, compatibility path or product policy. No consumer repository or Witness source changes.

## Hostile tests and fuzz

The entire project-local testing protocol was read before editing tests. Its unchanged SHA-256 is 5eaec400ce3eda457f3683fdacac8ec8f4f0f51ee229643b0f7f62ea420e6ec7.

Replace the padded count matrix with earned exact-result rows. Local table triads cover document framing beyond the old ceiling, typed arrays beyond the old count ceiling, name grammar and exact Unicode, nil/invalid/fragmented readers, native terminal failures and cancellation, receiver preservation, caller-owned validation, release representation boundaries, duplicate/unknown fields and malformed inputs. Current's compiled configuration is checked against an independent standard-library decode, including caller-copy isolation. Current has no variable external input: two fixed-source rows cover its actual contract without fabricated negative input.

The unchanged production failed the new quota boundary rows in red-boundaries. Five deliberate defects were killed in six targeted runs: restored JSON defaults (document and array checks), restored name quota, erased native error cause, cleared JSON receiver, and reject-all name admission. Each mutation was reverted. Exact changed source and failing output are preserved.

Two semantic fuzz targets cover Decode and both ParseProjectName/ProjectName.UnmarshalJSON doors. Seeds include typed production output; callbacks check independent Go decoding/Unicode grammar, generated valid input acceptance, exact values, typed zero/preserved refusals and byte-stable canonical re-encoding. Both passed once at 30 seconds and four workers: **614,737 executions**, 60 configured seconds. Actual elapsed durations and corpus growth are retained. This is not a claim of fresh empty fuzz caches.

## Validation

- Scoped race tests passed with **100.0% statement coverage**. Coverage alone is not a correctness proof.
- go fix, go vet, staticcheck, errcheck, witness-lint, production gocyclo <=10 and goconst (minimum 4 characters, 3 uses, no tests) passed.
- Full-module production go build ./... passed.
- macOS/arm64 and Windows/amd64 Compass test binaries compile; runtime tests were on Linux only.
- Full-module tests and the remaining global gates were not run.
- Five before/after benchmark cases ran with CPU/memory profiles and retained binaries. [All observations](../compass/after_benchmarks.md) include unchanged decoder allocations and the getter result.

The initial Witness failure for an unactionable test diagnostic remains recorded; its corrected final run passes. Intended red and mutation failures remain failures in the evidence.

## Evidence and remaining ownership limits

Raw evidence: /home/d/engineering-evidence/primitive/compass-upgrade-20260910. measurements.json includes absolute measurements and fuzz counts; mutations.json records each change; manifest.json inventories artifacts and the final reviewed source tree. Per-run receipts preserve exact full base revision, uncommitted source hashes and content snapshots, command, toolchain, selected environment, duration, status and output. They do not claim execution on a later release commit. Large binaries, profiles and snapshots stay outside Git.

Compass returns a complete caller-owned T. Core currently retains the raw JSON document while decoding, and its strict schema scanner still owns the 64-level nesting and 256-field object limits. This slice removes Compass's document/name quotas and inherited array quota; it does not claim an O(1)-memory JSON decoder or removal of Core's schema limits. A full-document string or slice in T also necessarily retains its content. Fixing the shared decoder belongs in a separate Core slice, rather than adding a competing decoder to Compass.
