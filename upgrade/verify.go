package upgrade

import (
	"context"
	"crypto/sha256"
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
	sha := sha256.New()
	crc := crc32.New(crc32.MakeTable(crc32.Castagnoli))
	count, err := filestore.Read(ctx, filestore.ReadRequest{
		Destination:  io.MultiWriter(sha, crc),
		Location:     filestore.Location{Root: root, Path: path},
		MaximumBytes: integrity.Extent(),
	})
	if err != nil {
		return verificationError(diagnosticCandidateBytes, err)
	}
	if count.Uint64() != extent ||
		core.NewSHA256Digest([sha256.Size]byte(sha.Sum(nil))) != integrity.SHA256() ||
		core.NewCRC32C(crc.Sum32()) != integrity.CRC32C() {
		return verificationError(diagnosticCandidateBytes)
	}
	return nil
}
