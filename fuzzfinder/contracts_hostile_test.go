package fuzzfinder

import (
	"encoding/hex"
	json "encoding/json/v2"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestCacheFormatExhaustsClosedDomainAndPinsGeneratedNameStorage(t *testing.T) {
	t.Parallel()

	storageWidth := uint64(len(GeneratedName{}.value))
	unknownLabel := CacheFormatUnknown.String()
	labels := make(map[string]CacheFormat, 1)
	for raw := range 256 {
		format := CacheFormat(raw)
		gotErr := format.Validate()
		wantValid := format == CacheFormatGo1_27
		if (gotErr == nil) != wantValid || format.IsValid() != wantValid {
			t.Fatalf("CacheFormat(%d) validity = Validate:%v IsValid:%t, want %t", raw, gotErr, format.IsValid(), wantValid)
		}
		if !wantValid {
			_, widthErr := format.GeneratedNameBytes(ArtifactCorpus)
			if format.String() != unknownLabel {
				t.Fatalf("CacheFormat(%d).String() = %q, want unknown label %q", raw, format.String(), unknownLabel)
			}
			if !errors.Is(gotErr, core.ErrFuzzFinderFormat) {
				t.Fatalf("CacheFormat(%d).Validate() error = %v, want %v", raw, gotErr, core.ErrFuzzFinderFormat)
			}
			if !errors.Is(widthErr, core.ErrFuzzFinderFormat) {
				t.Fatalf("CacheFormat(%d).GeneratedNameBytes() error = %v, want %v", raw, widthErr, core.ErrFuzzFinderFormat)
			}
			continue
		}
		if label := format.String(); label == "" || label == unknownLabel {
			t.Fatalf("CacheFormat(%d).String() = %q, want an admitted diagnostic", raw, label)
		} else if prior, exists := labels[label]; exists {
			t.Fatalf("CacheFormat values %d and %d share label %q, want unique labels", prior, format, label)
		} else {
			labels[label] = format
		}
		corpusWidth, corpusErr := format.GeneratedNameBytes(ArtifactCorpus)
		crasherWidth, crasherErr := format.GeneratedNameBytes(ArtifactCrasher)
		gotCorpusWidth, gotCorpusErr := corpusWidth.Uint64()
		gotCrasherWidth, gotCrasherErr := crasherWidth.Uint64()
		if corpusErr != nil || crasherErr != nil || gotCorpusErr != nil || gotCrasherErr != nil {
			t.Fatalf("CacheFormat(%d) widths = corpus:%d/%v/%v crasher:%d/%v/%v, want valid widths", raw, gotCorpusWidth, corpusErr, gotCorpusErr, gotCrasherWidth, crasherErr, gotCrasherErr)
		}
		if gotCorpusWidth != storageWidth || gotCrasherWidth != storageWidth {
			t.Fatalf("CacheFormat(%d) widths = corpus:%d crasher:%d storage:%d, want both persisted kinds equal storage", raw, gotCorpusWidth, gotCrasherWidth, storageWidth)
		}
	}
	if _, implemented := any(CacheFormatGo1_27).(json.Marshaler); implemented {
		t.Fatalf("%T implements json.Marshaler, want an off-wire enum", CacheFormatGo1_27)
	}
	format := CacheFormatGo1_27
	if _, implemented := any(&format).(json.Unmarshaler); implemented {
		t.Fatalf("%T implements json.Unmarshaler, want an off-wire enum", &format)
	}
}

func TestParseGeneratedNameHostileBoundaryTable(t *testing.T) {
	t.Parallel()

	type caseClass uint8
	const (
		caseValid caseClass = iota + 1
		caseReject
		caseBoundary
	)
	width := func(kind ArtifactKind) int {
		got, err := CacheFormatGo1_27.GeneratedNameBytes(kind)
		if err != nil {
			t.Fatalf("CacheFormatGo1_27.GeneratedNameBytes(%d) error = %v, want nil", kind, err)
		}
		value, err := got.Uint64()
		if err != nil {
			t.Fatalf("generated-name width Uint64() error = %v, want nil", err)
		}
		return int(value)
	}
	hexValue := func(size int, value byte) string {
		return strings.Repeat(string([]byte{value}), size)
	}
	corpusWidth := width(ArtifactCorpus)
	crasherWidth := width(ArtifactCrasher)
	cases := []struct {
		wantErr error
		name    string
		value   string
		class   caseClass
		format  CacheFormat
		kind    ArtifactKind
	}{
		{name: "valid corpus zeros", class: caseValid, format: CacheFormatGo1_27, kind: ArtifactCorpus, value: hexValue(corpusWidth, '0')},
		{name: "valid corpus maximum", class: caseValid, format: CacheFormatGo1_27, kind: ArtifactCorpus, value: hexValue(corpusWidth, 'f')},
		{name: "valid corpus decimal", class: caseValid, format: CacheFormatGo1_27, kind: ArtifactCorpus, value: hexValue(corpusWidth, '9')},
		{name: "valid corpus alternating", class: caseValid, format: CacheFormatGo1_27, kind: ArtifactCorpus, value: strings.Repeat("0a", corpusWidth/2)},
		{name: "valid corpus digest prefix", class: caseValid, format: CacheFormatGo1_27, kind: ArtifactCorpus, value: strings.Repeat("deadbeef", corpusWidth/8)},
		{name: "valid crasher zeros", class: caseValid, format: CacheFormatGo1_27, kind: ArtifactCrasher, value: hexValue(crasherWidth, '0')},
		{name: "valid crasher maximum", class: caseValid, format: CacheFormatGo1_27, kind: ArtifactCrasher, value: hexValue(crasherWidth, 'f')},
		{name: "valid crasher decimal", class: caseValid, format: CacheFormatGo1_27, kind: ArtifactCrasher, value: hexValue(crasherWidth, '9')},
		{name: "valid crasher alternating", class: caseValid, format: CacheFormatGo1_27, kind: ArtifactCrasher, value: "0a1b2c3d4e5f6789"},
		{name: "valid crasher digest prefix", class: caseValid, format: CacheFormatGo1_27, kind: ArtifactCrasher, value: "deadbeefdeadbeef"},

		{name: "reject unknown kind", class: caseReject, format: CacheFormatGo1_27, value: hexValue(corpusWidth, '0'), wantErr: core.ErrFuzzFinderContract},
		{name: "reject future kind", class: caseReject, format: CacheFormatGo1_27, kind: ArtifactKind(255), value: hexValue(corpusWidth, '0'), wantErr: core.ErrFuzzFinderContract},
		{name: "reject unknown format", class: caseReject, kind: ArtifactCorpus, value: hexValue(corpusWidth, '0'), wantErr: core.ErrFuzzFinderFormat},
		{name: "reject future format", class: caseReject, format: CacheFormat(255), kind: ArtifactCrasher, value: hexValue(crasherWidth, '0'), wantErr: core.ErrFuzzFinderFormat},
		{name: "reject corpus uppercase", class: caseReject, format: CacheFormatGo1_27, kind: ArtifactCorpus, value: hexValue(corpusWidth, 'F'), wantErr: core.ErrFuzzFinderFormat},
		{name: "reject crasher nonhex", class: caseReject, format: CacheFormatGo1_27, kind: ArtifactCrasher, value: hexValue(crasherWidth, 'g'), wantErr: core.ErrFuzzFinderFormat},
		{name: "reject corpus path separator", class: caseReject, format: CacheFormatGo1_27, kind: ArtifactCorpus, value: "/" + hexValue(corpusWidth-1, '0'), wantErr: core.ErrFuzzFinderFormat},
		{name: "reject crasher nul", class: caseReject, format: CacheFormatGo1_27, kind: ArtifactCrasher, value: "\x00" + hexValue(crasherWidth-1, '0'), wantErr: core.ErrFuzzFinderFormat},
		{name: "reject corpus whitespace", class: caseReject, format: CacheFormatGo1_27, kind: ArtifactCorpus, value: " " + hexValue(corpusWidth-1, '0'), wantErr: core.ErrFuzzFinderFormat},
		{name: "reject crasher dot traversal", class: caseReject, format: CacheFormatGo1_27, kind: ArtifactCrasher, value: ".." + hexValue(crasherWidth-2, '0'), wantErr: core.ErrFuzzFinderFormat},

		{name: "boundary corpus empty", class: caseBoundary, format: CacheFormatGo1_27, kind: ArtifactCorpus, wantErr: core.ErrFuzzFinderFormat},
		{name: "boundary corpus one below", class: caseBoundary, format: CacheFormatGo1_27, kind: ArtifactCorpus, value: hexValue(corpusWidth-1, '0'), wantErr: core.ErrFuzzFinderFormat},
		{name: "boundary corpus exact", class: caseBoundary, format: CacheFormatGo1_27, kind: ArtifactCorpus, value: hexValue(corpusWidth, 'a')},
		{name: "boundary corpus one above", class: caseBoundary, format: CacheFormatGo1_27, kind: ArtifactCorpus, value: hexValue(corpusWidth+1, '0'), wantErr: core.ErrFuzzFinderFormat},
		{name: "boundary corpus double width", class: caseBoundary, format: CacheFormatGo1_27, kind: ArtifactCorpus, value: hexValue(corpusWidth*2, '0'), wantErr: core.ErrFuzzFinderFormat},
		{name: "boundary corpus final uppercase", class: caseBoundary, format: CacheFormatGo1_27, kind: ArtifactCorpus, value: hexValue(corpusWidth-1, '0') + "A", wantErr: core.ErrFuzzFinderFormat},
		{name: "boundary corpus final lowerhex", class: caseBoundary, format: CacheFormatGo1_27, kind: ArtifactCorpus, value: hexValue(corpusWidth-1, '0') + "f"},
		{name: "boundary corpus first lowerhex", class: caseBoundary, format: CacheFormatGo1_27, kind: ArtifactCorpus, value: "f" + hexValue(corpusWidth-1, '0')},
		{name: "boundary corpus embedded newline", class: caseBoundary, format: CacheFormatGo1_27, kind: ArtifactCorpus, value: hexValue(corpusWidth/2, '0') + "\n" + hexValue(corpusWidth/2-1, '0'), wantErr: core.ErrFuzzFinderFormat},
		{name: "boundary corpus nonascii exact bytes", class: caseBoundary, format: CacheFormatGo1_27, kind: ArtifactCorpus, value: "é" + hexValue(corpusWidth-2, '0'), wantErr: core.ErrFuzzFinderFormat},
		{name: "boundary crasher empty", class: caseBoundary, format: CacheFormatGo1_27, kind: ArtifactCrasher, wantErr: core.ErrFuzzFinderFormat},
		{name: "boundary crasher one below", class: caseBoundary, format: CacheFormatGo1_27, kind: ArtifactCrasher, value: hexValue(crasherWidth-1, '0'), wantErr: core.ErrFuzzFinderFormat},
		{name: "boundary crasher exact", class: caseBoundary, format: CacheFormatGo1_27, kind: ArtifactCrasher, value: hexValue(crasherWidth, 'a')},
		{name: "boundary crasher one above", class: caseBoundary, format: CacheFormatGo1_27, kind: ArtifactCrasher, value: hexValue(crasherWidth+1, '0'), wantErr: core.ErrFuzzFinderFormat},
		{name: "boundary crasher half width", class: caseBoundary, format: CacheFormatGo1_27, kind: ArtifactCrasher, value: hexValue(crasherWidth/2, '0'), wantErr: core.ErrFuzzFinderFormat},
		{name: "boundary crasher final uppercase", class: caseBoundary, format: CacheFormatGo1_27, kind: ArtifactCrasher, value: hexValue(crasherWidth-1, '0') + "A", wantErr: core.ErrFuzzFinderFormat},
		{name: "boundary crasher final lowerhex", class: caseBoundary, format: CacheFormatGo1_27, kind: ArtifactCrasher, value: hexValue(crasherWidth-1, '0') + "f"},
		{name: "boundary crasher first lowerhex", class: caseBoundary, format: CacheFormatGo1_27, kind: ArtifactCrasher, value: "f" + hexValue(crasherWidth-1, '0')},
		{name: "boundary crasher embedded newline", class: caseBoundary, format: CacheFormatGo1_27, kind: ArtifactCrasher, value: hexValue(crasherWidth/2, '0') + "\n" + hexValue(crasherWidth/2-1, '0'), wantErr: core.ErrFuzzFinderFormat},
		{name: "boundary crasher nonascii exact bytes", class: caseBoundary, format: CacheFormatGo1_27, kind: ArtifactCrasher, value: "é" + hexValue(crasherWidth-2, '0'), wantErr: core.ErrFuzzFinderFormat},
	}
	counts := [4]int{}
	for _, tc := range cases {
		counts[tc.class]++
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := ParseGeneratedName(tc.format, tc.kind, tc.value)
			if tc.wantErr == nil {
				if gotErr != nil || got.String() != tc.value || got.Validate() != nil || got.Kind() != tc.kind || got.Format() != tc.format {
					t.Fatalf("ParseGeneratedName(%q) = (%q, %v), want (%q, nil)", tc.value, got.String(), gotErr, tc.value)
				}
				return
			}
			if !errors.Is(gotErr, tc.wantErr) || got != (GeneratedName{}) {
				t.Fatalf("ParseGeneratedName(%q) = (%q, %v), want zero and %v", tc.value, got.String(), gotErr, tc.wantErr)
			}
		})
	}
	if counts[caseValid] != 10 || counts[caseReject] != 10 || counts[caseBoundary] != 20 {
		t.Fatalf("hostile case counts = valid:%d reject:%d boundary:%d, want 10/10/20", counts[caseValid], counts[caseReject], counts[caseBoundary])
	}
}

