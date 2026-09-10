# Filelock upgrade — approved for v2026.1.48

User approved the reviewed Filelock slice for bump, commit and push on 2026-09-10. Base revision: 2c3ea10fab1e1c036fa03bbac3dfd7c35da390f2 (v2026.1.47). Source tree: /work/code/primitive on Furnace.

## Production and ownership

Acquire and Release now borrow the descriptor through Go's os.File.SyscallConn().Control. The prior File.Fd path cleared O_NONBLOCK on a Go-owned pipe on Linux; it also bypassed Go's borrowed descriptor lifetime. A recorded red test demonstrated the mode change before the fix. Control preserves the poller mode and holds Go's descriptor reference through the native operation.

Linux and macOS use the standard library's syscall.Flock directly. Windows retains the native x/sys/windows LockFileEx and UnlockFileEx bindings. Windows overlapped operations use a pinned OVERLAPPED, a private event with its documented completion-port suppression bit, and GetOverlappedResult to join pending native completion before releasing storage or Go's descriptor reference. This preserves Go's IOCP ownership. Native contention is classified before cleanup errors are joined, so event cleanup failures cannot become neutral contention.

The Windows mechanics follow [LockFileEx](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-lockfileex), [completion-port event suppression](https://learn.microsoft.com/en-us/windows/win32/api/ioapiset/nf-ioapiset-getqueuedcompletionstatus), and [UnlockFileEx](https://learn.microsoft.com/en-us/windows/win32/api/fileapi/nf-fileapi-unlockfileex). Go source references: os/rawconn.go, internal/poll/fd_posix.go, os/file_unix.go, os/file_windows.go and internal/poll/fd_windows.go from the installed Go 1.27.1 tree.

No public API shape changes. Core owns stable error identities. Invalid requests remain ErrPrimitiveContract; native/Go descriptor failures remain ErrFileLockUnavailable with their original cause. Closed files now preserve Go RawConn.Control's refusal instead of inducing a kernel EBADF; tests derive the actual Go error identity rather than matching diagnostics or fabricating fs.ErrClosed.

Acquisition is an immutable observation of one attempt, not a live ownership monitor. Immediate contention produces a valid observation with Held=false. Blocking calls validate context at admission and then obey the native blocking operation; they do not promise cancellation while parked. Closing the same handle waits for its borrowed operation to finish.

Memory stays O(1) per operation. File contents are neither read nor buffered, and there is no file-size ceiling, retry runtime, worker queue, product policy or state machine. Windows' whole native range is a lock coordinate, not a payload quota. Supported platform scope is Linux, macOS and Windows.

## Hostile proof

The entire local testing protocol was read before editing tests; SHA-256: 5eaec400ce3eda457f3683fdacac8ec8f4f0f51ee229643b0f7f62ea420e6ec7.

Tables exercise the shared/exclusive compatibility product, release and close handoffs, valid/unknown/future/max enum representations, no-effect invalid requests, cancelled/expired/nil/typed-nil contexts, exact native flags, closed descriptors and unchanged descriptor modes. Independent file handles probe actual kernel ownership; zero observations cannot claim an effect. Both public input doors are compiler-bound and checked against the production AST.

The blocking handoff table does not pretend a goroutine notification proves a parked syscall. Separate exact-native-flag cases prove Blocking semantics. The Windows pending-completion table observes actual ERROR_IO_PENDING while another holder owns the range; that table was compiled but could not execute on this Linux host.

Seven deliberate defects were killed by tests, not compilation failures: bypassed descriptor custody, erased native cause, ignored context, invented hold on contention, no-op unlock, removed nonblocking flag, and accepted unproduced observation. All mutations were restored.

One semantic ownership fuzz campaign passed with 205,043 executions at 30 configured seconds and four workers. It exercises both Acquire and Release against actual independent file handles, closed/cancelled states and invalid enums. Blocking fuzz cases are deliberately uncontended to avoid an unbounded native wait; deterministic tables exercise contention and handoff. The run began with seven corpus entries and discovered four additional interesting inputs. Existing fuzz-cache state is not claimed empty. No cases were skipped.

## Validation and measurements

Filelock tests passed uncached under the race detector with 95.1% Linux statement coverage. Scoped go fix, go vet, staticcheck, strict errcheck (-blank -asserts), witness-lint, production gocyclo <=10 and goconst (four characters, three uses, excluding tests) passed. Windows vet, staticcheck, errcheck and witness-lint passed. Filestore LockFile integration tests and the full production build passed. macOS/arm64 and Windows/amd64 test binaries compiled. Native runtime evidence is Linux only; macOS and Windows runtime verification remains unavailable. The EINTR retry branch was not forced. Full-module tests and remaining global gates were deferred.

Four before/after benchmark cases each ran once for 30 configured seconds with CPU/memory profiles and retained matching binaries. [Before](../filelock/before_benchmarks.md) and [after](../filelock/after_benchmarks.md) contain full commands and observations. The parent benchmark gained b.ReportAllocs after baseline to satisfy Witness; fixtures and every measured child loop are unchanged, verified against content snapshots.

CPU profiles remain dominated by the native syscall path (71.26% flat CPU after the change). The descriptor borrow adds visible fixed CPU cost; all four cases retain 0 B/op and 0 allocs/op. Allocation profiles contain benchmark/profiler/runtime setup, with no per-operation allocation reported. The 5–9% observed timing increase is retained and disclosed; correctness of Go descriptor custody takes precedence over reverting to File.Fd.

Historical failures remain visible: the initial red descriptor run, a test error-variable inference compile error (including Windows compilation), and Witness's missing parent allocation reporting/error-comparison findings. Final checks pass; those earlier runs are not relabelled successes.

## Evidence

Raw evidence: /home/d/engineering-evidence/primitive/filelock-upgrade-20260910. Each run records the full base commit, exact command, toolchain, environment, source hashes/content snapshots, output, exit, duration and source stability. measurements.json, mutations.json, manifest.json and audit.json preserve the checked work. Pre-release evidence binds the base plus exact source snapshot; it does not pretend to have run on a future release commit. Large binaries, profiles and source snapshots remain outside Git. release.json records the committed revision and verified remote references.
