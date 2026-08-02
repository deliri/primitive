package upgrade

import (
	"context"
	"errors"
	"math/bits"
	"os"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/release"
)

const downloadProviderDiagnostic = "upgrade provider cannot download whole objects"

// DownloadSource is one already-issued Objectstore download capability.
type DownloadSource struct {
	Client   objectstore.Client
	Target   objectstore.DownloadTarget
	Policy   objectstore.Policy
	Provider objectstore.Provider
}

func (s DownloadSource) Validate() error {
	if err := s.Client.Validate(); err != nil {
		return contractError(err)
	}
	if s.Provider != objectstore.ProviderAmazonS3 &&
		s.Provider != objectstore.ProviderGoogleCloudStorage {
		return contractError(errors.New(downloadProviderDiagnostic))
	}
	if err := s.Target.Validate(); err != nil {
		return contractError(err)
	}
	if err := s.Policy.Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

// StagePolicy is the caller-owned free-space reserve left after the complete
// candidate extent is admitted.
type StagePolicy struct {
	FreeSpaceReserve core.ByteLength
}

func (p StagePolicy) Validate() error {
	if err := p.FreeSpaceReserve.Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

// StageRequest binds one authenticated Release handoff to one exact download.
type StageRequest struct {
	Root      *os.Root
	Directory core.AbsolutePath
	Source    DownloadSource
	Prepared  release.PreparedRelease
	Policy    StagePolicy
}

func (r StageRequest) Validate() error {
	if err := validateRoot(r.Root, r.Directory); err != nil {
		return err
	}
	if err := r.Prepared.Validate(); err != nil {
		return contractError(err)
	}
	if err := r.Source.Validate(); err != nil {
		return err
	}
	return r.Policy.Validate()
}

// Stage downloads directly into the fixed unselected slot, verifies the exact
// artifact, and returns the only path a product may trial. It never changes the
// primary selection.
//
// Staging is idempotent for one exact candidate. A durable trial receipt binds
// the fixed slot to the prior selector and candidate before bytes arrive. An
// interrupted attempt is resumed, while a different live candidate conflicts
// without losing the earlier trial.
//
// One installation directory admits one writer. Upgrade owns no lock: two
// concurrent Stage, Promote, or DiscardTrial calls against the same directory
// are a caller error that the typed authority conflict reports but cannot
// prevent.
func Stage(ctx context.Context, request StageRequest) (TrialTarget, error) {
	if err := request.Validate(); err != nil {
		return TrialTarget{}, err
	}
	candidate, prior, err := stageAuthority(ctx, request)
	if err != nil {
		return TrialTarget{}, err
	}
	target, err := newTrialTarget(request.Directory, prior, candidate)
	if err != nil {
		return TrialTarget{}, err
	}
	if err := installCandidate(ctx, request, target); err != nil {
		return TrialTarget{}, err
	}
	return target, nil
}

func installCandidate(
	ctx context.Context,
	request StageRequest,
	target TrialTarget,
) error {
	build := target.candidate.Build()
	if err := prepareCandidateSlot(ctx, request.Root, target); err != nil {
		return newAttemptError(
			FailurePhasePersistence, build, core.ErrUpgradePersistence, err,
		)
	}
	receipt, err := ensureTrialReceipt(ctx, request.Root, target)
	if err != nil {
		identity := error(core.ErrUpgradePersistence)
		if errors.Is(err, core.ErrUpgradeConflict) {
			identity = core.ErrUpgradeConflict
		}
		return newAttemptError(
			FailurePhasePersistence, build, identity, err,
		)
	}
	installed, err := reclaimCandidateSlot(
		ctx, request.Root, target, receipt,
	)
	if err != nil {
		return newAttemptError(
			FailurePhaseCleanup, build, core.ErrUpgradeCleanup, err,
		)
	}
	if installed {
		return nil
	}
	if err := admitStageCapacity(ctx, request, target.candidate); err != nil {
		return newAttemptError(
			FailurePhaseCapacity, build, core.ErrUpgradeCapacity, err,
		)
	}
	return downloadAndVerifyCandidate(ctx, request, target)
}

func ensureTrialReceipt(
	ctx context.Context,
	root *os.Root,
	target TrialTarget,
) (trialDocument, error) {
	expected, err := newTrialDocument(target)
	if err != nil {
		return trialDocument{}, err
	}
	current, err := readTrial(ctx, root, target.slot)
	if err == nil {
		if current != expected {
			return trialDocument{}, conflictError(diagnosticActiveTrial)
		}
		return current, removeTrialTemporary(
			recoveryContext(ctx), root, target.slot,
		)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return trialDocument{}, err
	}
	if err := removeTrialTemporary(
		recoveryContext(ctx), root, target.slot,
	); err != nil {
		return trialDocument{}, persistenceError(err)
	}
	if err := writeTrial(ctx, root, target.slot, expected); err != nil {
		return trialDocument{}, err
	}
	return expected, nil
}

// reclaimCandidateSlot acts only under the exact durable receipt returned by
// ensureTrialReceipt. Authentic bytes are adopted; partial bytes belonging to
// that same attempt are removed for a bounded streaming retry.
func reclaimCandidateSlot(
	ctx context.Context,
	root *os.Root,
	target TrialTarget,
	receipt trialDocument,
) (bool, error) {
	expected, err := newTrialDocument(target)
	if err != nil {
		return false, err
	}
	if receipt != expected {
		return false, conflictError(diagnosticActiveTrial)
	}
	if verifyArtifact(ctx, root, target.slot, target.candidate) == nil {
		return true, nil
	}
	return false, filestore.Remove(ctx, filestore.RemovalRequest{
		Location: filestore.Location{Root: root, Path: target.path},
	})
}

func removeTrialTemporary(
	ctx context.Context,
	root *os.Root,
	slot Slot,
) error {
	path, err := trialTemporaryPath(slot)
	if err != nil {
		return err
	}
	return filestore.Remove(ctx, filestore.RemovalRequest{
		Location: filestore.Location{Root: root, Path: path},
	})
}

func downloadAndVerifyCandidate(
	ctx context.Context,
	request StageRequest,
	target TrialTarget,
) error {
	build := target.candidate.Build()
	download, err := downloadCandidate(ctx, request, target)
	if err != nil {
		cleanupErr := cleanupOwnedCandidate(ctx, request.Root, target, download)
		return newAttemptError(
			FailurePhaseDownload, build, core.ErrUpgradeDownload,
			err, classifyAttemptCleanup(cleanupErr),
		)
	}
	if err := verifyArtifact(
		ctx, request.Root, target.slot, target.candidate,
	); err != nil {
		cleanupErr := removeArtifact(
			recoveryContext(ctx), request.Root, target.slot, target.candidate,
		)
		return newAttemptError(
			FailurePhaseVerification, build,
			core.ErrUpgradeVerification, err, classifyAttemptCleanup(cleanupErr),
		)
	}
	return nil
}

type candidateDownload struct {
	owned bool
}

func cleanupOwnedCandidate(
	ctx context.Context,
	root *os.Root,
	target TrialTarget,
	download candidateDownload,
) error {
	if !download.owned {
		return nil
	}
	return removeArtifact(
		recoveryContext(ctx), root, target.slot, target.candidate,
	)
}

func stageAuthority(
	ctx context.Context,
	request StageRequest,
) (release.Artifact, selectionDocument, error) {
	candidate, err := request.Prepared.Artifact()
	if err != nil {
		return release.Artifact{}, selectionDocument{}, contractError(err)
	}
	installed, err := request.Prepared.InstalledManifest()
	if err != nil {
		return release.Artifact{}, selectionDocument{}, contractError(err)
	}
	artifacts := installed.Artifacts()
	installedArtifact, ok := artifacts.ForPlatform(candidate.Target())
	if !ok {
		return release.Artifact{}, selectionDocument{}, contractError(
			errors.New("installed manifest has no candidate platform"),
		)
	}
	prior, err := readSelection(ctx, request.Root)
	if err != nil {
		return release.Artifact{}, selectionDocument{}, err
	}
	if prior.Artifact != installedArtifact {
		return release.Artifact{}, selectionDocument{}, conflictError(diagnosticCurrentSelection)
	}
	if err := validateUpgradePair(prior.Artifact, candidate); err != nil {
		return release.Artifact{}, selectionDocument{}, err
	}
	return candidate, prior, nil
}

func admitStageCapacity(
	ctx context.Context,
	request StageRequest,
	candidate release.Artifact,
) error {
	extent, err := candidate.Integrity().Extent().Uint64()
	if err != nil {
		return err
	}
	floor, carry := bits.Add64(request.Policy.FreeSpaceReserve.Uint64(), extent, 0)
	if carry != 0 {
		return core.ErrNumericOverflow
	}
	freeSpaceFloor, err := core.NewByteLength(floor)
	if err != nil {
		return err
	}
	_, err = hostfacts.AssessDisk(ctx, hostfacts.DiskAssessmentRequest{
		Directory: request.Directory,
		Policy: hostfacts.DiskPressurePolicy{
			FreeSpaceFloor: freeSpaceFloor,
		},
	})
	return err
}

func prepareCandidateSlot(
	ctx context.Context,
	root *os.Root,
	target TrialTarget,
) error {
	directory, err := slotPath(target.slot)
	if err != nil {
		return err
	}
	return filestore.EnsureDirectory(ctx, filestore.DirectoryRequest{
		Location: filestore.Location{Root: root, Path: directory},
		Mode:     directoryMode,
	})
}

func classifyAttemptCleanup(err error) error {
	if err == nil {
		return nil
	}
	return cleanupError(err)
}

func downloadCandidate(
	ctx context.Context,
	request StageRequest,
	target TrialTarget,
) (candidateDownload, error) {
	file, err := filestore.OpenAppend(ctx, filestore.AppendRequest{
		Location: filestore.Location{Root: request.Root, Path: target.path},
		Mode:     executableMode, Append: filestore.AppendCreate,
	})
	if err != nil {
		return candidateDownload{}, err
	}
	result := candidateDownload{owned: true}
	integrity := target.candidate.Integrity()
	extent, extentErr := integrity.Extent().Uint64()
	if extentErr != nil {
		return result, closeCandidate(file, extentErr)
	}
	length, lengthErr := core.NewByteLength(extent)
	if lengthErr != nil {
		return result, closeCandidate(file, lengthErr)
	}
	download := objectstore.DownloadRequest{
		Destination: file,
		ContentType: core.HTTPMediaTypeOctetStream(),
		Target:      request.Source.Target,
		Integrity: objectstore.Integrity{
			SHA256: integrity.SHA256(),
			Length: length,
			CRC32C: integrity.CRC32C(),
		},
		Policy: request.Source.Policy,
	}
	transfer, downloadErr := executeDownload(
		ctx, request.Source.Provider, request.Source.Client, download,
	)
	if downloadErr == nil {
		downloadErr = transfer.Validate()
	}
	return result, closeCandidate(file, downloadErr)
}

func executeDownload(
	ctx context.Context,
	provider objectstore.Provider,
	client objectstore.Client,
	request objectstore.DownloadRequest,
) (objectstore.Transfer, error) {
	switch provider {
	case objectstore.ProviderAmazonS3:
		return objectstore.DownloadS3(ctx, client, request)
	case objectstore.ProviderGoogleCloudStorage:
		return objectstore.DownloadGCS(ctx, client, request)
	default:
		return objectstore.Transfer{}, contractError(
			errors.New(downloadProviderDiagnostic),
		)
	}
}

func closeCandidate(file *os.File, primary error) error {
	syncErr := file.Sync()
	closeErr := file.Close()
	return errors.Join(primary, syncErr, closeErr)
}
