# Explicit process output contract

Peachfuzz needs to scan arbitrarily long process and evidence streams without a
total output quota. The former mandatory positive Request.OutputLimit could not
express that contract. MaxInt64 would still be a quota, and zero was invalid.

Request and Plan now share OutputPolicy. Its closed mode explicitly chooses
Streaming or Bounded. Streaming requires zero Maximum and forwards bytes;
Bounded requires a positive representable Maximum and preserves the prior
independent stdout/stderr bounds. Zero/future modes and ignored bounds fail
validation. There is no compatibility field or constructor. The serialized
execution plan schema advances to 2. Every affected internal consumer now
constructs or preserves the whole shared policy.

The writer keeps existing stream ownership, cancellation, downstream error
identity, exact byte accounting and output serialization. It retains no whole
output. The output domain still cannot represent a count above uint64.

## Developer evidence

Base revision: 4d18d469852d3e3cca467d505c6c1644bfb6c094. Runs used a dirty
isolated worktree and Go 1.27.1 darwin/arm64. Raw attempts, source patches and
untracked Go sources are retained under
/private/tmp/primitive-process-streaming-evidence. These are local observations,
not independent acceptance receipts.

- Uncached process tests: 981 passing test events, including parent events; no
  failed or skipped events. Exact native output hashes and byte counts cover
  zero, 1 MiB + 1 and 8 MiB + 7 bytes, plus destination failure identity.
- A deliberate mutation rejecting Streaming in observedWriter failed the real
  output tests. The zero-output neutral case passed. The mutation was discarded.
- Migrated-package tests passed for process, gitrepo, machineprobe, release,
  runnercontrol and runworkspace. gotoolchain's native ps observation failed in
  the sandbox; its whole package passed when executed with native permission.
- Full-module compile, focused witness-lint, go vet and Staticcheck passed.
- One 3-second stdout benchmark and three serial 3-second semantic fuzz phases
  passed: OutputPolicy admission, OutputMode JSON and Plan JSON. No performance
  improvement or exhaustive search is claimed.
- The owner changed verification budgets during an earlier 30-second fuzz run.
  SIGINT stopped that run; Go reported PASS, but a separate interruption record
  marks it incomplete. The earlier completed 30-second benchmark and interrupted
  fuzz output remain intact. Current gate and doctrine budgets are at most
  3 seconds, including minimization. Actual elapsed time is recorded separately.

An earlier broad module execution is not a valid revision-bound proof because
process source changed while it ran. It also reported failures/timeouts and
incomplete test-output capture in unrelated packages. Those raw facts remain
retained; they are not erased by focused passes or described as a clean baseline.

Hammer compiled the source and claims with no unavailable package observations,
but reported refused assurance: 4 missing claims, 515 incomplete effect
classifications and 63 direct-effect reviews. Its zero exit is not acceptance.
