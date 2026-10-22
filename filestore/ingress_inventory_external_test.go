package filestore_test

import (
	"reflect"
	"testing"

	"github.com/deliri/primitive/v2026/filestore"
)

type filestoreIngressProof struct {
	door   reflect.Value
	fuzz   func(*testing.F)
	proof  func(*testing.T)
	reason string
}

func TestFilestorePublicIngressHasCompilerBoundSemanticProof(t *testing.T) {
	t.Parallel()
	declarations := filestore.FilestoreIngressDeclarationsForTest(t)
	entries := filestoreIngressProofs()
	for _, name := range declarations {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			var found []filestoreIngressProof
			for _, entry := range entries {
				if filestore.FilestoreIngressSymbolForTest(entry.door) == name {
					found = append(found, entry)
				}
			}
			if len(found) != 1 {
				t.Fatalf("%s proof bindings = %d, want exactly one", name, len(found))
			}
			entry := found[0]
			if entry.fuzz == nil && (entry.proof == nil || entry.reason == "") {
				t.Fatalf("%s has no semantic fuzz or explicit capability/projection proof", name)
			}
			if entry.fuzz != nil && entry.proof != nil {
				t.Fatalf("%s has ambiguous proof classification", name)
			}
		})
	}
	for _, entry := range entries {
		symbol := filestore.FilestoreIngressSymbolForTest(entry.door)
		matches := 0
		for _, decl := range declarations {
			if decl == symbol {
				matches++
			}
		}
		if matches != 1 {
			t.Errorf("compiler binding %q resolves to %d public declarations, want one", symbol, matches)
		}
	}
}

