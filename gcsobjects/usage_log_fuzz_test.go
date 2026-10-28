package gcsobjects

import (
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// This fast lower-layer campaign complements the real SDK/HTTP public-door
// campaign. It pressures CSV grammar without measuring provider-client setup.
func FuzzGCSUsageCSVSemanticClosure(f *testing.F) {
	seed := usageRecordFixture(f)
	names := usageColumnNames()
	values := usageRecordValues(f, seed)
	f.Add(usageCSVFixture(f, names[:], values))
	f.Add(usageCSVFixture(f, names[:]))
	f.Add([]byte{})
	f.Add([]byte("\"unterminated"))
	reversedNames := append([]string(nil), names[:]...)
	reversedValues := append([]string(nil), values...)
	for left, right := 0, len(names)-1; left < right; left, right = left+1, right-1 {
		reversedNames[left], reversedNames[right] = reversedNames[right], reversedNames[left]
		reversedValues[left], reversedValues[right] = reversedValues[right], reversedValues[left]
	}
	f.Add(usageCSVFixture(f, reversedNames, reversedValues))
	f.Fuzz(func(t *testing.T, input []byte) {
		// Model a caller-selected object extent of at most64KiB. Public request
		// admission separately proves whole objects above the caller bound refuse.
		data := input[:min(len(input), 64*1024)]
		before := bytes.Clone(data)
		reference := csv.NewReader(bytes.NewReader(data))
		header, headerErr := reference.Read()
		visits := uint64(0)
		count, err := visitGCSUsageCSV(t.Context(), bytes.NewReader(data), func(row GCSUsageRecord) error {
			if headerErr != nil {
				t.Fatalf("row emitted beside source header error=%v", headerErr)
			}
			raw, readErr := reference.Read()
			if readErr != nil {
				t.Fatalf("row emitted beside source CSV error=%v", readErr)
			}
			usageOracleSourceFacts(t, row, header, raw)
			encoded := usageCSVFixture(t, names[:], usageRecordValues(t, row))
			var round GCSUsageRecord
			roundCount, roundErr := visitGCSUsageCSV(t.Context(), bytes.NewReader(encoded), func(value GCSUsageRecord) error { round = value; return nil })
			if roundErr != nil || roundCount != 1 || round != row {
				t.Fatalf("typed round trip=%+v/%d/%v,want exact %+v/1/nil", round, roundCount, roundErr, row)
			}
			if second := usageCSVFixture(t, names[:], usageRecordValues(t, round)); !bytes.Equal(second, encoded) {
				t.Fatal("canonical second CSV differs from first")
			}
			visits++
			return nil
		})
		if !bytes.Equal(data, before) {
			t.Fatal("CSV admission mutated caller bytes")
		}
		if err != nil {
			if !errors.Is(err, core.ErrObjectStoreSource) || count != 0 {
				t.Fatalf("refusal=%d/%v,want zero count/typed source error", count, err)
			}
			return
		}
		usageOracleHeader(t, header)
		tail, tailErr := reference.Read()
		if headerErr != nil || tailErr != io.EOF || len(tail) != 0 || count != visits {
			t.Fatalf("source/count closure=%v/%v/%d/%d,want exact EOF/observed rows", headerErr, tailErr, count, visits)
		}
	})
}
