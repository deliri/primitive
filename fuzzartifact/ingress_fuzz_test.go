package fuzzartifact

import (
	"bytes"
	"encoding/hex"
	json "encoding/json/v2"
	"errors"
	"io"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func FuzzParseGeneratedNameSemanticClosure(f *testing.F) {
	for _, kind := range []ArtifactKind{ArtifactCorpus, ArtifactCrasher} {
		name, err := CacheFormatGo1_27.GeneratedName(kind, core.SHA256Of([]byte(kind.String())))
		if err != nil || name.Validate() != nil {
			f.Fatalf("seed=%+v/%v, want typed generated name", name, err)
		}
		f.Add(uint8(CacheFormatGo1_27), uint8(kind), name.String())
	}
	f.Add(uint8(CacheFormatUnknown), uint8(ArtifactUnknown), "")
	f.Add(uint8(CacheFormatGo1_27), uint8(ArtifactCorpus), "0123456789abcdeF")
	f.Fuzz(func(t *testing.T, rawFormat, rawKind uint8, text string) {
		format, kind := CacheFormat(rawFormat), ArtifactKind(rawKind)
		got, err := ParseGeneratedName(format, kind, text)
		validKind := kind == ArtifactCorpus || kind == ArtifactCrasher
		// Avoid duplicating arbitrary fuzz text into an unbounded oracle buffer.
		validText := false
		if len(text) == generatedNameBytesGo1_27 {
			decoded, decodeErr := hex.DecodeString(text)
			validText = decodeErr == nil && hex.EncodeToString(decoded) == text
		}
		wantValid := validKind && format == CacheFormatGo1_27 && validText
		if wantValid {
			if err != nil || got.Validate() != nil || got.String() != text || got.Kind() != kind || got.Format() != format {
				t.Fatalf("Parse=%+v/%v, want exact accepted input", got, err)
			}
			again, againErr := ParseGeneratedName(got.Format(), got.Kind(), got.String())
			if againErr != nil || again != got {
				t.Fatalf("round trip=%+v/%v, want %+v/nil", again, againErr, got)
			}
			return
		}
		wantErr := core.ErrFuzzArtifactFormat
		if !validKind {
			wantErr = core.ErrFuzzArtifactContract
		}
		if !errors.Is(err, wantErr) || got != (GeneratedName{}) {
			t.Fatalf("Parse refusal=%+v/%v, want zero and %v", got, err, wantErr)
		}
	})
}

func FuzzArtifactKindTextSemanticClosure(f *testing.F) {
	for _, kind := range []ArtifactKind{ArtifactCorpus, ArtifactCrasher} {
		if err := kind.Validate(); err != nil {
			f.Fatal(err)
		}
		f.Add(kind.String())
	}
	f.Add("")
	f.Add(core.UnknownEnumDiagnostic)
	f.Fuzz(func(t *testing.T, text string) {
		var want ArtifactKind
		for _, kind := range []ArtifactKind{ArtifactCorpus, ArtifactCrasher} {
			if text == kind.String() {
				want = kind
			}
		}
		got, err := ParseArtifactKind(text)
		if want == ArtifactUnknown {
			if !errors.Is(err, core.ErrFuzzArtifactFormat) || got != ArtifactUnknown {
				t.Fatalf("ParseArtifactKind=%v/%v, want unknown and format refusal", got, err)
			}
			return
		}
		if err != nil || got != want || got.Validate() != nil {
			t.Fatalf("ParseArtifactKind=%v/%v, want %v/nil", got, err, want)
		}
	})
}

func FuzzArtifactKindJSONSemanticClosure(f *testing.F) {
	for _, kind := range []ArtifactKind{ArtifactCorpus, ArtifactCrasher} {
		wire, err := kind.MarshalJSON()
		if err != nil {
			f.Fatal(err)
		}
		f.Add(wire)
		f.Add(append(bytes.Repeat([]byte{' '}, 65), wire...))
	}
	for _, wire := range [][]byte{nil, []byte("null"), []byte("{}"), []byte("\"\\ud800\""), []byte("\"\\udc00\""), []byte("\"\xff\"")} {
		f.Add(wire)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		// Go's strict decoder is the representation oracle; the two typed tokens
		// are the independent domain oracle. No secondary full-document copy.
		var token string
		decodeErr := json.Unmarshal(data, &token)
		want := ArtifactUnknown
		if decodeErr == nil {
			for _, kind := range []ArtifactKind{ArtifactCorpus, ArtifactCrasher} {
				if token == kind.String() {
					want = kind
				}
			}
		}
		got := ArtifactCrasher
		err := got.UnmarshalJSON(data)
		if want == ArtifactUnknown {
			if err == nil || got != ArtifactCrasher || (!errors.Is(err, core.ErrFuzzArtifactContract) && !errors.Is(err, core.ErrFuzzArtifactFormat)) {
				t.Fatalf("JSON refusal=%v/%v, want preserved receiver and typed refusal", got, err)
			}
			if decodeErr != nil && !errors.Is(err, core.ErrJSONContract) {
				t.Fatalf("JSON syntax refusal=%v, want %v", err, core.ErrJSONContract)
			}
			return
		}
		if err != nil || got != want || got.Validate() != nil {
			t.Fatalf("JSON decode=%v/%v, want %v/nil", got, err, want)
		}
		canonical, marshalErr := got.MarshalJSON()
		var again ArtifactKind
		roundErr := again.UnmarshalJSON(canonical)
		second, secondErr := again.MarshalJSON()
		if marshalErr != nil || roundErr != nil || secondErr != nil || again != got || !bytes.Equal(second, canonical) {
			t.Fatalf("JSON closure=%v/%v/%v/%v, want exact canonical value", again, marshalErr, roundErr, secondErr)
		}
	})
}

