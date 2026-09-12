# Streaming content index mechanics

Primitive filestore now owns a canonical digest/extent record codec and fixed-memory external sorting over two caller-owned scratch files. Peachfuzz remains responsible for corpus/crasher meaning. The sorter uses 4096-entry runs and 32 KiB I/O buffers; neither constant limits the complete input or output. Two-way disk merges deduplicate identical records, refuse conflicting extents, check merge cardinality, and preflight the sorted stream before emission. Output is provisional until success. Scratch handles remain ordinary caller-owned os.File values.

The codec admits exact SHA-256 bytes and Primitive ByteLength extents. Extents retain the existing signed Go file-size domain; there is no configured stream quota. A zero extent must carry the empty-content digest. Sorting returns no summary on failure. The preflight and emission passes compare their complete input digests and derived summaries. Callers must retain exclusive scratch custody; arbitrary writer wrappers cannot be inspected for hidden aliases.

Local evidence, not independent acceptance, is retained under `/private/tmp/peachfuzz-state-readonly-evidence`. Every listed attempt records the exact base revision, dirty source bytes, command, cache bypass, output digests, emitted test events and exit. Base revision: `5ee81b48b952be4437d069d502388542453c7028`.

| Attempt | Exit | Meaning |
| --- | ---: | --- |
| primitive-content-index-first-proof | 0 | Initial focused record/sort/inventory proof. |
| primitive-content-sort-semantic-proof | 1 | Failed seed construction: unsigned maximum exceeds the existing signed extent domain. |
| primitive-content-sort-signed-extent-proof | 0 | Corrected nominal seed; semantic sorting fuzz budget 3 seconds. |
| primitive-content-merge-record-loss-red | 1 | Deliberate mutation removes merge-length equality; the missing-record test fails. Mutation discarded. |
| primitive-content-record-loss-restored | 0 | Restored merge check and canonical-record fuzz, 3 seconds. |
| primitive-content-sort-package-proof | 1 | Complete package timed out at the harness default 60 seconds; never counted as passed. |
| primitive-content-sort-package-five-minute-backstop | 0 | Same complete package passed with a 5-minute deadlock backstop, about 98 seconds. |
| primitive-content-record-independent-admission | 0 | Independent record-admission oracle plus canonical closure; 3-second fuzz budget. |
| primitive-content-record-benchmark | 0 | Exploratory codec admission benchmark, 3 seconds. It is not acceptance or a comparative performance claim. |

The merge mutation pins complete-record loss, beyond partial-record decoding. The typed-union fuzz oracle pins exact sorted membership, duplicate idempotence, conflicting digest extents, order independence and total-extent overflow. Writer and request refusal tests preserve error identity and zero summaries. Production data-flow and external-ingress inventories name the new contracts.

Peachfuzz has not consumed this version at the time of this owner note. Its capped aggregate index and consumers still require conversion. The final whole-project gate and independent acceptance remain outstanding.

## Held-index inspection follow-through

The consuming product must preflight an existing index without sorting it again
or reopening its path. `InspectContentIndex` therefore binds one held regular
file to an expected index digest and encoded extent, refuses duplicate or
unordered records, and returns only the derived unique count/extent summary.
It leaves the caller's native file open for a rewind. This is byte and record
verification, not a durable receipt or an immutability claim.

`primitive-content-inspection-semantic-proof` passed the owner architecture,
record/sort tests and a three-second inspection fuzz phase. Its independent
oracle uses raw record widths, unsigned network byte order, standard-library
SHA-256, strict digest order and signed aggregate extent.
`primitive-content-inspection-foreign-digest-red` failed after the expected
digest comparison was deliberately replaced with digest-validity alone.
The mutation was discarded; `primitive-content-inspection-binding-restored`
records the restored focused scope. All attempts remain local evidence.
