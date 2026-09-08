# Exchange final benchmark evidence

After review, the user authorized deleting the retained binaries. Their hashes
and prior profile analyses remain in the records; CPU/memory profiles stay local.
See the [disposal receipt](../evidence/release-v2026.1.19/binary-cleanup.json).

Go 1.27.1, darwin/arm64, Apple M1 Max. Each of the 38 leaves ran separately with `-benchtime=30s -count=1 -benchmem`, plus CPU and memory profiles and a retained binary from that exact invocation. Serial leaves used `-cpu=1`; the two explicitly parallel leaves used `-cpu=8`. Leaves 01–37 completed on the same source. Leaf 38 then required a benchmark-fixture correction for native port exhaustion; production and the earlier benchmark bodies did not change. The failed sample and corrected source are retained. Source hashes and artifact hashes are recorded per invocation; raw artifacts remain local and are excluded from the proposed remote backup.

These are absolute candidate samples. They do not establish a package-wide speedup or substitute for independent review. The run plan and complete CPU/alloc-space summaries are in [the analysis report](testdata/test-upgrade-20260907/post-checkpoint-final-benchmark-analysis.json).

| Workload | Iterations | ns/op | B/op | allocs/op | Evidence |
| --- | ---: | ---: | ---: | ---: | --- |
| ResponseBufferRelease/128B | 85054572 | 428.4 | 672 | 6 | [receipt](testdata/test-upgrade-20260907/post-checkpoint-final-benchmark-01.json) |
| ResponseBufferRelease/4KiB | 30666879 | 1183 | 4640 | 6 | [receipt](testdata/test-upgrade-20260907/post-checkpoint-final-benchmark-02.json) |
| ResponseBufferRelease/64KiB | 3377016 | 10812 | 66080 | 6 | [receipt](testdata/test-upgrade-20260907/post-checkpoint-final-benchmark-03.json) |
| ServerJSONBoundary/128B | 4239070 | 8476 | 12866 | 96 | [receipt](testdata/test-upgrade-20260907/post-checkpoint-final-benchmark-04.json) |
| ServerJSONBoundary/1KiB | 2320178 | 15504 | 22196 | 104 | [receipt](testdata/test-upgrade-20260907/post-checkpoint-final-benchmark-05.json) |
| ServerJSONBoundary/8KiB | 485974 | 75009 | 148316 | 113 | [receipt](testdata/test-upgrade-20260907/post-checkpoint-final-benchmark-06.json) |
| ServerJSONBoundaryByLimit/limit1KiB | 4404194 | 8155 | 9921 | 96 | [receipt](testdata/test-upgrade-20260907/post-checkpoint-final-benchmark-07.json) |
| ServerJSONBoundaryByLimit/limit64KiB | 4242792 | 8496 | 12866 | 96 | [receipt](testdata/test-upgrade-20260907/post-checkpoint-final-benchmark-08.json) |
| ServerJSONBoundaryByLimit/limit1MiB | 4238485 | 8476 | 12866 | 96 | [receipt](testdata/test-upgrade-20260907/post-checkpoint-final-benchmark-09.json) |
| BoundedReceiveByDeclaredExtent/declared_extent | 329839 | 108078 | 1055079 | 48 | [receipt](testdata/test-upgrade-20260907/post-checkpoint-final-benchmark-10.json) |
| BoundedReceiveByDeclaredExtent/undeclared_extent | 321376 | 108672 | 1055080 | 48 | [receipt](testdata/test-upgrade-20260907/post-checkpoint-final-benchmark-11.json) |
| Upload10MiBFileOverLoopback | 10000 | 3267403 | 42263 | 132 | [receipt](testdata/test-upgrade-20260907/post-checkpoint-final-benchmark-12.json) |
| Download10MiBFileOverLoopback | 10000 | 3200676 | 42220 | 143 | [receipt](testdata/test-upgrade-20260907/post-checkpoint-final-benchmark-13.json) |
| ResolveClientAddress/direct_peer_ignores_header | 195914950 | 183.1 | 0 | 0 | [receipt](testdata/test-upgrade-20260907/post-checkpoint-final-benchmark-14.json) |
| ResolveClientAddress/trusted_proxy_chain | 78037275 | 464.2 | 32 | 2 | [receipt](testdata/test-upgrade-20260907/post-checkpoint-final-benchmark-15.json) |
| ResolveClientAddress/untrusted_peer_ignores_chain | 121704070 | 295.7 | 0 | 0 | [receipt](testdata/test-upgrade-20260907/post-checkpoint-final-benchmark-16.json) |
| ResolveClientAddress/google_cloud_final_pair | 122861793 | 292.9 | 32 | 2 | [receipt](testdata/test-upgrade-20260907/post-checkpoint-final-benchmark-17.json) |
| AggregateCompletedAttemptHandoff/binary_complete | 15625472 | 2295 | 2678 | 32 | [receipt](testdata/test-upgrade-20260907/post-checkpoint-final-benchmark-18.json) |
| AggregateCompletedAttemptHandoff/cancel_after_native_close | 13946605 | 2607 | 2886 | 40 | [receipt](testdata/test-upgrade-20260907/post-checkpoint-final-benchmark-19.json) |
| AggregateHeaderValueAdmission/at_limit | 7649838 | 4720 | 1721 | 71 | [receipt](testdata/test-upgrade-20260907/post-checkpoint-final-benchmark-20.json) |
| AggregateHeaderValueAdmission/above_limit | 238839908 | 151.1 | 144 | 5 | [receipt](testdata/test-upgrade-20260907/post-checkpoint-final-benchmark-21.json) |
| AggregateHeaderValueAdmission/excess_4096 | 236524135 | 152.4 | 144 | 5 | [receipt](testdata/test-upgrade-20260907/post-checkpoint-final-benchmark-22.json) |
| ReplayStreamDownloadHandoff/binary_success | 8893316 | 4064 | 4184 | 46 | [receipt](testdata/test-upgrade-20260907/post-checkpoint-final-benchmark-23.json) |
| ReplayStreamDownloadHandoff/cancelled_native_read_failure | 7547623 | 4776 | 4392 | 54 | [receipt](testdata/test-upgrade-20260907/post-checkpoint-final-benchmark-24.json) |
| ServerRuntimeBoundary/dormant_construction | 152692352 | 236.4 | 560 | 4 | [receipt](testdata/test-upgrade-20260907/post-checkpoint-final-benchmark-25.json) |
| ServerRuntimeBoundary/os_allocated_listener_acquire_close | 3231613 | 11100 | 544 | 16 | [receipt](testdata/test-upgrade-20260907/post-checkpoint-final-benchmark-26.json) |
| ListenAddressAdmission/concrete_ipv4 | 456466648 | 79.00 | 16 | 1 | [receipt](testdata/test-upgrade-20260907/post-checkpoint-final-benchmark-27.json) |
| ListenAddressAdmission/concrete_mapped_ipv4 | 355580390 | 101.3 | 24 | 1 | [receipt](testdata/test-upgrade-20260907/post-checkpoint-final-benchmark-28.json) |
| ListenAddressAdmission/mapped_wildcard | 533316394 | 67.78 | 0 | 0 | [receipt](testdata/test-upgrade-20260907/post-checkpoint-final-benchmark-29.json) |
| ListenAddressAdmission/zoned_wildcard | 474849439 | 76.02 | 0 | 0 | [receipt](testdata/test-upgrade-20260907/post-checkpoint-final-benchmark-30.json) |
| ListenAddressAdmission/zoned_mapped_wildcard | 353062930 | 102.1 | 0 | 0 | [receipt](testdata/test-upgrade-20260907/post-checkpoint-final-benchmark-31.json) |
| ParseIdempotencyKeyMaximum | 266944306 | 134.9 | 0 | 0 | [receipt](testdata/test-upgrade-20260907/post-checkpoint-final-benchmark-32.json) |
| ReceiveBasicAuthorizationCustody/maximum_credentials | 5954239 | 6051 | 1296 | 3 | [receipt](testdata/test-upgrade-20260907/post-checkpoint-final-benchmark-33.json) |
| ReceiveBasicAuthorizationCustody/partial_base64_refusal | 11110950 | 3231 | 80 | 4 | [receipt](testdata/test-upgrade-20260907/post-checkpoint-final-benchmark-34.json) |
| SocketVerifiedClientCertificateDigest | 172406488 | 208.7 | 0 | 0 | [receipt](testdata/test-upgrade-20260907/post-checkpoint-final-benchmark-35.json) |
| RequestConstructionControl | 18703876 | 1932 | 5656 | 18 | [receipt](testdata/test-upgrade-20260907/post-checkpoint-final-benchmark-36.json) |
| ServerJSONBoundaryParallel | 4287241 | 8073 | 22290 | 104 | [receipt](testdata/test-upgrade-20260907/post-checkpoint-final-benchmark-37.json) |
| JSONRoundTripOverLoopbackParallel | 983316 | 32308 | 42247 | 272 | [receipt](testdata/test-upgrade-20260907/post-checkpoint-final-benchmark-38-reuse-corrected.json) |

