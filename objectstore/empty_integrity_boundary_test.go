package objectstore

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestEmptyObjectIntegrityCannotCarryForeignDigests(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		sha  core.SHA256Digest
		crc  core.CRC32C
		want error
	}{
		{name: "empty object carries Go empty-stream digests", sha: core.SHA256Of(nil), crc: core.NewCRC32C(0)},
		{name: "nonempty SHA256 cannot describe zero bytes", sha: core.SHA256Of([]byte{1}), crc: core.NewCRC32C(0), want: core.ErrObjectStoreIntegrity},
		{name: "nonzero CRC32C cannot describe zero bytes", sha: core.SHA256Of(nil), crc: core.NewCRC32C(1), want: core.ErrObjectStoreIntegrity},
		{name: "two foreign digests cannot corroborate an empty object", sha: core.SHA256Of([]byte{1}), crc: core.NewCRC32C(1), want: core.ErrObjectStoreIntegrity},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			declaration := Integrity{Length: mustByteLength(t, 0), SHA256: tc.sha, CRC32C: tc.crc}
			if err := declaration.Validate(); !errors.Is(err, tc.want) {
				t.Errorf("Integrity.Validate=%v,want %v", err, tc.want)
			}
			transfer := Transfer{provider: ProviderAmazonS3, direction: DirectionUpload, commitment: CommitmentConfirmed, status: core.HTTPStatusOK(), bytes: declaration.Length, sha256: tc.sha, crc32c: tc.crc}
			if err := transfer.Validate(); !errors.Is(err, tc.want) {
				t.Errorf("Transfer.Validate=%v,want %v", err, tc.want)
			}
			evidence := TransferEvidence{provider: ProviderAmazonS3, direction: DirectionUpload, set: true, bytes: declaration.Length, sha256: tc.sha, crc32c: tc.crc}
			if err := evidence.Validate(); !errors.Is(err, tc.want) {
				t.Errorf("TransferEvidence.Validate=%v,want %v", err, tc.want)
			}
			encoded, err := core.MarshalCanonicalJSONDocument(transferEvidenceWireFrom(evidence))
			if err != nil {
				t.Fatal(err)
			}
			before := TransferEvidence{provider: ProviderAmazonS3, direction: DirectionUpload, set: true, sha256: core.SHA256Of(nil), crc32c: core.NewCRC32C(0)}
			got := before
			err = got.UnmarshalJSON(encoded)
			if !errors.Is(err, tc.want) || got != before {
				t.Errorf("received empty proof=(%v,%v),want unchanged exact empty proof %v and %v", got, err, before, tc.want)
			}
		})
	}
}
