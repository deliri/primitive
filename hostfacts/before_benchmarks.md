# hostfacts before

readBoundedValue reserved maximum+1 bytes (up to virtualFileMaximumBytes = 1 MiB) for a few-byte cgroup token.

## Measured

sparse capacity 1048577 bytes for payload max\n

This is the filestore defect class: a compiler-owned maximum used as a
heap reservation instead of an admission limit. Primitive must not do the
scan; Go's standard library (or the exact payload) must.
