package upgrade

import (
	"context"
	"errors"
	"os"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/release"
)

// Primary is the exact artifact selected for normal product execution.
type Primary struct {
	directory core.AbsolutePath
	command   core.AbsolutePath
	path      core.RelativePath
	artifact  release.Artifact
	slot      Slot
	valid     bool
}

func newPrimary(
	directory core.AbsolutePath,
	document selectionDocument,
) (Primary, error) {
	path, err := binaryPath(document.Slot, document.Artifact.Build())
	if err != nil {
		return Primary{}, err
	}
	command, err := absoluteBinaryPath(
		directory, document.Slot, document.Artifact.Build(),
	)
	if err != nil {
		return Primary{}, err
	}
	value := Primary{
		directory: directory, artifact: document.Artifact,
		slot: document.Slot, path: path, command: command, valid: true,
	}
	if err := value.Validate(); err != nil {
		return Primary{}, err
	}
	return value, nil
}

func (p Primary) Validate() error {
	if !p.valid {
		return contractError(diagnosticPrimary)
	}
	if err := p.directory.Validate(); err != nil {
		return contractError(diagnosticPrimary, err)
	}
	if err := p.artifact.Validate(); err != nil {
		return contractError(diagnosticPrimary, err)
	}
	if err := p.slot.Validate(); err != nil {
		return contractError(diagnosticPrimary, err)
	}
	path, err := binaryPath(p.slot, p.artifact.Build())
	if err != nil || path != p.path {
		return contractError(diagnosticPrimary, err)
	}
	command, err := absoluteBinaryPath(p.directory, p.slot, p.artifact.Build())
	if err != nil || command != p.command {
		return contractError(diagnosticPrimary, err)
	}
	return nil
}

func (p Primary) Artifact() release.Artifact { return p.artifact }
func (p Primary) Slot() Slot                 { return p.slot }
func (p Primary) Path() core.RelativePath    { return p.path }

// Command returns the exact absolute path products pass to Process.
func (p Primary) Command() core.AbsolutePath { return p.command }

func (p Primary) Directory() core.AbsolutePath {
	return p.directory
}

// ResolveRequest supplies the real rooted installation directory.
type ResolveRequest struct {
	Root      *os.Root
	Directory core.AbsolutePath
}

func (r ResolveRequest) Validate() error {
	return validateRoot(r.Root, r.Directory)
}

// ResolvePrimary reads and verifies the selected executable without opening
// any update or licensing state.
func ResolvePrimary(
	ctx context.Context,
	request ResolveRequest,
) (Primary, error) {
	if err := request.Validate(); err != nil {
		return Primary{}, err
	}
	document, err := readSelection(ctx, request.Root)
	if err != nil {
		return Primary{}, err
	}
	if err := verifyArtifact(ctx, request.Root, document.Slot, document.Artifact); err != nil {
		return Primary{}, err
	}
	return newPrimary(request.Directory, document)
}

// PromoteRequest binds a passing trial to the exact rooted installation.
type PromoteRequest struct {
	Root      *os.Root
	Directory core.AbsolutePath
	Promotion Promotion
}

func (r PromoteRequest) Validate() error {
	if err := validateRoot(r.Root, r.Directory); err != nil {
		return err
	}
	if err := r.Promotion.Validate(); err != nil {
		return err
	}
	if r.Promotion.target.directory != r.Directory {
		return contractError(diagnosticRoot)
	}
	return nil
}

