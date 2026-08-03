// Package hostfacts observes bounded, read-only facts about the current host.
//
// Hostfacts reports caller-available disk capacity, total physical memory,
// Go-runtime-managed memory, the effective Linux cgroup memory ceiling,
// logical regular-file tree extent, and the presence of canonical Go runtime
// out-of-memory banners. It never
// changes limits, monitors resources, removes files, supervises processes, or
// chooses the action a caller takes from an observation.
package hostfacts
