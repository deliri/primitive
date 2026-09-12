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
		{door: reflect.ValueOf(filestore.ReadContentIndexEntry), fuzz: FuzzContentIndexCanonicalRecord},
		{door: reflect.ValueOf(filestore.WriteContentIndexEntry), fuzz: FuzzContentIndexCanonicalRecord},
		{door: reflect.ValueOf(filestore.SortContentIndex), fuzz: FuzzContentSortExactUnion},
		{door: reflect.ValueOf(filestore.ContentIndexConflictError.Error), fuzz: FuzzContentSortExactUnion},
		{door: reflect.ValueOf(filestore.ContentIndexConflictError.Unwrap), fuzz: FuzzContentSortExactUnion},
		{door: reflect.ValueOf(filestore.Write), fuzz: FuzzWriteReadRoundTrip},
		{door: reflect.ValueOf(filestore.Read), fuzz: FuzzWriteReadRoundTrip},
		{door: reflect.ValueOf(filestore.Stage), fuzz: FuzzStageCommitRoundTrip},
		{door: reflect.ValueOf(filestore.Commit), fuzz: FuzzStageCommitRoundTrip},
		{door: reflect.ValueOf(filestore.Recover), fuzz: FuzzWriteRecoveryHandoffSemanticCustody},
		{door: reflect.ValueOf(filestore.Discard), fuzz: FuzzWriteRecoveryHandoffSemanticCustody},
		{door: reflect.ValueOf(filestore.OpenRead), fuzz: FuzzReadUpdateAppendLockNativeHandleCustody},
		{door: reflect.ValueOf(filestore.OpenUpdate), fuzz: FuzzReadUpdateAppendLockNativeHandleCustody},
		{door: reflect.ValueOf(filestore.OpenAppend), fuzz: FuzzReadUpdateAppendLockNativeHandleCustody},
		{door: reflect.ValueOf(filestore.OpenLockFile), fuzz: FuzzReadUpdateAppendLockNativeHandleCustody},
		{door: reflect.ValueOf(filestore.RotateAppend), fuzz: FuzzAppendRotationNativeOwnership},
		{door: reflect.ValueOf(filestore.OpenStagedRead), fuzz: FuzzStageDestinationNativeWriterCustody},
		{door: reflect.ValueOf(filestore.OpenParent), fuzz: FuzzOpenParentAndRootIdentityNativeCustody},
		{door: reflect.ValueOf(filestore.OpenRoot), fuzz: FuzzOpenParentAndRootIdentityNativeCustody},
		{door: reflect.ValueOf(filestore.ValidateRootIdentity), fuzz: FuzzOpenParentAndRootIdentityNativeCustody},
		{door: reflect.ValueOf(filestore.OpenPipe), fuzz: FuzzPipeNativeCustodyAndBytes},
		{door: reflect.ValueOf(filestore.EnsureDirectory), fuzz: FuzzEnsureDirectoryNativeNamespaceCustody},
		{door: reflect.ValueOf(filestore.EnsureScratchDirectory), fuzz: FuzzScratchCreationModesSemanticClosure},
		{door: reflect.ValueOf(filestore.OpenScratch), fuzz: FuzzScratchCreationModesSemanticClosure},
		{door: reflect.ValueOf(filestore.Touch), fuzz: FuzzCustodyNamespaceAndTimestampSemanticClosure},
		{door: reflect.ValueOf(filestore.ConfirmDurable), fuzz: FuzzCustodyNamespaceAndTimestampSemanticClosure},
		{door: reflect.ValueOf(filestore.Inspect), fuzz: FuzzInspectionNativeFactsSemanticClosure},
		{door: reflect.ValueOf(filestore.Canonicalize), fuzz: FuzzSymbolicLinkObservationAndResolution},
		{door: reflect.ValueOf(filestore.ReadSymbolicLink), fuzz: FuzzSymbolicLinkObservationAndResolution},
		{door: reflect.ValueOf(filestore.ObserveSharing), fuzz: FuzzSharingNativeObservationAndCustody},
		{door: reflect.ValueOf(filestore.ObserveHeldStanding), fuzz: FuzzHeldStandingNativeIdentityAndCustody},
		{door: reflect.ValueOf(filestore.SetPermissions), fuzz: FuzzPermissionNativeMetadataSemanticClosure},
		{door: reflect.ValueOf(filestore.OpenStageDestination), fuzz: FuzzStageDestinationNativeWriterCustody},
		{door: reflect.ValueOf(filestore.FinishStageDestination), fuzz: FuzzStageDestinationNativeWriterCustody},
		{door: reflect.ValueOf(filestore.AbandonStageDestination), fuzz: FuzzStageDestinationNativeWriterCustody},
		{door: reflect.ValueOf(filestore.Walk), fuzz: FuzzWalkCardinalitySemanticClosure},
		{door: reflect.ValueOf(filestore.Remove), fuzz: FuzzRemovalNativeNamespaceSemanticClosure},
		{door: reflect.ValueOf(filestore.RemoveTree), fuzz: FuzzRemovalNativeNamespaceSemanticClosure},
		{door: reflect.ValueOf(filestore.Rename), fuzz: FuzzRenameNativeNamespaceCustody},
		{door: reflect.ValueOf((*filestore.StageDestination).File), fuzz: FuzzStageDestinationNativeWriterCustody},
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
