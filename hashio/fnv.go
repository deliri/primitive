package hashio

import "hash/fnv"

// FNV1a64TextParts hashes the exact concatenation of borrowed text parts using
// Go's FNV-1a engine. Part boundaries add no bytes. The caller owns framing;
// Primitive retains only Go's fixed-size hash state and never joins the text.
func FNV1a64TextParts(parts ...string) uint64 {
	digest := fnv.New64a()
	for _, part := range parts {
		_, _ = digest.Write([]byte(part))
	}
	return digest.Sum64()
}
