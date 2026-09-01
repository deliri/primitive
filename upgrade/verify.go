package upgrade

import (
	"context"
	"errors"
	"hash/crc32"
	"io"
	"os"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/release"
)

type artifactVerificationState uint8

const (
	artifactVerificationUnknown artifactVerificationState = iota
	artifactVerificationAuthentic
	artifactVerificationAbsent
	artifactVerificationInvalid
)

func verifyArtifact(
	ctx context.Context,
	root *os.Root,
	slot Slot,
	artifact release.Artifact,
) error {
	state, err := inspectArtifact(ctx, root, slot, artifact)
	if state == artifactVerificationAuthentic {
		return nil
	}
	return verificationError(diagnosticCandidateBytes, err)
}

func inspectArtifact(
	ctx context.Context,
	root *os.Root,
	slot Slot,
	artifact release.Artifact,
) (artifactVerificationState, error) {
	path, integrity, extent, err := artifactVerificationInput(slot, artifact)
	if err != nil {
		return artifactVerificationUnknown, err
	}
	count, digest, hashed, checksum, err := readArtifactIntegrity(ctx, root, path, integrity)
	if err != nil {
		return classifyArtifactReadError(err)
	}
	if count.Uint64() != extent || hashed.Uint64() != extent ||
		digest != integrity.SHA256() || checksum != integrity.CRC32C() {
		return artifactVerificationInvalid, nil
	}
	return artifactVerificationAuthentic, nil
}

func artifactVerificationInput(
	slot Slot,
	artifact release.Artifact,
) (core.RelativePath, release.ArtifactIntegrity, uint64, error) {
	if err := artifact.Validate(); err != nil {
		return core.RelativePath{}, release.ArtifactIntegrity{}, 0, err
	}
	path, err := binaryPath(slot, artifact.Build())
	if err != nil {
		return core.RelativePath{}, release.ArtifactIntegrity{}, 0, err
	}
	integrity := artifact.Integrity()
	extent, err := integrity.Extent().Uint64()
	return path, integrity, extent, err
}

func readArtifactIntegrity(
	ctx context.Context,
	root *os.Root,
	path core.RelativePath,
	integrity release.ArtifactIntegrity,
) (core.ByteLength, core.SHA256Digest, core.ByteLength, core.CRC32C, error) {
	sha := core.NewDigestWriter()
	crc := crc32.New(crc32.MakeTable(crc32.Castagnoli))
	count, err := filestore.Read(ctx, filestore.ReadRequest{
		Destination:  io.MultiWriter(sha, crc),
		Location:     filestore.Location{Root: root, Path: path},
		MaximumBytes: integrity.Extent(),
	})
	if err != nil {
		return core.ByteLength{}, core.SHA256Digest{}, core.ByteLength{}, core.CRC32C{}, err
	}
	digest, hashed, err := sha.Seal()
	if err != nil {
		return core.ByteLength{}, core.SHA256Digest{}, core.ByteLength{}, core.CRC32C{}, err
	}
	return count, digest, hashed, core.NewCRC32C(crc.Sum32()), nil
}

func classifyArtifactReadError(err error) (artifactVerificationState, error) {
	if errors.Is(err, os.ErrNotExist) {
		return artifactVerificationAbsent, err
	}
	if errors.Is(err, core.ErrFilestoreSize) {
		return artifactVerificationInvalid, nil
	}
	return artifactVerificationUnknown, err
}