func filestoreIngressProofs() []filestoreIngressProof {
	entries := []filestoreIngressProof{
		{door: reflect.ValueOf(filestore.ConfirmTreeDurable), proof: filestore.ConfirmTreeDurableNativeProofForTest, reason: "Synchronizes an already-owned native tree; native empty/batched/link/cancelled/closed-root proof preserves content and native causes."},
		{door: reflect.ValueOf(filestore.CloseFile), fuzz: FuzzNativeFileHandleCustody},
		{door: reflect.ValueOf(filestore.SyncFile), fuzz: FuzzNativeFileHandleCustody},
		{door: reflect.ValueOf(filestore.CloseRoot), proof: TestNativeRootHandleLayerTriad, reason: "Closes an acquired native root; direct native tests prove absent/closed-root refusal and independent child-file ownership."},
		{door: reflect.ValueOf(filestore.WithSortedRecordScope), fuzz: FuzzSortedRecordScopeMatchesIndependentStableRows},
		{door: reflect.ValueOf(filestore.OpenAppendDestination), fuzz: FuzzAppendDestinationExactBytesAndSettlement},
		{door: reflect.ValueOf((*filestore.AppendDestination).WithInspectedReadScope), fuzz: FuzzAppendReadScopeMatchesOriginalNativeObject},
		{door: reflect.ValueOf((*filestore.AppendDestination).WithHeldReadScope), fuzz: FuzzHeldAppendReadMatchesIndependentBytesAfterNameRemoval},
		{door: reflect.ValueOf((*filestore.AppendDestination).Inspect), fuzz: FuzzAppendDestinationExactBytesAndSettlement},
		{door: reflect.ValueOf((*filestore.AppendDestination).ObserveStanding), fuzz: FuzzAppendDestinationExactBytesAndSettlement},
		{door: reflect.ValueOf((*filestore.AppendDestination).Buffered), fuzz: FuzzAppendDestinationExactBytesAndSettlement},
		{door: reflect.ValueOf((*filestore.AppendDestination).DiscardBuffered), fuzz: FuzzAppendDestinationExactBytesAndSettlement},
		{door: reflect.ValueOf((*filestore.AppendDestination).Truncate), fuzz: FuzzAppendDestinationExactBytesAndSettlement},
		{door: reflect.ValueOf((*filestore.AppendDestination).Write), fuzz: FuzzAppendDestinationExactBytesAndSettlement},
		{door: reflect.ValueOf((*filestore.AppendDestination).AcceptedBytes), fuzz: FuzzAppendAcceptanceMatchesGoWriteReturns},
		{door: reflect.ValueOf((*filestore.AppendDestination).Flush), fuzz: FuzzAppendDestinationExactBytesAndSettlement},
		{door: reflect.ValueOf((*filestore.AppendDestination).Sync), fuzz: FuzzAppendDestinationExactBytesAndSettlement},
		{door: reflect.ValueOf(filestore.SyncAppend), fuzz: FuzzAppendSynchronizationReceiptsMatchNativeBytes},
		{door: reflect.ValueOf(filestore.AppendSyncObservation.AcceptedBytes), fuzz: FuzzAppendSynchronizationReceiptsMatchNativeBytes},
		{door: reflect.ValueOf((*filestore.AppendDestination).Close), fuzz: FuzzAppendDestinationExactBytesAndSettlement},
		{door: reflect.ValueOf((*filestore.AppendDestination).Rotate), fuzz: FuzzAppendDestinationExactBytesAndSettlement},
		{door: reflect.ValueOf((*filestore.AppendDestination).RotatePath), fuzz: FuzzAppendRotationObservationMatchesNativeFiles},
		{door: reflect.ValueOf((*filestore.AppendDestination).ObserveAppend), fuzz: FuzzAppendRotationObservationMatchesNativeFiles},
		{door: reflect.ValueOf(filestore.BrowserCommittedMoveModule), proof: TestBrowserCommittedMoveModuleCoreIdentityProjection, reason: "Publishes immutable embedded browser code with compiler-owned Core error identities; takes no external input. Native IndexedDB execution is proved by the browser suite."},
		{door: reflect.ValueOf(filestore.InspectContentIndex), fuzz: FuzzInspectContentIndexExactBinding},
		{door: reflect.ValueOf(filestore.WriteTAR), fuzz: FuzzTARNativeEntryAndOutputAgreement},
		{door: reflect.ValueOf(filestore.ReadTAR), fuzz: FuzzReadTARMatchesNativeFramingAndClosure},
		{door: reflect.ValueOf(filestore.CopyPrefix), fuzz: FuzzCopyPrefixNativeAgreement},
		{door: reflect.ValueOf(filestore.CopyContent), fuzz: FuzzCopyContentExactAgreement},
		{door: reflect.ValueOf(filestore.CompareContent), fuzz: FuzzCompareContentMatchesNativeBytes},
		{door: reflect.ValueOf(filestore.WriteChunk), fuzz: FuzzReadSnapshotAndChunkMatchNativeByteSequence},
		{door: reflect.ValueOf(filestore.ObserveReadSnapshot), fuzz: FuzzReadSnapshotAndChunkMatchNativeByteSequence},
		{door: reflect.ValueOf(filestore.ReadPrefix), fuzz: FuzzPrefixReadConservesWindowAndUnconsumedSource},
		{door: reflect.ValueOf(filestore.TransferContent), fuzz: FuzzCopyContentExactAgreement},
		{door: reflect.ValueOf(filestore.NewContentPartitionSource), fuzz: FuzzCopyPartitionMatchesIndependentByteSlices},
		{door: reflect.ValueOf(filestore.CopyPartition), fuzz: FuzzCopyPartitionMatchesIndependentByteSlices},
		{door: reflect.ValueOf(filestore.ReadContentIndexEntry), fuzz: FuzzContentIndexCanonicalRecord},
		{door: reflect.ValueOf(filestore.WriteContentIndexEntry), fuzz: FuzzContentIndexCanonicalRecord},
		{door: reflect.ValueOf(filestore.SortRecordStream), fuzz: FuzzRecordRangeSortMatchesIndependentStableBytes},
		{door: reflect.ValueOf(filestore.SortRecordIndexStream), fuzz: FuzzRecordRangeSortMatchesIndependentStableBytes},
		{door: reflect.ValueOf(filestore.SortRecordRanges), fuzz: FuzzRecordRangeSortMatchesIndependentStableBytes},
		{door: reflect.ValueOf(filestore.ReadRecordRangeIndex), fuzz: FuzzRecordRangeIndexCanonicalTuple},
		{door: reflect.ValueOf(filestore.ObserveRecordRangeIndexAt), fuzz: FuzzRecordRangeReadObservationMatchesNativeTupleOracle},
		{door: reflect.ValueOf(filestore.ReadRecordRangeIndexAt), fuzz: FuzzRecordRangeLookupMatchesIndependentForwardOracle},
		{door: reflect.ValueOf(filestore.LowerBoundRecordRangeIndex), fuzz: FuzzRecordRangeLookupMatchesIndependentForwardOracle},
		{door: reflect.ValueOf(filestore.WriteRecordRangeIndex), fuzz: FuzzRecordRangeIndexCanonicalTuple},
		{door: reflect.ValueOf(filestore.SortContentIndex), fuzz: FuzzContentSortExactUnion},
		{door: reflect.ValueOf(filestore.SortContentStream), fuzz: FuzzContentSortExactUnion},
		{door: reflect.ValueOf(filestore.ContentIndexConflictError.Error), fuzz: FuzzContentSortExactUnion},
		{door: reflect.ValueOf(filestore.ContentIndexConflictError.Unwrap), fuzz: FuzzContentSortExactUnion},
		{door: reflect.ValueOf(filestore.Write), fuzz: FuzzWriteReadRoundTrip},
		{door: reflect.ValueOf(filestore.Read), fuzz: FuzzWriteReadRoundTrip},
		{door: reflect.ValueOf(filestore.Stage), fuzz: FuzzStageCommitRoundTrip},
		{door: reflect.ValueOf(filestore.Commit), fuzz: FuzzStageCommitRoundTrip},
		{door: reflect.ValueOf(filestore.Recover), fuzz: FuzzWriteRecoveryHandoffSemanticCustody},
		{door: reflect.ValueOf(filestore.Discard), fuzz: FuzzWriteRecoveryHandoffSemanticCustody},
		{door: reflect.ValueOf(filestore.OpenRead), fuzz: FuzzReadUpdateAppendLockNativeHandleCustody},
		{door: reflect.ValueOf(filestore.OpenCustodyRead), fuzz: FuzzOpenCustodyReadExactInodeAndBytes},
		{door: reflect.ValueOf(filestore.OpenUpdate), fuzz: FuzzReadUpdateAppendLockNativeHandleCustody},
		{door: reflect.ValueOf(filestore.OpenAppend), fuzz: FuzzReadUpdateAppendLockNativeHandleCustody},
		{door: reflect.ValueOf(filestore.OpenLockFile), fuzz: FuzzReadUpdateAppendLockNativeHandleCustody},
		{door: reflect.ValueOf(filestore.RotateAppend), fuzz: FuzzAppendRotationNativeOwnership},
		{door: reflect.ValueOf(filestore.OpenStagedRead), fuzz: FuzzStageDestinationNativeWriterCustody},
		{door: reflect.ValueOf(filestore.OpenParent), fuzz: FuzzOpenParentAndRootIdentityNativeCustody},
		{door: reflect.ValueOf(filestore.OpenRoot), fuzz: FuzzOpenParentAndRootIdentityNativeCustody},
		{door: reflect.ValueOf(filestore.ValidateRootIdentity), fuzz: FuzzOpenParentAndRootIdentityNativeCustody},
		{door: reflect.ValueOf(filestore.OpenPipe), fuzz: FuzzPipeNativeCustodyAndBytes},
		{door: reflect.ValueOf(filestore.TransferPipe), fuzz: FuzzPipeTransferPreservesNativeBytes},
		{door: reflect.ValueOf(filestore.EnsureDirectory), fuzz: FuzzEnsureDirectoryNativeNamespaceCustody},
		{door: reflect.ValueOf(filestore.CreateDirectory), fuzz: FuzzCreateDirectoryNativeModeAndExclusivity},
		{door: reflect.ValueOf(filestore.WithCreatedDirectoryScope), fuzz: FuzzCreatedDirectoryScopeNativeOwnership},
		{door: reflect.ValueOf(filestore.OpenCreatedDirectoryDestination), fuzz: FuzzCreatedDirectoryDestinationNativeBytesAndSettlement},
		{door: reflect.ValueOf((*filestore.CreatedDirectoryDestination).Root), fuzz: FuzzCreatedDirectoryDestinationNativeBytesAndSettlement},
		{door: reflect.ValueOf((*filestore.CreatedDirectoryDestination).Location), fuzz: FuzzCreatedDirectoryDestinationNativeBytesAndSettlement},
		{door: reflect.ValueOf(filestore.FinishCreatedDirectoryDestination), fuzz: FuzzCreatedDirectoryDestinationNativeBytesAndSettlement},
		{door: reflect.ValueOf(filestore.AbandonCreatedDirectoryDestination), fuzz: FuzzCreatedDirectoryDestinationNativeBytesAndSettlement},
		{door: reflect.ValueOf(filestore.EnsureScratchDirectory), fuzz: FuzzScratchCreationModesSemanticClosure},
		{door: reflect.ValueOf(filestore.OpenScratch), fuzz: FuzzScratchCreationModesSemanticClosure},
		{door: reflect.ValueOf(filestore.ResetScratch), proof: TestScratchResetDiscardsBytesAndRestoresWriteCoordinate, reason: "Consumes an owned native scratch handle and proves exact zero extent plus restored write offset across transfer boundaries."},
		{door: reflect.ValueOf(filestore.RestoreScratchPrefix), fuzz: FuzzScratchRestoreMatchesNativePrefix},
		{door: reflect.ValueOf(filestore.Rewind), proof: TestRewindRestoresNativeReadCoordinateWithoutChangingBytes, reason: "Borrows a Go seek capability; real native bytes and coordinates prove replay without truncation, with separate nil/cancellation/closed-source refusal and standard-stream semantic fuzz."},
		{door: reflect.ValueOf(filestore.WithScratchScope), proof: TestScratchScopeOwnsNativeLifetime, reason: "Executes a synchronous resource scope and verifies native namespace cleanup, expired root refusal, and preserved consumer cancellation/failure."},
		{door: reflect.ValueOf(filestore.WithRootScope), fuzz: FuzzScopePanicPolicyConservesNativeBytesAndCloseOwnership},
		{door: reflect.ValueOf(filestore.WithInspectedReadScope), fuzz: FuzzInspectedReadScopePreservesHeldObjectBytesAndMetadata},
		{door: reflect.ValueOf(filestore.WithReadScope), fuzz: FuzzScopePanicPolicyConservesNativeBytesAndCloseOwnership},
		{door: reflect.ValueOf(filestore.WithReadAtScope), fuzz: FuzzReadAtScopeMatchesNativeByteRanges},
		{door: reflect.ValueOf(filestore.WithScratchWriterScope), fuzz: FuzzFileScopesConserveNativeBytesAndCloseBorrowedHandles},
		{door: reflect.ValueOf(filestore.WithScratchReplayScope), fuzz: FuzzScratchReplayConservesReplacementBytes},
		{door: reflect.ValueOf(filestore.WithLockFileScope), proof: TestLockFileScopeClosesEveryConsumerOutcome, reason: "Native borrowed handle refuses Stat after success, consumer refusal and panic; no second file lifetime model."},
		{door: reflect.ValueOf(filestore.FileScopeResult.OperationError), fuzz: FuzzFileScopesConserveNativeBytesAndCloseBorrowedHandles},
		{door: reflect.ValueOf(filestore.FileScopeResult.CleanupError), fuzz: FuzzFileScopesConserveNativeBytesAndCloseBorrowedHandles},
		{door: reflect.ValueOf(filestore.Touch), fuzz: FuzzCustodyNamespaceAndTimestampSemanticClosure},
		{door: reflect.ValueOf(filestore.ConfirmDurable), fuzz: FuzzCustodyNamespaceAndTimestampSemanticClosure},
		{door: reflect.ValueOf(filestore.Inspect), fuzz: FuzzInspectionNativeFactsSemanticClosure},
		{door: reflect.ValueOf(filestore.InspectEntry), fuzz: FuzzEntryInspectionNativeFacts},
		{door: reflect.ValueOf(filestore.InspectOpenFile), fuzz: FuzzInspectOpenFilePreservesNativeExtent},
		{door: reflect.ValueOf(filestore.Canonicalize), fuzz: FuzzSymbolicLinkObservationAndResolution},
		{door: reflect.ValueOf(filestore.ReadSymbolicLink), fuzz: FuzzSymbolicLinkObservationAndResolution},
		{door: reflect.ValueOf(filestore.ObserveSharing), fuzz: FuzzSharingNativeObservationAndCustody},
		{door: reflect.ValueOf(filestore.ObserveHeldStanding), fuzz: FuzzHeldStandingNativeIdentityAndCustody},
		{door: reflect.ValueOf(filestore.SetPermissions), fuzz: FuzzPermissionNativeMetadataSemanticClosure},
		{door: reflect.ValueOf(filestore.OpenStageDestination), fuzz: FuzzStageDestinationNativeWriterCustody},
		{door: reflect.ValueOf(filestore.StageProduced), fuzz: FuzzStageProducedPublicationRequiresCompletion},
		{door: reflect.ValueOf(filestore.WriteZIP), fuzz: FuzzZIPNativeEntriesPreserveBytesAndRefusals},
		{door: reflect.ValueOf(filestore.FinishStageDestination), fuzz: FuzzStageDestinationNativeWriterCustody},
		{door: reflect.ValueOf(filestore.AbandonStageDestination), fuzz: FuzzStageDestinationNativeWriterCustody},
		{door: reflect.ValueOf(filestore.Walk), fuzz: FuzzWalkCardinalitySemanticClosure},
		{door: reflect.ValueOf(filestore.Remove), fuzz: FuzzRemovalPolicyConservesRefusedNativeBytes},
		{door: reflect.ValueOf(filestore.RemoveTree), fuzz: FuzzRemovalNativeNamespaceSemanticClosure},
		{door: reflect.ValueOf(filestore.RemoveDirectoryContents), fuzz: FuzzDirectoryContentsRemovalConservesNativeNamespace},
		{door: reflect.ValueOf(filestore.Rename), fuzz: FuzzRenameNativeNamespaceCustody},
		{door: reflect.ValueOf((*filestore.StageDestination).File), fuzz: FuzzStageDestinationNativeWriterCustody},
		{door: reflect.ValueOf(filestore.WriteStage), fuzz: FuzzStageDestinationNativeWriterCustody},
		{door: reflect.ValueOf(filestore.SyncStage), fuzz: FuzzStageDestinationNativeWriterCustody},
		{door: reflect.ValueOf(filestore.StageSyncObservation.BytesWritten), fuzz: FuzzStageDestinationNativeWriterCustody},
		{door: reflect.ValueOf(filestore.StageWriteObservation.BytesWritten), fuzz: FuzzStageDestinationNativeWriterCustody},
		{door: reflect.ValueOf(filestore.RestoreStage), fuzz: FuzzStageRestoreConservesNativePrefixAndCursor},
		{door: reflect.ValueOf(filestore.StageRestoreObservation.BeforeBytes), fuzz: FuzzStageRestoreConservesNativePrefixAndCursor},
		{door: reflect.ValueOf(filestore.StageRestoreObservation.AfterBytes), fuzz: FuzzStageRestoreConservesNativePrefixAndCursor},
		{door: reflect.ValueOf(filestore.StageRestoreObservation.Offset), fuzz: FuzzStageRestoreConservesNativePrefixAndCursor},
		{door: reflect.ValueOf(filestore.StagedFile.Path), fuzz: FuzzStageCommitRoundTrip},
		{door: reflect.ValueOf(filestore.StagedFile.BytesWritten), fuzz: FuzzStageCommitRoundTrip},
	}
	// These accessors consume only already sealed typed facts. Their owning
	// observation/admission boundaries above have semantic external fuzz targets.
	for _, door := range []reflect.Value{
		reflect.ValueOf(filestore.Inspection.Kind), reflect.ValueOf(filestore.Inspection.SizeBytes), reflect.ValueOf(filestore.Inspection.ModifiedAt),
		reflect.ValueOf(filestore.Inspection.Permissions), reflect.ValueOf(filestore.Inspection.Ownership), reflect.ValueOf(filestore.Inspection.Allocation),
		reflect.ValueOf(filestore.Permissions.Bits), reflect.ValueOf(filestore.Permissions.FileMode), reflect.ValueOf(filestore.Permissions.IsSet),
		reflect.ValueOf(filestore.Ownership.IsSet), reflect.ValueOf(filestore.Ownership.UID), reflect.ValueOf(filestore.Ownership.GID),
		reflect.ValueOf(filestore.Allocation.Reported), reflect.ValueOf(filestore.Allocation.Bytes),
	} {
		entries = append(entries, filestoreIngressProof{door: door, proof: TestInspectNativeObservationLayerTriad, reason: "Projects metadata sealed by Inspect; no external representation is decoded here."})
	}
	for _, door := range []reflect.Value{reflect.ValueOf(filestore.ActivationRequest.CommitRequest), reflect.ValueOf(filestore.ActivationRequest.StageDestination)} {
		entries = append(entries, filestoreIngressProof{door: door, proof: TestActivationStageAgreementLayerTriad, reason: "Builds a typed stage/commit request from validated caller intent; execution and native facts are fuzzed at Stage/Commit."})
	}

	for _, door := range []reflect.Value{reflect.ValueOf(filestore.OpenDirectory), reflect.ValueOf((*filestore.HeldDirectory).File), reflect.ValueOf((*filestore.HeldDirectory).Filesystem)} {
		entries = append(entries, filestoreIngressProof{door: door, fuzz: FuzzDirectoryAcquisitionNamespaceCustody})
	}
	entries = append(entries, filestoreIngressProof{door: reflect.ValueOf(filestore.FilesystemIdentity.Uint64), proof: filestore.TestFilesystemIdentityProjectionLayerTriad, reason: "Projects the complete uint64 coordinate of an already sealed native identity; no external representation is admitted."})
	entries = append(entries, filestoreIngressProof{door: reflect.ValueOf((*filestore.HeldDirectory).Close), proof: TestOpenDirectoryLayerTriad, reason: "Releases an owned Go handle; no external representation is admitted."})
	return entries
}
