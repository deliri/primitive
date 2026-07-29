// Package contextstate provides bounded observation of context terminal state.
//
// Context.Err is the sole standard-library terminal-state truth used here.
// Contextstate does not inspect Deadline, Done, Value, Cause, or the wall
// clock. The context contract requires Err to return immediately, so the
// package calls it directly rather than moving observation into a goroutine
// that could leak when given a broken implementation.
//
// Contextstate does not own causal wall-timeout aggregation, context detachment
// composition, process termination, or pipeline termination precedence. Those
// policies remain with the packages that own them.
package contextstate
