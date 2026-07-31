// Package receipt owns authenticated accepted-evidence facts and one
// fixed-size monotonic watermark for later Controlstate composition.
//
// Receipt does not own transport, persistence, retries, provider behavior,
// payment policy, pagination, scheduling, or customer-facing rendering.
//
// Three contracts hold on every path. Every operation returns its zero result
// with every error, so a caller can never hold an authenticated proof or a
// selected watermark alongside a failure. Every sealed result revalidates the
// state it closes over before answering, and answers with an error rather than
// a silent zero. Both typed rejections are sealed interfaces backed only by
// package-private values: a ScopeMismatch always names the authenticated fact
// that differed, and a WatermarkConflict always names the invariant that
// refused the advance, so no caller-built carrier can claim either identity
// without a reason.
//
// Canonical member order is owned by pointer-only wire structures, never by
// the declaration order of the exported types. Signed bytes therefore stay
// fixed while memory layout stays free to change.
package receipt
