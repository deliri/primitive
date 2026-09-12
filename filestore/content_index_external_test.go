package filestore_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"io"
	"math"
	"testing"
)

func FuzzContentIndexCanonicalRecord(f *testing.F) {
	extent, err := core.NewByteLength(23)
	if err != nil {
		f.Fatal(err)
	}
	entry := filestore.ContentIndexEntry{Digest: core.SHA256Of([]byte("content record seed")), Extent: extent}
	var seed bytes.Buffer
	if err := filestore.WriteContentIndexEntry(&seed, entry); err != nil {
		f.Fatal(err)
	}
	f.Add(seed.Bytes())
	f.Add([]byte{})
	f.Add([]byte{1})
	f.Fuzz(func(t *testing.T, data []byte) {
		source := bytes.NewReader(data)
		got, err := filestore.ReadContentIndexEntry(source)
		wantAdmitted := false
		if len(data) >= filestore.ContentIndexRecordBytes {
			extent := binary.BigEndian.Uint64(data[core.SHA256DigestBytes:filestore.ContentIndexRecordBytes])
			empty := sha256.Sum256(nil)
			wantAdmitted = extent <= math.MaxInt64 && (extent != 0 || bytes.Equal(data[:core.SHA256DigestBytes], empty[:]))
		}
		if (err == nil) != wantAdmitted {
			t.Fatalf("record admission error = %v, want admitted = %v from exact width, extent and empty-content binding", err, wantAdmitted)
		}
		if err != nil {
			if got != (filestore.ContentIndexEntry{}) || (!errors.Is(err, io.EOF) && !errors.Is(err, core.ErrFilestoreContract)) {
				t.Fatalf("filestore.ReadContentIndexEntry(refused) = (%+v,%v), want zero and typed refusal", got, err)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("admitted Validate() error = %v, want nil", err)
		}
		var output bytes.Buffer
		if err := filestore.WriteContentIndexEntry(&output, got); err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(output.Bytes(), data[:filestore.ContentIndexRecordBytes]) || source.Len() != len(data)-filestore.ContentIndexRecordBytes {
			t.Fatalf("canonical record = %x, remaining = %d, want exact first record and untouched suffix", output.Bytes(), source.Len())
		}
		roundTrip, err := filestore.ReadContentIndexEntry(&output)
		if err != nil || roundTrip != got {
			t.Fatalf("round trip = (%+v,%v), want (%+v,nil)", roundTrip, err, got)
		}
	})
}
