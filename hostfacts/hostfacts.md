# Hostfacts

## Objective

Hostfacts is Primitive's read-only host observation owner. It turns the facts
the Go standard library and `golang.org/x/sys` expose about the current host
into validated, bounded values that a caller can use without inventing its own
filesystem, runtime, cgroup, terminal, or platform probe.

## Owned observations

- caller-available and total capacity for one held directory;
- block-device rotation when the platform can answer it;
- Go logical CPU count and Go-managed memory pressure;
- physical memory and the effective Linux cgroup memory limit;
- logical regular-file extent below one held root;
- bounded Go runtime OOM-banner evidence;
- terminal attachment and column geometry for one open descriptor;
- hostname and the current platform; and
- standard-library home, configuration, cache, and temporary directory bases.

The result of an unavailable observation is closed and typed. Hostfacts does
not manufacture a plausible value when the platform cannot answer.

## Deliberate non-ownership

Hostfacts does not set runtime limits, choose worker counts, decide resource
floors or percentages, supervise processes, infer process death from banner
bytes, pause work, delete files, select product directories, or render customer
messages. Witness, Bug, Peachfuzz, Kernel, and other callers retain those
product decisions.

## Real substrate

Go's `os`, `runtime`, `runtime/debug`, and real file handles own the portable
observations. Platform leaves use `golang.org/x/sys/unix` or
`golang.org/x/sys/windows` only where the standard library cannot express the
operation. Kernel text enters through fixed ceilings and is decoded immediately
into package-owned facts; no host model, probe registry, scheduler, monitor, or
background lifecycle exists.
