package upgrade

import (
	"context"
	"errors"
	"io"
	"os"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/release"
)

const bootstrapTemporaryFilename = ".primitive-bootstrap"

// BootstrapRequest establishes the first selector from an authenticated
// installed manifest and the exact currently working executable stream.
type BootstrapRequest struct {
	Source    io.Reader
	Root      *os.Root
	Directory core.AbsolutePath
	Manifest  release.VerifiedManifest
	Build     core.BuildIdentity
}

func (r BootstrapRequest) Validate() error {
	if err := validateRoot(r.Root, r.Directory); err != nil {
		return err
	}
	if err := r.Manifest.Validate(); err != nil {
		return contractError(err)
	}
	if err := r.Build.Validate(); err != nil {
		return contractError(err)
	}
	if r.Source == nil {
		return contractError(errors.New("bootstrap source is missing"))
	}
	return nil
}

// Bootstrap writes and verifies slot A before exclusively creating the first
// selection. Any failure leaves no primary selector.
func Bootstrap(
	ctx context.Context,
	request BootstrapRequest,
) (Primary, error) {
	if err := request.Validate(); err != nil {
		return Primary{}, err
	}
	artifacts, err := request.Manifest.Artifacts()
	if err != nil {
		return Primary{}, contractError(err)
	}
	artifact, ok := artifacts.ForPlatform(request.Build.Platform())
	if !ok || artifact.Build() != request.Build {
		return Primary{}, contractError(errors.New("bootstrap build differs from manifest"))
	}
	write, err := writeBootstrapArtifact(
		ctx, request.Root, artifact, request.Source,
	)
	if err != nil {
		cleanupErr := cleanupBootstrapArtifact(
			ctx, request.Root, artifact, write,
		)
		return Primary{}, newAttemptError(
			FailurePhaseBootstrap, artifact.Build(), core.ErrUpgradePersistence,
			err, classifyAttemptCleanup(cleanupErr),
		)
	}
	document := selectionDocument{
		Revision: selectionRevisionCurrent, Slot: SlotA, Artifact: artifact,
	}
	if err := writeSelection(ctx, request.Root, document, filestore.InstallCreate); err != nil {
		cleanupErr := removeArtifact(recoveryContext(ctx), request.Root, SlotA, artifact)
		return Primary{}, newAttemptError(
			FailurePhaseBootstrap, artifact.Build(), core.ErrUpgradePersistence,
			err, classifyAttemptCleanup(cleanupErr),
		)
	}
	return ResolvePrimary(ctx, ResolveRequest{
		Root: request.Root, Directory: request.Directory,
	})
}

func writeBootstrapArtifact(
	ctx context.Context,
	root *os.Root,
	artifact release.Artifact,
	source io.Reader,
) (bootstrapWrite, error) {
	directory, err := slotPath(SlotA)
	if err != nil {
		return bootstrapWrite{}, err
	}
	if err := filestore.EnsureDirectory(ctx, filestore.DirectoryRequest{
		Location: filestore.Location{Root: root, Path: directory},
		Mode:     directoryMode,
	}); err != nil {
		return bootstrapWrite{}, err
	}
	path, err := binaryPath(SlotA, artifact.Build())
	if err != nil {
		return bootstrapWrite{}, err
	}
	temporary, err := core.ParseRelativePath(
		SlotA.String() + string(os.PathSeparator) + bootstrapTemporaryFilename,
	)
	if err != nil {
		return bootstrapWrite{}, err
	}
	recovery, err := filestore.Write(ctx, filestore.WriteRequest{
		Source:    source,
		Location:  filestore.Location{Root: root, Path: path},
		Temporary: temporary, Mode: executableMode,
		Install:      filestore.InstallCreate,
		MaximumBytes: artifact.Integrity().Extent(),
	})
	if err != nil {
		if recovery.Validate() != nil {
			// A zero receipt is Filestore's proof that no package-owned name
			// survived the failed write, so this call created nothing to clean.
			return bootstrapWrite{}, err
		}
		if recoveryErr := filestore.Recover(
			recoveryContext(ctx), recovery,
		); recoveryErr != nil {
			// A live receipt that would not settle leaves the target name in an
			// indeterminate state. This call created it, so this call owns it.
			return bootstrapWrite{owned: true}, errors.Join(err, recoveryErr)
		}
	}
	owned := bootstrapWrite{owned: true}
	return owned, verifyArtifact(ctx, root, SlotA, artifact)
}

type bootstrapWrite struct {
	owned bool
}

func cleanupBootstrapArtifact(
	ctx context.Context,
	root *os.Root,
	artifact release.Artifact,
	write bootstrapWrite,
) error {
	if !write.owned {
		return nil
	}
	return removeArtifact(recoveryContext(ctx), root, SlotA, artifact)
}
