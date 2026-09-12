package filestore_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func FuzzInspectContentIndexExactBinding(f *testing.F) {
	extent, err := core.NewByteLength(1)
	if err != nil {
		f.Fatal(err)
	}
	entry := filestore.ContentIndexEntry{Digest: core.SHA256Of([]byte("index seed")), Extent: extent}
	var seed bytes.Buffer
	if err := filestore.WriteContentIndexEntry(&seed, entry); err != nil {
		f.Fatal(err)
	}
	f.Add(seed.Bytes(), false)
	f.Add(seed.Bytes(), true)
	f.Add([]byte{}, false)
	f.Add(append(bytes.Clone(seed.Bytes()), seed.Bytes()...), false)
	f.Fuzz(func(t *testing.T, data []byte, foreignDigest bool) {
		dir := t.TempDir()
		file, err := os.Create(filepath.Join(dir, "index"))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := file.Close(); err != nil {
				t.Errorf("Close() error = %v, want nil", err)
			}
		})
		if _, err := file.Write(data); err != nil {
			t.Fatal(err)
		}
		encodedExtent, err := core.NewByteLength(uint64(len(data)))
		if err != nil {
			t.Fatal(err)
		}
		checksum := sha256.Sum256(data)
		if foreignDigest {
			checksum[0] ^= 1
		}
		request := filestore.ContentIndexInspectionRequest{File: file, Content: filestore.ContentIndexEntry{Digest: core.NewSHA256Digest(checksum), Extent: encodedExtent}}
		wantAdmitted := !foreignDigest && len(data)%filestore.ContentIndexRecordBytes == 0
		var count, total uint64
		var prior [core.SHA256DigestBytes]byte
		empty := sha256.Sum256(nil)
		for offset := 0; wantAdmitted && offset < len(data); offset += filestore.ContentIndexRecordBytes {
			var digest [core.SHA256DigestBytes]byte
			copy(digest[:], data[offset:offset+core.SHA256DigestBytes])
			size := binary.BigEndian.Uint64(data[offset+core.SHA256DigestBytes : offset+filestore.ContentIndexRecordBytes])
			if size > math.MaxInt64 || (size == 0 && digest != empty) || (count != 0 && bytes.Compare(prior[:], digest[:]) >= 0) || total > math.MaxInt64-size {
				wantAdmitted = false
				break
			}
			prior = digest
			total += size
			count++
		}
		got, err := filestore.InspectContentIndex(t.Context(), request)
		if !wantAdmitted {
			if !errors.Is(err, core.ErrFilestoreContract) || got != (filestore.ContentIndexSummary{}) {
				t.Fatalf("InspectContentIndex(refused) = (%+v,%v), want zero and typed refusal", got, err)
			}
			return
		}
		if err != nil || got.Entries != count || got.Extent.Uint64() != total {
			t.Fatalf("InspectContentIndex() = (%+v,%v), want %d entries/%d bytes and nil", got, err, count, total)
		}
		info, err := file.Stat()
		if err != nil || info.Size() != int64(len(data)) {
			t.Fatalf("borrowed file = (%v,%v), want unchanged extent and open handle", info, err)
		}
	})
}
