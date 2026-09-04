// Package gitrepo exposes narrow, typed observations of a local Git
// repository. Callers own the meaning of the admitted source set;
// gitrepo owns command resolution, deterministic Git configuration,
// streaming decode, cancellation, and exact mechanical results.
package gitrepo
