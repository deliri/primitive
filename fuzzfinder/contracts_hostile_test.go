package fuzzfinder

import (
	"encoding/hex"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"math"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestCacheFormatExhaustsClosedDomainAndPinsGeneratedNameStorage(t *testing.T) {
	t.Parallel()

	// The storage width is the ratchet, not the constant. Asserting the declared
	// width against generatedNameBytesGo1_26 would compare the constant to
	// itself; comparing it to the fixed array a parsed name is stored in means a
	// second format whose names are not exactly this wide fails here instead of
	// being silently truncated or zero-padded by ParseGeneratedName.
	storageWidth := uint64(len(GeneratedName{}.value))
	unknownLabel := CacheFormatUnknown.String()
	labels := make(map[string]CacheFormat, 1)
	for raw := range 256 {
		format := CacheFormat(raw)
		gotErr := format.Validate()
		wantValid := format == CacheFormatGo1_26
		if (gotErr == nil) != wantValid || format.IsValid() != wantValid {
			t.Fatalf("CacheFormat(%d) validity = Validate:%v IsValid:%t, want %t", raw, gotErr, format.IsValid(), wantValid)
		}
		gotWidth, widthErr := format.GeneratedNameBytes()
		if !wantValid {
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
		gotWidthValue, gotWidthErr := gotWidth.Uint64()
		if widthErr != nil || gotWidthErr != nil || gotWidthValue != storageWidth {
			t.Fatalf("CacheFormat(%d).GeneratedNameBytes() = (%d, %v/%v), want (%d, nil)", raw, gotWidthValue, widthErr, gotWidthErr, storageWidth)
		}
	}
	if _, implemented := any(CacheFormatGo1_26).(json.Marshaler); implemented {
		t.Fatalf("%T implements json.Marshaler, want an off-wire enum", CacheFormatGo1_26)
	}
	format := CacheFormatGo1_26
	if _, implemented := any(&format).(json.Unmarshaler); implemented {
		t.Fatalf("%T implements json.Unmarshaler, want an off-wire enum", &format)
	}
}

func TestParseGeneratedNameHostileBoundaryTable(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		value     string
		wantValid bool
	}{
		{name: "all decimal digits", value: "0123456789012345", wantValid: true},
		{name: "all lower hexadecimal letters", value: "abcdefabcdefabcd", wantValid: true},
		{name: "mixed lower hexadecimal", value: "0123456789abcdef", wantValid: true},
		{name: "leading zero retained", value: "0000000000000001", wantValid: true},
		{name: "trailing zero retained", value: "1000000000000000", wantValid: true},
		{name: "all zero identity", value: "0000000000000000", wantValid: true},
		{name: "all f identity", value: "ffffffffffffffff", wantValid: true},
		{name: "alternating digits and letters", value: "0a1b2c3d4e5f6789", wantValid: true},
		{name: "repeated digest prefix", value: "deadbeefdeadbeef", wantValid: true},
		{name: "numeric upper edge", value: "9999999999999999", wantValid: true},
		{name: "empty input", value: ""},
		{name: "one byte", value: "0"},
		{name: "one below exact width", value: strings.Repeat("0", generatedNameBytesGo1_26-1)},
		{name: "one above exact width", value: strings.Repeat("0", generatedNameBytesGo1_26+1)},
		{name: "archive eight byte guess", value: "0123abcd"},
		{name: "archive sixty four byte guess", value: strings.Repeat("a", 64)},
		{name: "uppercase hexadecimal", value: "0123456789abcdeF"},
		{name: "non hexadecimal letter", value: "0123456789abcdeg"},
		{name: "path separator", value: "01234567/9abcdef"},
		{name: "dot separator", value: "01234567.9abcdef"},
		{name: "space", value: "01234567 9abcdef"},
		{name: "newline", value: "0123456789abcde\n"},
		{name: "nul byte", value: "0123456789abcde\x00"},
		{name: "non ASCII rune", value: "0123456789abcdé"},
		{name: "leading space padding to exact width", value: " 123456789abcdef"},
		{name: "trailing space padding to exact width", value: "0123456789abcde "},
		{name: "hexadecimal prefix consuming the width", value: "0x23456789abcdef"},
		{name: "negative sign at exact width", value: "-123456789abcdef"},
		{name: "underscore digit separator", value: "0123456789ab_def"},
		{name: "current directory reference", value: "0123456789abcd.."},
		{name: "uppercase throughout", value: "0123456789ABCDEF"},
		{name: "utf-8 continuation bytes only", value: "\x80\x80\x80\x80\x80\x80\x80\x80\x80\x80\x80\x80\x80\x80\x80\x80"},
		{name: "nul padding to exact width", value: "\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00"},
		{name: "delete byte at exact width", value: "0123456789abcde\x7f"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := ParseGeneratedName(CacheFormatGo1_26, tc.value)
			if tc.wantValid {
				if gotErr != nil || got.String() != tc.value || got.Validate() != nil {
					t.Fatalf("ParseGeneratedName(%q) = (%q, %v), want (%q, nil)", tc.value, got.String(), gotErr, tc.value)
				}
				return
			}
			if !errors.Is(gotErr, core.ErrFuzzFinderFormat) || got != (GeneratedName{}) {
				t.Fatalf("ParseGeneratedName(%q) = (%q, %v), want zero and %v", tc.value, got.String(), gotErr, core.ErrFuzzFinderFormat)
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
	for _, seed := range []string{"", "0123456789abcdef", "0123456789abcdeF", strings.Repeat("a", 15), strings.Repeat("f", 17)} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, value string) {
		got, gotErr := ParseGeneratedName(CacheFormatGo1_26, value)
		// The oracle is differential, not a second copy of the production
		// predicate. The Go toolchain writes these names with %x over a digest
		// prefix, so the admitted set is exactly the strings encoding/hex both
		// decodes and re-emits unchanged: hex.DecodeString alone would admit
		// uppercase, and re-encoding is what pins the lowercase rule. Restating
		// production's own byte range here would mirror a wrong range instead of
		// contradicting it.
		decoded, decodeErr := hex.DecodeString(value)
		wantValid := decodeErr == nil &&
			len(decoded) == generatedNameBytesGo1_26/2 &&
			hex.EncodeToString(decoded) == value
		if wantValid {
			if gotErr != nil || got.String() != value || got.Validate() != nil {
				t.Fatalf("ParseGeneratedName(%q) = (%q, %v), want exact validated closure", value, got.String(), gotErr)
			}
			return
		}
		if !errors.Is(gotErr, core.ErrFuzzFinderFormat) || got != (GeneratedName{}) {
			t.Fatalf("ParseGeneratedName(%q) = (%q, %v), want zero and %v", value, got.String(), gotErr, core.ErrFuzzFinderFormat)
		}
	})
}

func generatedNameForPosition(t testing.TB, position uint64) GeneratedName {
	t.Helper()
	value := fmt.Sprintf("%016x", position)
	got, err := ParseGeneratedName(CacheFormatGo1_26, value)
	if err != nil {
		t.Fatalf("ParseGeneratedName(%q) error = %v, want nil", value, err)
	}
	return got
}
