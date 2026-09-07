# secretstore before

Value embedded [PayloadMaximumBytes]byte (64 KiB Google Secret Manager ceiling) on every secret, including an 18-byte password.

## Measured

every NewValue reserved 65536 payload bytes plus state

This is the filestore defect class: a compiler-owned maximum used as a
heap reservation instead of an admission limit. Primitive must not do the
scan; Go's standard library (or the exact payload) must.
