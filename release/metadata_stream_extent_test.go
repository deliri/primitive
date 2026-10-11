package release

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"hash/crc32"
	"io"
	"math"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzMetadataDigestNativeExtentAndIntegrity(f *testing.F) {
	for _, data := range [][]byte{nil, []byte{0}, []byte("metadata"), bytes.Repeat([]byte{251}, 65537)} {
		f.Add(data, int8(0), uint8(MetadataKindDocumentation))
	}
	f.Add([]byte("metadata"), int8(-1), uint8(MetadataKindDocumentation))
	f.Add([]byte("metadata"), int8(1), uint8(MetadataKindDocumentation))
	f.Fuzz(func(t *testing.T, data []byte, delta int8, rawKind uint8) {
		claimed := int64(len(data)) + int64(delta)
		var extent core.ByteCount
		if claimed > 0 {
			var err error
			extent, err = core.NewByteCount(uint64(claimed))
			if err != nil {
				t.Fatal(err)
			}
		}
		kind := MetadataKind(rawKind)
		got, gotErr := InspectMetadataAsset(MetadataInspectionRequest{Source: bytes.NewReader(data), Extent: extent, Kind: kind})
		wantAccepted := len(data) > 0 && delta == 0 && rawKind >= uint8(MetadataKindDependencies) && rawKind <= uint8(MetadataKindReleaseNotes)
		if !wantAccepted {
			if !errors.Is(gotErr, core.ErrReleaseContract) || got != (MetadataAsset{}) {
				t.Fatalf("native refusal = (%v,%v), want zero and release contract", got, gotErr)
			}
			return
		}
		if gotErr != nil || got.Kind() != kind || got.Integrity().Extent() != extent || got.Integrity().SHA256() != core.NewSHA256Digest(sha256.Sum256(data)) || got.Integrity().CRC32C() != core.NewCRC32C(crc32.Checksum(data, crc32.MakeTable(crc32.Castagnoli))) {
			t.Fatalf("native metadata = (%v,%v), want exact kind, extent and integrity", got, gotErr)
		}
	})
}

func BenchmarkMetadataDigestNativeFixedWindow(b *testing.B) {
	for _, size := range [...]uint64{65536, 65537, 16<<20 + 1} {
		b.Run(strconv.FormatUint(size, 10), func(b *testing.B) {
			extent, err := core.NewByteCount(size)
			if err != nil {
				b.Fatal(err)
			}
			request := MetadataInspectionRequest{Source: nativeMetadataBytes{extent: int64(size)}, Extent: extent, Kind: MetadataKindDocumentation}
			b.ReportAllocs()
			b.SetBytes(int64(size))
			for b.Loop() {
				got, err := InspectMetadataAsset(request)
				if err != nil || got.Kind() != request.Kind || got.Integrity().Extent() != extent {
					b.Fatalf("metadata = (%v,%v), want exact kind and extent %v", got, err, extent)
				}
			}
		})
	}
}

type nativeMetadataBytes struct {
	extent   int64
	terminal error
}

func (s nativeMetadataBytes) ReadAt(buffer []byte, offset int64) (int, error) {
	if offset >= s.extent {
		if s.terminal != nil {
			return 0, s.terminal
		}
		return 0, io.EOF
	}
	count := min(int64(len(buffer)), s.extent-offset)
	for index := range int(count) {
		buffer[index] = byte((offset + int64(index)) % 251)
	}
	if count != int64(len(buffer)) {
		return int(count), io.EOF
	}
	return int(count), nil
}

func TestMetadataDigestContinuesBeyondFormerExtentCeiling(t *testing.T) {
	t.Parallel()
	for _, size := range [...]uint64{1, 65535, 65536, 65537, 256<<20 - 1, 256 << 20, 256<<20 + 1} {
		t.Run(strconv.FormatUint(size, 10), func(t *testing.T) {
			t.Parallel()
			extent, err := core.NewByteCount(size)
			if err != nil {
				t.Fatal(err)
			}
			source := nativeMetadataBytes{extent: int64(size)}
			got, gotErr := InspectMetadataAsset(MetadataInspectionRequest{Source: source, Extent: extent, Kind: MetadataKindDocumentation})
			sha := sha256.New()
			crc := crc32.New(crc32.MakeTable(crc32.Castagnoli))
			count, oracleErr := io.CopyBuffer(io.MultiWriter(sha, crc), io.NewSectionReader(source, 0, int64(size)), make([]byte, 65536))
			want := core.NewSHA256Digest([32]byte(sha.Sum(nil)))
			if gotErr != nil || oracleErr != nil || count != int64(size) || got.Integrity().Extent() != extent || got.Integrity().SHA256() != want || got.Integrity().CRC32C() != core.NewCRC32C(crc.Sum32()) {
				t.Fatalf("metadata extent %d = (%v,%v), oracle (%d,%v); want exact native extent, SHA-256 and CRC32C", size, got, gotErr, count, oracleErr)
			}
		})
	}
}

func TestMetadataDigestRequiresExactExtentAndStandaloneEOF(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		sourceBytes int64
		claimed     uint64
		terminal    error
		cause       error
	}{
		{name: "short source retains read refusal", sourceBytes: 65536, claimed: 65537},
		{name: "extra source byte prevents proof", sourceBytes: 65537, claimed: 65536},
		{name: "joined EOF cannot hide producer failure", sourceBytes: 65536, claimed: 65536, terminal: errors.Join(io.EOF, io.ErrClosedPipe), cause: io.ErrClosedPipe},
		{name: "wrapped failure prevents proof", sourceBytes: 65536, claimed: 65536, terminal: io.ErrUnexpectedEOF, cause: io.ErrUnexpectedEOF},
		{name: "unsigned extent cannot wrap native section offset", claimed: math.MaxUint64},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			extent, err := core.NewByteCount(tc.claimed)
			if err != nil {
				t.Fatal(err)
			}
			got, gotErr := InspectMetadataAsset(MetadataInspectionRequest{Source: nativeMetadataBytes{extent: tc.sourceBytes, terminal: tc.terminal}, Extent: extent, Kind: MetadataKindDocumentation})
			if !errors.Is(gotErr, core.ErrReleaseContract) || tc.cause != nil && !errors.Is(gotErr, tc.cause) || got != (MetadataAsset{}) {
				t.Fatalf("extent refusal = (%v,%v), want zero, release contract and cause %v", got, gotErr, tc.cause)
			}
		})
	}
}
