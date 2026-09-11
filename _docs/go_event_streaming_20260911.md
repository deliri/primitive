# Go event and OOM streams

This change exposes the existing cmd/go decoder through ReadGoEventStream and
removes the OOM scanner's one-MiB total-input ceiling. Primitive owns framing,
closed enums, exact bytes and source/callback error identity. Consumers own the
meaning of events and whether a run is complete.

ReadGoEventStream holds one fixed reader buffer and the existing scalar decoder.
It emits borrowed GoEventStringFragment values and validated GoEventFrame values.
String fragments are provisional until their event closes. Earlier complete
frames do not certify a stream whose later read fails. No package map, event
history, complete diagnostic string, or line/count quota is retained by this API.
The aggregate GoTestObservationCompiler keeps its existing accounting behavior
and uses the same decoder and exported action/output/field enums.

ClassifyGoOOMBanner uses a fixed buffer with a banner-sized overlap and examines
the declared ByteLength without a total quota. Late banners, split banners, final
read failures, joined EOF failures and cancellation retain their exact contract.

## Local evidence

The base revision is 54c2d09ea63b273a59ed6d3a7b73ff59cbf5cce8. Local attempts retain
dirty source bytes, toolchain, argv, cache bypass, exit status and artifact hashes
under /private/tmp/peachfuzz-state-readonly-evidence. These are development facts,
not independent acceptance. The full repository gate is deferred to the end of
the authorized conversion; no gate result or independent receipt is claimed here.

The complete runnercontrol package passed in primitive-go-event-owner-package.
The long-string test checks all 67,108,865 decoded bytes, exact field closure and
one event. The deliberate discard-fragments mutation failed that test and was
restored. Source/callback refusals, cancellation, duplicate and unknown actions,
all action and output-kind arms, and UTF-8/escape fragment boundaries are covered.
The external-ingress inventory now includes free Read/Parse/Decode/Load/Replay
functions and binds ReadGoEventStream to its semantic fuzz target.

The first semantic fuzz attempt found an overly strict oracle for neutral
whitespace, not a production defect. That failure remains retained; the corrected
three-second run passed. The 64-KiB benchmark ran with -benchtime=3s and reported
about 114 MB/s on this machine. That is not a baseline/candidate improvement claim.

Peachfuzz must consume the published release before deleting its remaining
Go-event scanners. This change does not implement an OGS service or decide fuzz
outcomes, payment policy, evidence acceptance, or customer completion.

The release-package attempt also found four existing packages missing from the
authored source claims: Compass, Tailnet, Tailnetconfig and Version. Their claims
now describe their existing implementations; the unchanged claim-coverage test
passes. No Tailnet integration was added to Peachfuzz, whose CLI dependency graph
contains neither Tailnet nor Tailscale. The failed attempt remains recorded as a
failure. A second semantic fuzz target checks every decoded payload byte across
bounded oracle chunks while admitting the entire fuzz input; its three-second
run passed. The module-wide production build also completed successfully.

## Native metadata correction after v2026.1.73

Inspection of the installed Go 1.27.1 test2json source exposed two missing actions:
attr and artifacts, with Key, Value and Path string fields. The published
v2026.1.73 tag remains immutable. The correction adds closed action/field enum arms
and routes their bytes through the same fixed-buffer decoder. It neither opens
artifact paths nor assigns meaning to attributes.

Both native producer tests failed against revision
8f3e57046be6bbfe20c0fe9ff270c94df6c5b3ab in primitive-native-metadata-red.
The corrected decoder passed primitive-native-metadata-green. The tests execute
real Go tests using Attr and ArtifactDir, check exact attribute values and verify
that the emitted artifact path names the directory Go actually created under the
test-owned root. Metadata semantic fuzzing consumes all input in bounded oracle
chunks and checks every decoded byte and field closure. Its first attempt exposed
a fixture error: JSON v2 omitempty omitted explicitly empty string pointers.
Changing the typed fixture to omitzero preserves that distinction; the subsequent
three-second attempt primitive-metadata-semantic-fuzz-explicit-empty passed.
Both attempts remain retained. These are local execution facts, not independent
acceptance, and do not close the full repository gate.
