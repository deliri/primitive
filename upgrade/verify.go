package upgrade

import (
	"context"
	"hash/crc32"
	"io"
	"os"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/release"
)

func verifyArtifact(
	ctx context.Context,
	root *os.Root,
	slot Slot,
	artifact release.Artifact,
) error {
	if err := artifact.Validate(); err != nil {
		return verificationError(err)
	}
	path, err := binaryPath(slot, artifact.Build())
	if err != nil {
		return verificationError(err)
	}
	integrity := artifact.Integrity()
	extent, err := integrity.Extent().Uint64()
	if err != nil {
		return verificationError(diagnosticCandidateBytes, err)
	}
	sha := core.NewDigestWriter()
	crc := crc32.New(crc32.MakeTable(crc32.Castagnoli))
	count, err := filestore.Read(ctx, filestore.ReadRequest{
		Destination:  io.MultiWriter(sha, crc),
		Location:     filestore.Location{Root: root, Path: path},
		MaximumBytes: integrity.Extent(),
	})
	if err != nil {
		return verificationError(diagnosticCandidateBytes, err)
	}
	digest, hashed, err := sha.Seal()
	if err != nil {
		return verificationError(diagnosticCandidateBytes, err)
	}
	// The sealed count is checked alongside the streamed count: they describe
	// the same bytes from two independent owners, so a disagreement means the
	// artifact was not read the way verification believes it was.
	if count.Uint64() != extent || hashed.Uint64() != extent {
		return verificationError(diagnosticCandidateBytes)
	}
	if digest != integrity.SHA256() || core.NewCRC32C(crc.Sum32()) != integrity.CRC32C() {
		return verificationError(diagnosticCandidateBytes)
	}
	return nil
}