func FuzzFindNativeDirectorySemanticClosure(f *testing.F) {
	seed, err := CacheFormatGo1_27.GeneratedName(ArtifactCorpus, core.SHA256Of([]byte("native directory seed")))
	if err != nil || seed.Validate() != nil {
		f.Fatalf("seed=%+v/%v, want typed generated name", seed, err)
	}
	for _, count := range []uint16{0, 1, 63, 64, 65, 127, 128, 129, 257} {
		f.Add(uint64(1), count, uint8(ArtifactCorpus), uint8(0), uint16(0))
	}
	f.Add(uint64(0), uint16(3), uint8(ArtifactCrasher), uint8(3), uint16(1))
	f.Fuzz(func(t *testing.T, base uint64, rawCount uint16, rawKind, flags uint8, rawStop uint16) {
		directory := t.TempDir()
		location := fuzzDirectoryLocation(t, directory)
		// This is a fixture work budget, not a production acceptance limit.
		count := int(rawCount) % 258
		kind := ArtifactKind(rawKind)
		for i := range count {
			writeNameAt(t, location, generatedNameForPosition(t, ArtifactCorpus, base+uint64(i)).String())
		}
		if flags&1 != 0 {
			writeNameAt(t, location, "unsupported")
		}
		if flags&2 != 0 {
			child := location
			child.Path = relativePathForTest(t, cacheDirectoryComponent+"/child")
			if err := filestore.EnsureScratchDirectory(t.Context(), filestore.DirectoryRequest{Location: child, Mode: 0700}); err != nil {
				t.Fatal(err)
			}
			writeNameAt(t, child, seed.String())
		}
		stop := int(rawStop) % (count + 1)
		seen := make([]bool, count)
		calls := 0
		got, err := Find(t.Context(), FindRequest{Location: location, Kind: kind, Format: CacheFormatGo1_27, Visit: func(name GeneratedName) error {
			position, parseErr := strconv.ParseUint(name.String(), 16, 64)
			index := position - base
			if parseErr != nil || index >= uint64(count) || seen[index] || name.Kind() != kind || name.Validate() != nil {
				t.Fatalf("visitor=%+v/%v index=%d, want unique source identity", name, parseErr, index)
			}
			seen[index] = true
			calls++
			if stop != 0 && calls == stop {
				return io.ErrClosedPipe
			}
			return nil
		}})
		if kind != ArtifactCorpus && kind != ArtifactCrasher {
			if !errors.Is(err, core.ErrFuzzArtifactContract) || got != (Observation{}) || calls != 0 {
				t.Fatalf("invalid request=%+v/%v calls=%d, want no facts or callbacks", got, err, calls)
			}
			return
		}
		if got.Validate() != nil || got.Kind() != kind || got.Format() != CacheFormatGo1_27 {
			t.Fatalf("observation=%+v/%v, want validated declared identities", got, err)
		}
		if stop != 0 {
			if !errors.Is(err, io.ErrClosedPipe) || got.State() != ObservationPartial || calls != stop || got.Matched().Uint64() != uint64(stop) || got.Delivered().Uint64() != uint64(stop-1) {
				t.Fatalf("stopped scan=%+v/%v calls=%d, want prefix %d and callback refusal", got, err, calls, stop)
			}
			return
		}
		wantState := ObservationComplete
		var wantErr error
		if flags&1 != 0 {
			wantState = ObservationUnsupportedFormat
			wantErr = core.ErrFuzzArtifactFormat
		}
		if !errors.Is(err, wantErr) || got.State() != wantState || calls != count || got.Matched().Uint64() != uint64(count) || got.Delivered().Uint64() != uint64(count) || got.UnsupportedRegular().Uint64() != uint64(flags&1) || got.IgnoredDirectories().Uint64() != uint64((flags>>1)&1) || got.NonRegular().Uint64() != 0 {
			t.Fatalf("full scan=%+v/%v calls=%d, want exact %d source names and directory facts", got, err, calls, count)
		}
	})
}
