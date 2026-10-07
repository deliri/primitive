// Package filestore composes real os.Root and os.File primitives into rooted,
// streaming durability effects with fixed transfer windows on macOS, Linux,
// and Windows.
//
// Callers own names, schemas, thresholds, retention, accounting, capacity
// policy, cloud custody, and coordination. Filestore owns only the finite
// filesystem effect requested by one validated value.
//
// ResetScratch disposes a regular scratch handle's bytes and restores its
// native write offset. The caller owns scratch lifetime and any failed-work
// decision; Filestore does not retain a second extent or file model.
// SortContentStream owns disposable sort handles and their rooted cleanup,
// returning observed and unique counts without deciding duplicate policy.
// WithScratchScope lends one native private root and owns its cleanup after
// the caller's synchronous operation; filenames and schemas remain caller policy.
//
// StageDestination lends a real Go file to an external streaming producer.
// The caller checks the producer's error and abandons the stage on failure.
// Only successful production proceeds to FinishStageDestination and Commit;
// InstallReplace publishes the completed stage over the prior file. Finishing
// an unknown-length stage observes its native extent; it cannot determine
// whether the producer intended to send more bytes. That success decision
// remains with the producer's typed result and the caller consuming it.
package filestore
