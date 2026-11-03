# Prospective systemd scope memory

`ObserveWorkloadMemoryScopeParent(ctx)` observes the actual memory ancestry
of the default parent used by `systemd-run --user --scope` without a slice
override. This is the placement used by Primitive's capped process command.
It queries the user manager's `app.slice` ControlGroup, resolves that record
against the visible memory-controller mount, and reuses the native paired
limit/current ancestor fold. It does not substitute the calling SSH session.

The query and fold share the existing three-second host-summary bound and
caller cancellation. The current membership and queried ControlGroup are
rechecked. A changed target, malformed record, missing finite charge, native
query refusal or cancellation cannot produce a partial usable capacity fact.
An absent target directory or namespace/control interface is unavailable,
not unlimited. Unlimited requires actual unlimited interfaces. Other operating
systems report Unsupported.

The result is an observation, not a memory reservation or a promise about
future capacity. Physical host memory remains a different fact. Admission
must reobserve, preserve the whole budgets promised to other jobs, and keep
its reserve. Process RSS is not the parent cgroup's native charged current.
The call creates no jobs, changes no systemd properties, and signals no scope.

Owned filesystem tests cover a future branch tighter than the calling session,
a tighter common ancestor, exhaustion, v1/v2 charge names, unknown/missing
parents, malformed command output and terminal contexts. The native Linux
check queries the real user manager without creating a new scope. These do
not claim execution fit for an arbitrary package or every systemd platform.
