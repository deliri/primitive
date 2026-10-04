# Final execution results

Source: `0e6c118e1d0c5db042da9054a3eee8a61e766254`; clean Mac and Furnace checkouts, Go 1.27.1.

**The complete required gate is not passing.** Existing static findings and the library-only deadcode invocation remain explicit. Passing package tests do not erase these results.

| Gate command | Exit | Result detail |
| --- | ---: | --- |
| `go fix ./...` | 0 | Passed. |
| `go vet ./...` | 0 | Passed. |
| `fieldalignment ./...` | 3 | Existing layout findings outside the new SDK/signing files. |
| `gocyclo [complete discovered source/package scope; see receipt]` | 1 | Existing Plunk functions at complexity 15, 11 and 11. |
| `goconst -ignore vendor -min-occurrences 3 -min-length 4 -ignore-tests ./...` | 0 | Exit 0 with existing repeated-token diagnostics; output is retained. |
| `nilaway [complete discovered source/package scope; see receipt]` | 0 | Passed. |
| `errcheck ./...` | 0 | Passed. |
| `staticcheck ./...` | 0 | Passed. |
| `deadcode ./...` | 1 | Library module: tool exits 1 with “no main packages”; no fake entrypoint was added. |
| `deadcode -test ./...` | 0 | Passed. |
| `govulncheck ./...` | 0 | 0 reachable vulnerabilities; unused dependency findings remain in raw output. |
| `gosec ./...` | 0 | Passed. |
| `witness-lint ./...` | 1 | 16 existing findings across 8 unchanged files; existing waiver counts are retained, no waiver added. |
| `go test -json -count=1 ./...` | 0 | 68 packages pass; 3 live GCS smoke skips; 7 expected nested rejection events. |
| `go test -json -race -shuffle=on -count=2 ./...` | 0 | 68 packages pass; 6 live GCS smoke skips over two passes; 28 expected nested rejection events. |

## Behavioral results

- Mac full ordinary suite: 68 passing packages; 90,794 pass events, 7 expected nested rejection events, 3 skips.
- Mac full race/shuffle/count=2 suite: 68 passing packages; 181,600 pass events, 28 expected nested rejection events, 6 skips.
- Furnace focused race/shuffle/count=2 suite: 27782 pass events, 0 failures and 0 skips across Cloudflare, Exchange, Core, Objectstore and authored claims.
- Mac semantic fuzz: 12 targets, 136382 executions, 0 failed targets.
- Furnace semantic fuzz: 12 targets, 70527 executions, 0 failed targets.

Counts include subtests and seed executions, not unique requirements. The exact skipped tests and all raw failure events are retained in `final-summary.json` and the archived logs. Three unavailable live GCS smoke tests are not represented as passed. No live Cloudflare provider smoke is claimed.

## Existing gate findings

The Witness findings are in `_tools/fieldalignmentgate`, Controlplane registration, Filestore copy/sort helper signatures, Google Identity IAM, Plunk Operation enum contracts, and Runnercontrol enum switches. `gate-findings-baseline-bindings.json` in the history archive proves these production files are byte-identical to baseline `64264429f65417c96ea1e38c481c4f6ff9f9aeb1`.

These remain open proof surfaces, not waivers or successful checks. Raw fieldalignment output also retains existing protocol/layout findings. No blanket layout rewrite was used to change unrelated signed wire structures. The direct `deadcode ./...` invocation is unavailable for this library shape; `deadcode -test ./...` completed successfully.

The final source checkpoint adds no Witness waiver. The existing repository waiver inventory printed by Witness remains visible. All final command scopes and complete expanded argument vectors are in `final.attempts.json` and the individual archived receipts.

The final gate launcher exits 1, matching the failed child commands. This is implementation evidence for review, not independent acceptance.
