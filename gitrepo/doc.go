// Package gitrepo exposes narrow, typed observations of a local Git
// repository. Callers own the meaning of the admitted source set;
// gitrepo owns command resolution, deterministic Git configuration,
// streaming decode, cancellation, and exact mechanical results.
// ObserveStatus drains native porcelain output and returns process byte counts;
// it retains no output body or path inventory and imposes no output quota.
// This describes the Primitive process stream, not Git's internal memory use.
package gitrepo
