package objectstore_test

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"hash/crc32"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/zeebo/blake3"
)

func FuzzInspectSemanticIntegrityAndSizeClosure(f *testing.F) {
	seeds := []struct {
		data    []byte
		maximum uint16
	}{
		{data: []byte("x"), maximum: 1},
		{data: []byte("xy"), maximum: 1},
		{data: []byte("xy"), maximum: 2},
		{data: bytes.Repeat([]byte{0xff}, 32767), maximum: 32767},
		{data: bytes.Repeat([]byte{0x7f}, 32768), maximum: 32768},
		{data: bytes.Repeat([]byte{0x01}, 32769), maximum: 32768},
		{data: nil, maximum: 1},
		{data: []byte("x"), maximum: 0},
	}
	for _, seed := range seeds {
		f.Add(seed.data, seed.maximum)
	}
	f.Fuzz(func(t *testing.T, data []byte, maximum uint16) {
		if len(data) > 1<<20 {
			return
		}
		limit, limitErr := core.NewByteCount(uint64(maximum))
		got, gotErr := objectstore.Inspect(t.Context(), objectstore.InspectionRequest{
			Source: bytes.NewReader(data), MaximumBytes: limit,
		})
		if limitErr != nil {
			if got != (objectstore.Inspection{}) || !errors.Is(gotErr, core.ErrObjectStoreContract) {
				t.Fatalf("objectstore.Inspect(zero maximum) = (%+v, %v), want zero typed refusal", got, gotErr)
			}
			return
		}
		if len(data) == 0 || uint64(len(data)) > uint64(maximum) {
			if got != (objectstore.Inspection{}) || !errors.Is(gotErr, core.ErrObjectStoreSize) {
				t.Fatalf("objectstore.Inspect(size %d, maximum %d) = (%+v, %v), want zero and errors.Is(_, %v)", len(data), maximum, got, gotErr, core.ErrObjectStoreSize)
			}
			return
		}
		if gotErr != nil || got.Validate() != nil {
			t.Fatalf("objectstore.Inspect(accepted size %d) = (%+v, %v), want validated and nil", len(data), got, gotErr)
		}
		wantSHA := core.NewSHA256Digest(sha256.Sum256(data))
		wantBLAKE3 := objectstore.NewBLAKE3Digest(blake3.Sum256(data))
		wantCRC := core.NewCRC32C(crc32.Checksum(data, crc32.MakeTable(crc32.Castagnoli)))
		if got.Integrity.Length.Uint64() != uint64(len(data)) || got.Integrity.SHA256 != wantSHA || got.BLAKE3 != wantBLAKE3 || got.Integrity.CRC32C != wantCRC {
			t.Fatalf("objectstore.Inspect(accepted) = %+v, want independent length=%d sha256=%v blake3=%v crc32c=%v", got, len(data), wantSHA, wantBLAKE3, wantCRC)
		}
	})
}
