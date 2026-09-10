package fuzzartifact

import (
	"encoding/hex"
	json "encoding/json/v2"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestCacheFormatExhaustsClosedDomainAndPinsGeneratedNameStorage(t *testing.T) {
	t.Parallel()
	for raw := range 256 {
		t.Run(strconv.Itoa(raw), func(t *testing.T) {
			t.Parallel()
			format := CacheFormat(raw)
			wantValid := format == CacheFormatGo1_27
			err := format.Validate()
			if (err == nil) != wantValid || format.IsValid() != wantValid {
				t.Fatalf("format %d validity=%v/%v, want %v", raw, err, format.IsValid(), wantValid)
			}
			if !wantValid {
				_, widthErr := format.GeneratedNameBytes(ArtifactCorpus)
				if !errors.Is(err, core.ErrFuzzArtifactFormat) || !errors.Is(widthErr, core.ErrFuzzArtifactFormat) || format.String() != core.UnknownEnumDiagnostic {
					t.Fatalf("unknown format=%v/%v/%v, want format identity and unknown diagnostic", format, err, widthErr)
				}
				return
			}
			for _, kind := range []ArtifactKind{ArtifactCorpus, ArtifactCrasher} {
				width, err := format.GeneratedNameBytes(kind)
				n, nErr := width.Uint64()
				if err != nil || nErr != nil || n != uint64(len(GeneratedName{}.value)) {
					t.Fatalf("name width=%d/%v/%v, want fixed storage width", n, err, nErr)
				}
			}
		})
	}
	if _, ok := any(CacheFormatGo1_27).(json.Marshaler); ok {
		t.Fatalf("CacheFormat marshaler=%v, want off-wire", ok)
	}
}

// Exact-width lowercase hexadecimal has a compositional grammar: exhaust
// every byte at every position, with encoding/hex as an independent oracle.
// This replaces repeated 10/10/20 rows that changed only decorative payload.
func TestGeneratedNameByteGrammarExhaustiveLayerTriad(t *testing.T) {
	t.Parallel()
	for position := range generatedNameBytesGo1_27 {
		t.Run("byte "+strconv.Itoa(position), func(t *testing.T) {
			t.Parallel()
			for raw := range 256 {
				data := []byte(strings.Repeat("0", generatedNameBytesGo1_27))
				data[position] = byte(raw)
				text := string(data)
				decoded, decodeErr := hex.DecodeString(text)
				wantValid := decodeErr == nil && hex.EncodeToString(decoded) == text
				for _, kind := range []ArtifactKind{ArtifactCorpus, ArtifactCrasher} {
					got, err := ParseGeneratedName(CacheFormatGo1_27, kind, text)
					if wantValid {
						if err != nil || got.Validate() != nil || got.String() != text || got.Kind() != kind || got.Format() != CacheFormatGo1_27 {
							t.Fatalf("Parse byte[%d]=%d kind=%v got=%+v/%v, want exact valid input", position, raw, kind, got, err)
						}
						round, roundErr := ParseGeneratedName(got.Format(), got.Kind(), got.String())
						if roundErr != nil || round != got {
							t.Fatalf("name round trip=%+v/%v, want %+v/nil", round, roundErr, got)
						}
					} else if !errors.Is(err, core.ErrFuzzArtifactFormat) || got != (GeneratedName{}) {
						t.Fatalf("Parse byte[%d]=%d = %+v/%v, want zero and format refusal", position, raw, got, err)
					}
				}
			}
		})
	}
}

func TestGeneratedNameShapeAndDomainBoundaries(t *testing.T) {
	t.Parallel()
	for _, length := range []int{0, generatedNameBytesGo1_27 - 1, generatedNameBytesGo1_27, generatedNameBytesGo1_27 + 1, 32769} {
		t.Run("width "+strconv.Itoa(length), func(t *testing.T) {
			t.Parallel()
			text := strings.Repeat("a", length)
			got, err := ParseGeneratedName(CacheFormatGo1_27, ArtifactCorpus, text)
			if length == generatedNameBytesGo1_27 {
				if err != nil || got.Validate() != nil || got.String() != text {
					t.Fatalf("exact width=%+v/%v, want admitted name", got, err)
				}
			} else if !errors.Is(err, core.ErrFuzzArtifactFormat) || got != (GeneratedName{}) {
				t.Fatalf("width %d=%+v/%v, want zero format refusal", length, got, err)
			}
		})
	}
	for raw := range 256 {
		t.Run("kind "+strconv.Itoa(raw), func(t *testing.T) {
			t.Parallel()
			kind := ArtifactKind(raw)
			wantValid := kind == ArtifactCorpus || kind == ArtifactCrasher
			got, err := ParseGeneratedName(CacheFormatGo1_27, kind, strings.Repeat("0", generatedNameBytesGo1_27))
			if wantValid {
				if err != nil || got.Kind() != kind {
					t.Fatalf("kind=%v/%v, want %v/nil", got.Kind(), err, kind)
				}
			} else if !errors.Is(err, core.ErrFuzzArtifactContract) || got != (GeneratedName{}) {
				t.Fatalf("kind refusal=%+v/%v, want zero contract refusal", got, err)
			}
		})
	}
}

func TestCacheFormatGeneratedNameDigestProjectionLayerTriad(t *testing.T) {
	t.Parallel()
	// Every digest byte value at every position detects endian, nibble, prefix,
	// and accidental suffix inclusion mistakes without mirroring hex encoding.
	for position := range core.SHA256DigestBytes {
		t.Run("digest byte "+strconv.Itoa(position), func(t *testing.T) {
			t.Parallel()
			for value := range 256 {
				var raw [core.SHA256DigestBytes]byte
				raw[position] = byte(value)
				digest := core.NewSHA256Digest(raw)
				want := hex.EncodeToString(raw[:generatedNameBytesGo1_27/2])
				for _, kind := range []ArtifactKind{ArtifactCorpus, ArtifactCrasher} {
					got, err := CacheFormatGo1_27.GeneratedName(kind, digest)
					if err != nil || got.String() != want || got.Kind() != kind || got.Validate() != nil {
						t.Fatalf("digest byte %d=%d projection=%q/%v, want %q/nil", position, value, got.String(), err, want)
					}
				}
			}
		})
	}
	cases := []struct {
		name    string
		format  CacheFormat
		kind    ArtifactKind
		digest  core.SHA256Digest
		wantErr error
	}{
		{name: "unset digest cannot impersonate all-zero digest", format: CacheFormatGo1_27, kind: ArtifactCorpus, wantErr: core.ErrPrimitiveContract},
		{name: "unknown kind cannot produce a name", format: CacheFormatGo1_27, digest: core.NewSHA256Digest([core.SHA256DigestBytes]byte{}), wantErr: core.ErrFuzzArtifactContract},
		{name: "unknown format cannot produce a name", kind: ArtifactCorpus, digest: core.NewSHA256Digest([core.SHA256DigestBytes]byte{}), wantErr: core.ErrFuzzArtifactFormat},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := tc.format.GeneratedName(tc.kind, tc.digest)
			if !errors.Is(err, tc.wantErr) || got != (GeneratedName{}) {
				t.Fatalf("GeneratedName=%+v/%v, want zero and %v", got, err, tc.wantErr)
			}
		})
	}
}

func BenchmarkCacheFormatGeneratedName(b *testing.B) {
	digest := core.NewSHA256Digest([core.SHA256DigestBytes]byte{0: 1, 7: 255, 31: 127})
	b.ReportAllocs()
	for _, kind := range []ArtifactKind{ArtifactCorpus, ArtifactCrasher} {
		b.Run(kind.String(), func(b *testing.B) {
			var got GeneratedName
			var err error
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				got, err = CacheFormatGo1_27.GeneratedName(kind, digest)
			}
			if err != nil || got.Validate() != nil {
				b.Fatalf("GeneratedName=%q/%v, want valid name and nil", got.String(), err)
			}
		})
	}
}

func generatedNameForPosition(t testing.TB, kind ArtifactKind, position uint64) GeneratedName {
	t.Helper()
	var raw [core.SHA256DigestBytes]byte
	for index := generatedNameBytesGo1_27 / 2; index > 0 && position != 0; index-- {
		raw[index-1] = byte(position)
		position >>= 8
	}
	got, err := CacheFormatGo1_27.GeneratedName(kind, core.NewSHA256Digest(raw))
	if err != nil {
		t.Fatalf("GeneratedName=%+v/%v, want validated name", got, err)
	}
	return got
}
