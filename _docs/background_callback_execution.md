# Owned asynchronous callback execution

`background` owns a fixed native Go worker set, a bounded resident callback
queue and one cancellation/join observer. It retains no identities, durable
jobs, schedules, retries or lifetime work history. Products lend validated
callbacks and retain their own policy.

The public handoff is `Open(OpenRequest{Parent, Policy})`, nonblocking
`Executor.Submit(Work{Run, Complete})`, and `Executor.Stop(ctx)`. Policy declares
worker count, waiting capacity and a typed cooperative timeout. Liftclean can
select four workers, 256 waiting callbacks and 600 seconds. Capacity refusal
transfers no ownership. Every accepted callback gets exactly one completion,
including cancellation before its action starts. Stop seals admission, cancels
the lifetime and returns nil only after actions and completions have returned.
A callback that ignores cancellation cannot be forcibly stopped; an incomplete
join preserves the native caller cancellation/deadline. A later Stop can join.

Core owns compiler constant contract/capacity/stopped/callback-panic identities.
Native error-valued panics retain their original cause. Action failures go
directly to completion; only a first completion-callback panic is retained for
Stop. No growing error chain or work history is accumulated.

Furnace proof used an isolated archive of
`9e5b824d76bf3f018ae6e67ac9a78f873322e830` with explicit owned overlays.
Unrelated user PayPal source was excluded; the shared export ratchet used only
the additive Background catalog hunk. Evidence remains outside Git under
`/data/evidence/processed/primitive/2026-10-13/background/`.

| Attempt | Observed result |
| --- | --- |
| background-native-and-core-green01 | 2025 pass / 4 fail / 0 skip. Native Background and authored claims passed; broad Core was non-accepting. |
| background-existing-core-baseline01 | Untouched published source reproduces exactly the same four Core failures: two RelativePath JSON witnesses, its external-door fuzz inventory, and preexisting Geteuid/Uname native effect counts. |
| background-false-join-red01 | Setup failure: external mutation overlay unavailable after refused SSH setup; no tests executed. |
| background-false-join-red02 | Genuine one-fact mutation: Stop returns nil on an incomplete join. The blocked-completion test fails, 0 pass / 1 fail. Original production bytes stay unchanged. |
| background-matching-domain-green02 | 56 pass / 0 fail / 0 skip for declared changed-domain Background, Core error/architecture/export and claim scope. |
| background-final-native-green03 | 23 pass / 0 fail / 0 skip; superseded cause-selector fixture. |
| background-final-native-green04 | Final internal-cause fixture: 23 pass / 0 fail / 0 skip. |
| background-callback-fuzz-build01 / fuzz01 | Superseded Core JSON selector bridge. Preserved as an experiment; not a new Background wire grammar. |
| background-callback-fuzz-build02 / fuzz02 | Actual retained race/fuzz-instrumented final callback binary; configured 3-second serial mutation, 1100 executions, exit 0. Engine reports mutation at 3 seconds and shutdown at 4 seconds; complete runner elapsed 5.202 seconds. |

Native tests prove four executing actions plus three waiting callbacks at
saturation, zero refused effects, retirement of all seven accepted callbacks,
no queued action after cancellation, parent-cause preservation, cooperative
deadline refusal, panic identity and worker reuse, blocked completion joining,
and 4096 successive windows with no lifetime quota. The internal-cause fuzzer
uses validated Core causes directly; it asserts one native action/completion
and exact cause/panic preservation. It does not add or certify an external byte
protocol. Source identities, native binary, mutation, stdout/stderr and first
failures remain retained. The broad Core failures are explicitly unresolved;
this checkpoint does not claim a full Primitive release or independent acceptance.
