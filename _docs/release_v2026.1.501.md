# Primitive 2026.1.501

Attestation signing and verification stream canonical bodies through Core’s existing digest writer without a one-MiB transfer quota. Core supplies the exact byte count and enforces native signed extent; empty bodies, callback failures, native writer refusals and late writes remain failures. Ed25519, the signed frame, domain separation, trust selection and digest binding are unchanged.

Canonical envelope encoding now derives its finite maximum from the native body-length domain. Lease’s finite document bound retains its own smaller decision-body width. Buffered canonical objects remain caller-owned whole values; their extent checks protect native buffer and member-coordinate representation, rather than limiting streamed signing bodies.

The original Blink Kernel run retained 100,465 test passes, 528 failure records and 44 skips without an OOM kill, then failed final machine-attestation publication at the former quota. That incomplete bundle is preserved. This source repair does not convert its subject failures into passes or claim a completed profile.

Focused proof includes the original quota failures, failed-only correction, 270 affected passing test nodes, a native digest-writer triad, exact envelope and lease bounds, and five three-second semantic fuzz campaigns. Final affected formatting, Vet, Staticcheck and Gosec checks passed. Generated 1 MiB and 17 MiB streaming-sign benchmarks each measured 18 allocations and about 1 KiB allocated per operation; this measures signing, not whole-project memory or latency. The native-maximum extent and multi-gigabyte buffered-object boundary were not transferred or allocated in these checks. Independent acceptance remains with the user.