func TestCacheFormatGeneratedNameLayerTriadHostileTable(t *testing.T) {
	t.Parallel()

	type caseClass uint8
	const (
		caseValid caseClass = iota + 1
		caseReject
		caseBoundary
	)
	type testCase struct {
		wantErr error
		name    string
		digest  core.SHA256Digest
		class   caseClass
		format  CacheFormat
		kind    ArtifactKind
	}
	filled := func(value byte) core.SHA256Digest {
		var raw [core.SHA256DigestBytes]byte
		for index := range raw {
			raw[index] = value
		}
		return core.NewSHA256Digest(raw)
	}
	changed := func(index int, value byte) core.SHA256Digest {
		var raw [core.SHA256DigestBytes]byte
		raw[index] = value
		return core.NewSHA256Digest(raw)
	}
	cases := []testCase{
		{name: "valid corpus zero", class: caseValid, format: CacheFormatGo1_27, kind: ArtifactCorpus, digest: filled(0x00)},
		{name: "valid corpus one", class: caseValid, format: CacheFormatGo1_27, kind: ArtifactCorpus, digest: filled(0x01)},
		{name: "valid corpus low nibble", class: caseValid, format: CacheFormatGo1_27, kind: ArtifactCorpus, digest: filled(0x0f)},
		{name: "valid corpus high byte", class: caseValid, format: CacheFormatGo1_27, kind: ArtifactCorpus, digest: filled(0x80)},
		{name: "valid corpus maximum", class: caseValid, format: CacheFormatGo1_27, kind: ArtifactCorpus, digest: filled(0xff)},
		{name: "valid crasher zero", class: caseValid, format: CacheFormatGo1_27, kind: ArtifactCrasher, digest: filled(0x00)},
		{name: "valid crasher one", class: caseValid, format: CacheFormatGo1_27, kind: ArtifactCrasher, digest: filled(0x01)},
		{name: "valid crasher high nibble", class: caseValid, format: CacheFormatGo1_27, kind: ArtifactCrasher, digest: filled(0x10)},
		{name: "valid crasher signed edge", class: caseValid, format: CacheFormatGo1_27, kind: ArtifactCrasher, digest: filled(0x7f)},
		{name: "valid crasher maximum", class: caseValid, format: CacheFormatGo1_27, kind: ArtifactCrasher, digest: filled(0xff)},

		{name: "reject corpus unset digest", class: caseReject, format: CacheFormatGo1_27, kind: ArtifactCorpus, wantErr: core.ErrPrimitiveContract},
		{name: "reject crasher unset digest", class: caseReject, format: CacheFormatGo1_27, kind: ArtifactCrasher, wantErr: core.ErrPrimitiveContract},
		{name: "reject unknown kind", class: caseReject, format: CacheFormatGo1_27, digest: filled(1), wantErr: core.ErrFuzzFinderContract},
		{name: "reject first future kind", class: caseReject, format: CacheFormatGo1_27, kind: ArtifactCrasher + 1, digest: filled(1), wantErr: core.ErrFuzzFinderContract},
		{name: "reject maximum kind", class: caseReject, format: CacheFormatGo1_27, kind: ArtifactKind(255), digest: filled(1), wantErr: core.ErrFuzzFinderContract},
		{name: "reject corpus unknown format", class: caseReject, kind: ArtifactCorpus, digest: filled(1), wantErr: core.ErrFuzzFinderFormat},
		{name: "reject crasher unknown format", class: caseReject, kind: ArtifactCrasher, digest: filled(1), wantErr: core.ErrFuzzFinderFormat},
		{name: "reject corpus future format", class: caseReject, format: CacheFormatGo1_27 + 1, kind: ArtifactCorpus, digest: filled(1), wantErr: core.ErrFuzzFinderFormat},
		{name: "reject crasher middle format", class: caseReject, format: CacheFormat(128), kind: ArtifactCrasher, digest: filled(1), wantErr: core.ErrFuzzFinderFormat},
		{name: "reject crasher maximum format", class: caseReject, format: CacheFormat(255), kind: ArtifactCrasher, digest: filled(1), wantErr: core.ErrFuzzFinderFormat},

		{name: "boundary corpus first included", class: caseBoundary, format: CacheFormatGo1_27, kind: ArtifactCorpus, digest: changed(0, 1)},
		{name: "boundary corpus final included", class: caseBoundary, format: CacheFormatGo1_27, kind: ArtifactCorpus, digest: changed(7, 1)},
		{name: "boundary corpus first excluded", class: caseBoundary, format: CacheFormatGo1_27, kind: ArtifactCorpus, digest: changed(8, 1)},
		{name: "boundary corpus first excluded maximum", class: caseBoundary, format: CacheFormatGo1_27, kind: ArtifactCorpus, digest: changed(8, 255)},
		{name: "boundary corpus final excluded", class: caseBoundary, format: CacheFormatGo1_27, kind: ArtifactCorpus, digest: changed(31, 1)},
		{name: "boundary corpus included and excluded", class: caseBoundary, format: CacheFormatGo1_27, kind: ArtifactCorpus, digest: core.NewSHA256Digest([32]byte{7: 1, 8: 255})},
		{name: "boundary corpus maximum included", class: caseBoundary, format: CacheFormatGo1_27, kind: ArtifactCorpus, digest: changed(7, 255)},
		{name: "boundary corpus middle included", class: caseBoundary, format: CacheFormatGo1_27, kind: ArtifactCorpus, digest: changed(4, 1)},
		{name: "boundary corpus middle excluded", class: caseBoundary, format: CacheFormatGo1_27, kind: ArtifactCorpus, digest: changed(16, 1)},
		{name: "boundary corpus final excluded maximum", class: caseBoundary, format: CacheFormatGo1_27, kind: ArtifactCorpus, digest: changed(31, 255)},
		{name: "boundary crasher first included", class: caseBoundary, format: CacheFormatGo1_27, kind: ArtifactCrasher, digest: changed(0, 1)},
		{name: "boundary crasher final included", class: caseBoundary, format: CacheFormatGo1_27, kind: ArtifactCrasher, digest: changed(7, 1)},
		{name: "boundary crasher first excluded", class: caseBoundary, format: CacheFormatGo1_27, kind: ArtifactCrasher, digest: changed(8, 1)},
		{name: "boundary crasher first excluded maximum", class: caseBoundary, format: CacheFormatGo1_27, kind: ArtifactCrasher, digest: changed(8, 255)},
		{name: "boundary crasher final excluded", class: caseBoundary, format: CacheFormatGo1_27, kind: ArtifactCrasher, digest: changed(31, 1)},
		{name: "boundary crasher included and excluded", class: caseBoundary, format: CacheFormatGo1_27, kind: ArtifactCrasher, digest: core.NewSHA256Digest([32]byte{7: 1, 8: 255})},
		{name: "boundary crasher maximum included", class: caseBoundary, format: CacheFormatGo1_27, kind: ArtifactCrasher, digest: changed(7, 255)},
		{name: "boundary crasher middle included", class: caseBoundary, format: CacheFormatGo1_27, kind: ArtifactCrasher, digest: changed(4, 1)},
		{name: "boundary crasher middle excluded", class: caseBoundary, format: CacheFormatGo1_27, kind: ArtifactCrasher, digest: changed(16, 1)},
		{name: "boundary crasher final excluded maximum", class: caseBoundary, format: CacheFormatGo1_27, kind: ArtifactCrasher, digest: changed(31, 255)},
	}
	counts := [4]int{}
	for _, tc := range cases {
		counts[tc.class]++
	}
	if counts[caseValid] != 10 || counts[caseReject] != 10 || counts[caseBoundary] != 20 {
		t.Fatalf("hostile case counts = valid:%d reject:%d boundary:%d, want 10/10/20", counts[caseValid], counts[caseReject], counts[caseBoundary])
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := tc.format.GeneratedName(tc.kind, tc.digest)
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) || got != (GeneratedName{}) {
					t.Fatalf("CacheFormat.GeneratedName() = (%q, %v), want zero and %v", got.String(), gotErr, tc.wantErr)
				}
				return
			}
			width, widthErr := tc.format.GeneratedNameBytes(tc.kind)
			widthValue, uintErr := width.Uint64()
			digestBytes, digestErr := tc.digest.Bytes()
			if widthErr != nil || uintErr != nil || digestErr != nil {
				t.Fatalf("test oracle setup = width:%v uint:%v digest:%v, want nil", widthErr, uintErr, digestErr)
			}
			want := hex.EncodeToString(digestBytes[:widthValue/2])
			if gotErr != nil || got.String() != want || got.Validate() != nil || got.Kind() != tc.kind || got.Format() != tc.format {
				t.Fatalf("CacheFormat.GeneratedName() = (%q, %v), want (%q, nil)", got.String(), gotErr, want)
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
			var gotErr error
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				got, gotErr = CacheFormatGo1_27.GeneratedName(kind, digest)
			}
			if gotErr != nil || got.Validate() != nil {
				b.Fatalf("CacheFormat.GeneratedName() = (%q, %v), want valid and nil", got.String(), gotErr)
			}
		})
	}
}

