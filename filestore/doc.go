// Package filestore composes real os.Root and os.File primitives into rooted,
// streaming filesystem effects with fixed transfer windows. Native durability
// proof covers macOS and Linux. Windows namespace durability is unverified;
// this package does not claim a Windows durable-operation guarantee.
//
// Callers own names, schemas, thresholds, retention, accounting, capacity
// policy, cloud custody, and coordination. Filestore owns only the finite
// filesystem effect requested by one validated value.
//
// WithAppendSpoolScope exclusively creates a working file on the caller-selected
// filesystem. Independent reader callbacks follow the same held inode and wait
// at temporary EOF until explicit Seal. Seal records producer success or abort;
// it does not acknowledge a cloud or secondary destination. Callers own and join
// their tasks before Use returns. Scope cleanup cancels and joins accepted reader
// callbacks before closing the original descriptor, and never deletes the file.
//
// WithFilesystemDurability holds a directory descriptor across synchronous
// caller work, then asks Linux syncfs to flush that descriptor's whole
// filesystem. It accepts opaque symlink entries without inspecting targets;
// ConfirmTreeDurable retains its stricter regular-file/directory-only contract.
// Callback failure or terminal context skips writeback. This scope is Linux
// amd64/arm64 only and acknowledges the OS, not hardware immunity. On Linux
// 5.8+ the held descriptor also observes its filesystem writeback-error window.
// Callers still own error custody on their original writing descriptors and
// must join every writer before returning from the callback.
//
// ResetScratch disposes a regular scratch handle's bytes and restores its
// native write offset. The caller owns scratch lifetime and any failed-work
// decision; Filestore does not retain a second extent or file model.
// SortContentStream owns disposable sort handles and their rooted cleanup,
// returning observed and unique counts without deciding duplicate policy.
// WithScratchScope lends one native private root and owns its cleanup after
// the caller's synchronous operation; filenames and schemas remain caller policy.
// Relay joins both directions of borrowed streams. RelayCurrentProcess instead
// owns a dedicated relay process: Far EOF terminates that process even when its
// client's stdin remains blocking until child exit. It accepts only trusted
// inherited-standard-stream policy and returns no fabricated join receipt.
//
// StageDestination lends a real Go file to an external streaming producer.
// The caller checks the producer's error and abandons the stage on failure.
// Only successful production proceeds to FinishStageDestination and Commit;
// InstallReplace publishes the completed stage over the prior file. Finishing
// an unknown-length stage observes its native extent; it cannot determine
// whether the producer intended to send more bytes. That success decision
// remains with the producer's typed result and the caller consuming it.
// StageProduced owns this complete synchronous lifetime for a producer that
// lends no handle beyond its callback. A refusal yields no staged capability;
// the caller publishes a successful stage through the existing Commit contract.
package filestore
