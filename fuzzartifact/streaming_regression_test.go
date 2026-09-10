package fuzzartifact

import (
	"bytes"
	"errors"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestFindDoesNotLoseNamesAtFormerRetentionBoundary(t *testing.T) {
	t.Parallel()
	for _, size := range []int{127, 128, 129, 8192} {
		t.Run(strconv.Itoa(size)+" native generated names", func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			location := fuzzDirectoryLocation(t, directory)
			for position := range size {
				writeNameAt(t, location, generatedNameForPosition(t, ArtifactCorpus, uint64(position)).String())
			}
			var calls uint64
			got, err := Find(t.Context(), FindRequest{Location: location, Kind: ArtifactCorpus, Format: CacheFormatGo1_27, Visit: func(GeneratedName) error { calls++; return nil }})
			if err != nil || got.Delivered().Uint64() != uint64(size) || calls != uint64(size) {
				t.Fatalf("Find generated count/error = %d/%v, want %d/nil", got.Delivered().Uint64(), err, size)
			}
		})
	}
}

func TestArtifactKindWhitespaceDoesNotBecomeAByteQuota(t *testing.T) {
	t.Parallel()
	wire, err := ArtifactCorpus.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	for _, size := range []int{len(wire), 63, 64, 65, 32769} {
		t.Run(strconv.Itoa(size)+" byte valid token document", func(t *testing.T) {
			t.Parallel()
			data := append(bytes.Repeat([]byte{' '}, size-len(wire)), wire...)
			got := ArtifactCrasher
			err := got.UnmarshalJSON(data)
			if err != nil || got != ArtifactCorpus {
				t.Fatalf("UnmarshalJSON(%d bytes) = %v/%v, want %v/nil", len(data), got, err, ArtifactCorpus)
			}
			poisoned := append(data, '!')
			got = ArtifactCrasher
			if err := got.UnmarshalJSON(poisoned); !errors.Is(err, core.ErrJSONContract) || got != ArtifactCrasher {
				t.Fatalf("UnmarshalJSON(invalid tail) = %v/%v, want unchanged receiver and %v", got, err, core.ErrJSONContract)
			}
		})
	}
}
