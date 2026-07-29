// Package filestore composes real os.Root and os.File primitives into rooted,
// bounded, streaming durability effects.
//
// Callers own names, schemas, thresholds, retention, accounting, capacity
// policy, cloud custody, and coordination. Filestore owns only the finite
// filesystem effect requested by one validated value.
package filestore
