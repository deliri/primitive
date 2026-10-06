// Package hostfacts owns bounded host observations and narrow ambient process effects.
//
// Hostfacts reports caller-available disk capacity, the rotational class of
// the block device backing a directory, the Go runtime's logical CPU count,
// total physical memory,
// Go-runtime-managed memory, the effective Linux cgroup memory ceiling,
// the presence of canonical Go runtime
// out-of-memory banners, the column geometry of a terminal attached to an
// open descriptor, the platform's own name for the host, and the platform's
// per-user home, configuration, cache, and temporary bases. It never
// changes resource limits, monitors resources, removes files, supervises processes, or
// chooses the action a caller takes from an observation.
// SetAmbientEnvironment and RemoveAmbientEnvironment execute one typed
// process environment change. Callers own coordination, binding lifetime, and
// whether an environment change is permitted; no environment snapshot or
// restoration model is retained.
// ObserveTimeZone admits an explicit bounded IANA name and uses Go's timezone
// database loader. Calendar interpretation remains caller policy; no ambient
// Local fallback, product cache or alternate timezone database is introduced.
package hostfacts
