# Primitive 2026.1.504

The Go fuzz metric reader exposes the corpus total from native progress records. Consumers can distinguish seed execution from work on mutated inputs.

Focused verification passed 400 named nodes under the race detector, including a real Go fuzz producer and bounded record grammar. Removing total observation failed the intended leaf. The three-second semantic fuzz run completed its 10-input baseline and reported 19,215 executions. Vet and Staticcheck passed for runprotocol.

Original evidence is retained on Furnace under `/data/evidence/processed/primitive/2026-11-21/native-fuzz-total504`.