// Promote re-verifies the candidate, atomically selects it, proves the new
// primary, then removes only the former fixed slot.
func Promote(
	ctx context.Context,
	request PromoteRequest,
) (Primary, error) {
	if err := request.Validate(); err != nil {
		return Primary{}, err
	}
	current, err := readSelection(ctx, request.Root)
	if err != nil {
		return Primary{}, err
	}
	target := request.Promotion.target
	if current != target.prior {
		return Primary{}, newAttemptError(
			FailurePhasePromotion, target.candidate.Build(),
			core.ErrUpgradeConflict, diagnosticCurrentSelection,
		)
	}
	if err := requireTrialReceipt(ctx, request.Root, target); err != nil {
		return Primary{}, newAttemptError(
			FailurePhasePromotion, target.candidate.Build(),
			core.ErrUpgradeConflict, err,
		)
	}
	if err := verifyArtifact(ctx, request.Root, target.slot, target.candidate); err != nil {
		return Primary{}, newAttemptError(
			FailurePhaseVerification, target.candidate.Build(),
			core.ErrUpgradeVerification, err,
		)
	}
	next := selectionDocument{
		Revision: selectionRevisionCurrent,
		Slot:     target.slot, Artifact: target.candidate,
	}
	if err := writeSelection(ctx, request.Root, next, filestore.InstallReplace); err != nil {
		return Primary{}, newAttemptError(
			FailurePhasePersistence, target.candidate.Build(),
			core.ErrUpgradePersistence, err,
		)
	}
	primary, err := ResolvePrimary(ctx, ResolveRequest{
		Root: request.Root, Directory: request.Directory,
	})
	if err != nil {
		projected, primaryErr := newPrimary(request.Directory, next)
		return projected, newAttemptError(
			FailurePhasePromotion, target.candidate.Build(),
			core.ErrUpgradePromotion, err, primaryErr,
		)
	}
	// The selector is already committed, so removing the former slot must settle
	// even when the caller's context is done. Leaving those bytes behind would
	// occupy the next candidate's slot.
	cleanupErr := errors.Join(
		removeTrialMetadata(recoveryContext(ctx), request.Root, target.slot),
		removeArtifact(
			recoveryContext(ctx), request.Root,
			target.prior.Slot, target.prior.Artifact,
		),
	)
	if cleanupErr != nil {
		return primary, newAttemptError(
			FailurePhaseCleanup, target.candidate.Build(),
			core.ErrUpgradeCleanup, cleanupErr,
		)
	}
	return primary, nil
}

// DiscardTrialRequest names one exact unselected candidate to remove after a
// rejected or abandoned product trial.
type DiscardTrialRequest struct {
	Root      *os.Root
	Directory core.AbsolutePath
	Target    TrialTarget
}

func (r DiscardTrialRequest) Validate() error {
	if err := validateRoot(r.Root, r.Directory); err != nil {
		return err
	}
	if err := r.Target.Validate(); err != nil {
		return err
	}
	if r.Target.directory != r.Directory {
		return contractError(diagnosticRoot)
	}
	return nil
}

// DiscardTrial removes only the exact candidate while the primary still
// matches the authority from which that candidate was staged.
func DiscardTrial(
	ctx context.Context,
	request DiscardTrialRequest,
) error {
	if err := request.Validate(); err != nil {
		return err
	}
	current, err := readSelection(ctx, request.Root)
	if err != nil {
		return err
	}
	if current != request.Target.prior {
		return newAttemptError(
			FailurePhaseCleanup, request.Target.candidate.Build(),
			core.ErrUpgradeConflict, diagnosticCurrentSelection,
		)
	}
	if err := requireTrialReceipt(
		ctx, request.Root, request.Target,
	); err != nil {
		return newAttemptError(
			FailurePhaseCleanup, request.Target.candidate.Build(),
			core.ErrUpgradeConflict, err,
		)
	}
	if err := verifyArtifact(
		ctx, request.Root, request.Target.slot, request.Target.candidate,
	); err != nil {
		return newAttemptError(
			FailurePhaseVerification, request.Target.candidate.Build(),
			core.ErrUpgradeVerification, err,
		)
	}
	if err := removeArtifact(
		ctx, request.Root, request.Target.slot, request.Target.candidate,
	); err != nil {
		return newAttemptError(
			FailurePhaseCleanup, request.Target.candidate.Build(),
			core.ErrUpgradeCleanup, err,
		)
	}
	return nil
}

func removeArtifact(
	ctx context.Context,
	root *os.Root,
	slot Slot,
	artifact release.Artifact,
) error {
	path, err := binaryPath(slot, artifact.Build())
	if err != nil {
		return err
	}
	if err := filestore.Remove(ctx, filestore.RemovalRequest{
		Location: filestore.Location{Root: root, Path: path},
	}); err != nil {
		return err
	}
	if err := removeTrialMetadata(ctx, root, slot); err != nil {
		return err
	}
	directory, err := slotPath(slot)
	if err != nil {
		return err
	}
	return filestore.Remove(ctx, filestore.RemovalRequest{
		Location: filestore.Location{Root: root, Path: directory},
	})
}

func removeTrialMetadata(
	ctx context.Context,
	root *os.Root,
	slot Slot,
) error {
	temporary, err := trialTemporaryPath(slot)
	if err != nil {
		return err
	}
	if err := filestore.Remove(ctx, filestore.RemovalRequest{
		Location: filestore.Location{Root: root, Path: temporary},
	}); err != nil {
		return err
	}
	path, err := trialPath(slot)
	if err != nil {
		return err
	}
	return filestore.Remove(ctx, filestore.RemovalRequest{
		Location: filestore.Location{Root: root, Path: path},
	})
}
