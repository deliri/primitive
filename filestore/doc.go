// Package filestore composes real os.Root and os.File primitives into rooted,
// streaming filesystem effects with fixed transfer windows. Native durability
// proof covers macOS and Linux. Windows namespace durability is unverified;
// this package does not claim a Windows durable-operation guarantee.
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
