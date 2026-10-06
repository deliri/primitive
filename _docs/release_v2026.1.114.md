# Primitive v2026.1.114 — typed time text and display mechanics

Temporal owns canonical UTC parsing under a caller-selected Go `TimeLayout`,
and formatting a typed instant in an explicit `LocationName`. Unknown or
ambient locations fail with the Temporal contract identity. Go owns calendar
syntax and timezone data; Primitive does not retain a timezone model or impose
an arbitrary text quota. Product filenames and display layouts remain caller
policy. Input and projection requests are classified in the compiler-visible
inventory, and both public text boundaries have differential semantic fuzzers.

Duration truncation uses the existing closed `Precision` domain; duration text
uses Go's admitted representation. Neither operation reads a clock.

Development evidence is retained under `_docs/work/2026-10-06/time-text/`.
The Temporal, Compass and Version race run passed 984 tests without skips.
Repository-wide errcheck and staticcheck passed. Alignment repaired source and
test field order. Mutation and fuzz results remain separate execution facts.
These are development observations, not independent acceptance or a claim
that the full Primitive repository release gate ran.
