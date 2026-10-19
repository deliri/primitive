# Primitive

Primitive is the product-neutral Go primitive layer used to build reliable
command-line tools and services.

Module: `github.com/deliri/primitive/v2026`

First release: `v2026.0.0`

Primitive is a clean rebuild with no compatibility obligation to the archived
implementation. It makes Go's standard library, operating-system primitives,
documented protocols, and official SDKs typed, validated, bounded, streaming,
and easier to compose. The real substrate still does the work.

- [LEDGER.md](LEDGER.md) owns project state.
- [\_docs/testing_protocol.md](_docs/testing_protocol.md) owns test doctrine.
- [\_docs/interviews/](_docs/interviews/) preserves the recon evidence.

## Live project truth

Run `forge project info refresh` from the repository root. Forge inspects the
current Go tree with compiler-backed source analysis, refreshes each package's
`standard_test.go` functions, and emits the bounded project projection
under `.forge/`.

Primitive owns package identities, effect ownership, schemas, and validation.
It does not maintain a second handwritten list of current files, imports,
declarations, benchmarks, fuzz targets, or effect sites. Those are observations
and remain "not observed" until Forge produces them.

## Host resource observations

`hostfacts.ObserveResourceUsage(ctx, hostfacts.ResourceUsageRequest{IntervalSeconds: 1})`
returns one interval CPU observation, currently available physical memory, and
Linux pressure facts. Darwin uses the native `top` host summary and `vm_stat`;
Linux uses the kernel's `/proc` interfaces. Disk capacity remains `AssessDisk`.
The caller owns utilization ceilings, memory reserves, reservations, storage
placement and the admission decision. Observation does not change host limits
or start a monitoring service. Unsupported systems and failed/cancelled samples
return no resource sample.

On Linux, `ResourceUsageRequest.Scopes` names systemd scopes whose cgroup v2
`cpu.stat` `usage_usec` is read strictly inside the two `/proc/stat` samples.
Each scope's delta is divided by the monotonic window bracketing both host
samples and rounded down, so `ScopeCPU.BusyCPUMilli` never exceeds that scope's
use of the host window. The kernel keeps CPU time in the cgroup's counters, so
exited processes remain counted. A scope absent at either read reports zero.
Repeated scopes are refused. Darwin has no cgroup CPU counters and refuses a
request with scopes. Memory is not split per scope: cgroup `memory.current`
includes page cache charged to whichever cgroup first touched it.

`hostfacts.ObserveSystemdScope` observes one named native Linux transient scope
without signalling it. A loaded scope remains conservatively alive until the
manager collects it, including descendants that changed POSIX process groups.
Unsupported systems and failed observations return no liveness fact.

## Native process configuration

`process.WriteServiceDocument` encodes a typed user-service definition as
launchd XML or a systemd unit. The caller selects paths, argv, environment,
restart policy and timeouts, then uses Filestore and Process to install it.
`process.QuotePOSIXArgument` preserves one exact argument for OpenSSH's
remote-shell command field. Neither operation registers or starts a service.

## License

Primitive is licensed under the [Mozilla Public License 2.0](LICENSE).

Copyright 2026 Deliri Software Inc., operating as Off Grid Software.