## Comparisons supported by retained evidence

| Workload | Before ns/op; B/op; allocations | Final ns/op; B/op; allocations |
| --- | --- | --- |
| JSON 128 B | 12,317; 45,512; 97 | 8,476; 12,866; 96 |
| JSON 1 KiB | 19,009; 53,834; 105 | 15,504; 22,196; 104 |
| JSON 8 KiB control | 74,910; 148,317; 113 | 75,009; 148,316; 113 |
| Buffer release 128 B, before/after panic containment | 428.8; 672; 6 | 428.4; 672; 6 |

The reverse-order 128-byte JSON comparison was candidate 8,480 ns/op versus reference 12,325 ns/op, with the same respective allocation counts. The improvement comes from Go `io.Copy` sizing its first scratch buffer from a bounded `io.LimitedReader`, instead of reserving the full generic buffer for a tiny declared body. The declaration remains untrusted: continuation and overflow checks still apply. The before allocation profile attributed 71.92% of volume to `io.copyBuffer`. The compared candidate CPU profile was 91.29% `runtime.kevent`; it does not support application CPU attribution.

Maximum Basic credentials previously allocated 3,856 bytes in seven allocations; the ownership repair uses 1,296 bytes in three allocations. The original sample was 5,373 ns/op and the pre-final candidate was 6,084 ns/op. This is an allocation improvement, not a latency improvement. The complete final measurement above is retained even when slower. Maximum-key parsing likewise has no claimed latency improvement over its original 135.1 ns/op sample.

The declared and undeclared 512 KiB receive workloads aggregate bounded payloads, allocating about 1.06 MB per operation. They are not constant-memory streaming measurements. The 10 MiB upload/download workloads exercise real loopback HTTP and check exact bytes and observations; they are local-host samples, not external-network throughput promises.

Every profile summary is retained, including runtime/poller dominated samples. Setup allocations visible in allocation profiles are not automatically per-operation allocations; the benchmark measurement provides that denominator. Broad gates, race checks and linters remain deferred by user instruction.

The corrected parallel loopback run completed 983,316 requests over eight
accepted TCP connections (0.0000081 connections/op). Its original attempt
exhausted local ports with Go's default two idle connections per host. The
corrected fixture sets Go's active/idle ceilings to its worker count and checks
accepted connections. Disabling keep-alives failed that oracle with 100
connections in 100 requests; that deliberately short mutation is not benchmark
timing evidence. The correction changes the workload's connection policy and
supports no before/after production latency claim.
