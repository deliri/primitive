// Package hostfacts observes bounded, read-only facts about the current host.
//
// Hostfacts reports caller-available disk capacity, the rotational class of
// the block device backing a directory, the Go runtime's logical CPU count,
// total physical memory,
// Go-runtime-managed memory, the effective Linux cgroup memory ceiling,
// logical regular-file tree extent, the presence of canonical Go runtime
// out-of-memory banners, the column geometry of a terminal attached to an
// open descriptor, the platform's own name for the host, and the platform's
// per-user home, configuration, cache, and temporary bases. It never
// changes limits, monitors resources, removes files, supervises processes, or
// chooses the action a caller takes from an observation.
package hostfacts