func TestRetentionLimitPressureAcrossCompleteNumericDomain(t *testing.T) {
	t.Parallel()

	values := []uint16{0, 1, 2, MaximumRetainedEntries - 1, MaximumRetainedEntries, MaximumRetainedEntries + 1, math.MaxUint16}
	for _, value := range values {
		got, gotErr := NewRetentionLimit(value)
		wantValid := value > 0 && value <= MaximumRetainedEntries
		if (gotErr == nil) != wantValid {
			t.Fatalf("NewRetentionLimit(%d) = (%d, %v), want valid %t", value, got.Uint16(), gotErr, wantValid)
		}
		if !wantValid && (!errors.Is(gotErr, core.ErrFuzzFinderContract) || got != (RetentionLimit{})) {
			t.Fatalf("NewRetentionLimit(%d) = (%d, %v), want zero and %v", value, got.Uint16(), gotErr, core.ErrFuzzFinderContract)
		}
		if wantValid && got.Uint16() != value {
			t.Fatalf("NewRetentionLimit(%d).Uint16() = %d, want %d", value, got.Uint16(), value)
		}
	}
}

func FuzzParseGeneratedNameSemanticClosure(f *testing.F) {
	digest := core.NewSHA256Digest([core.SHA256DigestBytes]byte{0: 1, 3: 127, 7: 255, 31: 1})
	for _, kind := range []ArtifactKind{ArtifactCorpus, ArtifactCrasher} {
		name, err := CacheFormatGo1_27.GeneratedName(kind, digest)
		if err != nil || name.Validate() != nil {
			f.Fatalf("CacheFormat.GeneratedName(%d) = (%q, %v), want valid seed", kind, name.String(), err)
		}
		f.Add(uint8(kind), name.String())
	}
	for _, seed := range []string{"", "0123456789abcdeF", strings.Repeat("a", generatedNameBytesGo1_27-1), strings.Repeat("f", generatedNameBytesGo1_27+1)} {
		f.Add(uint8(ArtifactCorpus), seed)
	}
	f.Add(uint8(ArtifactUnknown), strings.Repeat("0", generatedNameBytesGo1_27))
	f.Fuzz(func(t *testing.T, rawKind uint8, value string) {
		kind := ArtifactKind(rawKind)
		got, gotErr := ParseGeneratedName(CacheFormatGo1_27, kind, value)
		// The oracle is differential, not a second copy of the production
		// predicate. The Go toolchain writes these names with %x over a digest
		// prefix, so the admitted set is exactly the strings encoding/hex both
		// decodes and re-emits unchanged: hex.DecodeString alone would admit
		// uppercase, and re-encoding is what pins the lowercase rule. Restating
		// production's own byte range here would mirror a wrong range instead of
		// contradicting it.
		width, widthErr := CacheFormatGo1_27.GeneratedNameBytes(kind)
		widthValue, uintErr := width.Uint64()
		decoded, decodeErr := hex.DecodeString(value)
		wantValid := widthErr == nil && uintErr == nil && decodeErr == nil &&
			uint64(len(decoded)) == widthValue/2 &&
			hex.EncodeToString(decoded) == value
		if wantValid {
			if gotErr != nil || got.String() != value || got.Validate() != nil || got.Kind() != kind || got.Format() != CacheFormatGo1_27 {
				t.Fatalf("ParseGeneratedName(%q) = (%q, %v), want exact validated closure", value, got.String(), gotErr)
			}
			roundTrip, roundTripErr := ParseGeneratedName(got.Format(), got.Kind(), got.String())
			if roundTripErr != nil || roundTrip != got {
				t.Fatalf("ParseGeneratedName(canonical) = (%q, %v), want (%q, nil)", roundTrip.String(), roundTripErr, got.String())
			}
			return
		}
		wantErr := core.ErrFuzzFinderFormat
		if widthErr != nil && errors.Is(widthErr, core.ErrFuzzFinderContract) {
			wantErr = core.ErrFuzzFinderContract
		}
		if !errors.Is(gotErr, wantErr) || got != (GeneratedName{}) {
			t.Fatalf("ParseGeneratedName(%q) = (%q, %v), want zero and %v", value, got.String(), gotErr, wantErr)
		}
	})
}

func generatedNameForPosition(t testing.TB, kind ArtifactKind, position uint64) GeneratedName {
	t.Helper()
	width, err := CacheFormatGo1_27.GeneratedNameBytes(kind)
	if err != nil {
		t.Fatalf("CacheFormat.GeneratedNameBytes(%d) error = %v, want nil", kind, err)
	}
	widthValue, err := width.Uint64()
	if err != nil {
		t.Fatalf("generated-name width Uint64() error = %v, want nil", err)
	}
	var raw [core.SHA256DigestBytes]byte
	for index := int(widthValue / 2); index > 0 && position != 0; index-- {
		raw[index-1] = byte(position)
		position >>= 8
	}
	got, err := CacheFormatGo1_27.GeneratedName(kind, core.NewSHA256Digest(raw))
	if err != nil {
		t.Fatalf("CacheFormat.GeneratedName(%d) error = %v, want nil", kind, err)
	}
	return got
}
