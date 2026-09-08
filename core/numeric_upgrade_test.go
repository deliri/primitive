package core

import (
	"errors"
	"math"
	"strconv"
	"strings"
	"testing"
)

func TestNumericJSONRefusalCannotRetainUnboundedInput(t *testing.T) {
	t.Parallel()
	maximum := strconv.FormatUint(math.MaxUint64, 10)
	cases := []struct {
		name, wire string
		want       uint64
		wantErr    error
		wantNative error
	}{
		{name: "neutral/zero", wire: "0"},
		{name: "positive/maximum uint64", wire: maximum, want: math.MaxUint64},
		{name: "boundary/one digit below maximum width", wire: strings.Repeat("9", len(maximum)-1), want: 9999999999999999999},
		{name: "boundary/maximum width overflow retains Go range cause", wire: strings.Repeat("9", len(maximum)), wantErr: ErrJSONContract, wantNative: strconv.ErrRange},
		{name: "boundary/one digit over maximum width", wire: strings.Repeat("9", len(maximum)+1), wantErr: ErrJSONContract},
		{name: "negative/megabyte overflow must not survive in NumError", wire: strings.Repeat("9", JSONDocumentMaximumBytes), wantErr: ErrJSONContract},
		{name: "negative/megabyte zero prefix cannot consume unbounded parse work", wire: strings.Repeat("0", JSONDocumentMaximumBytes), wantErr: ErrJSONContract},
		{name: "negative/maximum width syntax retains native cause", wire: strings.Repeat("0", len(maximum)-1) + "x", wantErr: ErrJSONContract, wantNative: strconv.ErrSyntax},
		{name: "negative/short syntax retains native cause", wire: "x", wantErr: ErrJSONContract, wantNative: strconv.ErrSyntax},
		{name: "negative/empty input", wantErr: ErrJSONContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := parseCanonicalUint64JSON([]byte(tc.wire))
			if got != tc.want || !errors.Is(err, tc.wantErr) {
				t.Fatalf("parse %d bytes=%d, %v; want %d and %v", len(tc.wire), got, err, tc.want, tc.wantErr)
			}
			if tc.wantNative != nil && !errors.Is(err, tc.wantNative) {
				t.Fatalf("native error=%v; want %v", err, tc.wantNative)
			}
			if native, ok := errors.AsType[*strconv.NumError](err); ok && len(native.Num) > len(maximum) {
				t.Fatalf("refusal retains %d attacker bytes; want at most %d", len(native.Num), len(maximum))
			}
		})
	}
}

// The constructor's only invalid state is unset. Component boundaries are
// independently varied; the existing Compare table checks ordering precedence.
func TestReleaseVersionConstructorPreservesUint32Boundaries(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                string
		major, minor, patch uint32
	}{
		{name: "neutral/all zero"},
		{name: "major/first nonzero", major: 1}, {name: "minor/first nonzero", minor: 1}, {name: "patch/first nonzero", patch: 1},
		{name: "major/signed ceiling", major: math.MaxInt32}, {name: "minor/signed ceiling", minor: math.MaxInt32}, {name: "patch/signed ceiling", patch: math.MaxInt32},
		{name: "major/unsigned sign bit", major: math.MaxInt32 + 1}, {name: "minor/unsigned sign bit", minor: math.MaxInt32 + 1}, {name: "patch/unsigned sign bit", patch: math.MaxInt32 + 1},
		{name: "major/below maximum", major: math.MaxUint32 - 1}, {name: "minor/below maximum", minor: math.MaxUint32 - 1}, {name: "patch/below maximum", patch: math.MaxUint32 - 1},
		{name: "major/maximum", major: math.MaxUint32}, {name: "minor/maximum", minor: math.MaxUint32}, {name: "patch/maximum", patch: math.MaxUint32},
		{name: "boundary/all components reach maximum", major: math.MaxUint32, minor: math.MaxUint32, patch: math.MaxUint32},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := NewReleaseVersion(tc.major, tc.minor, tc.patch)
			want := strconv.FormatUint(uint64(tc.major), 10) + "." + strconv.FormatUint(uint64(tc.minor), 10) + "." + strconv.FormatUint(uint64(tc.patch), 10)
			if got.Validate() != nil || got.String() != want {
				t.Fatalf("version=%v; want complete constructor domain %q", got, want)
			}
			var parsed ReleaseVersion
			if err := parsed.UnmarshalText([]byte(want)); err != nil || parsed != got {
				t.Fatalf("text=%v, %v; want exact version %v", parsed, err, got)
			}
			same, err := got.Compare(parsed)
			if err != nil || same != ComparisonEqual {
				t.Fatalf("same=%v, %v; want equal", same, err)
			}
			absent := got
			absent.set = false
			if err := absent.Validate(); !errors.Is(err, ErrPrimitiveContract) {
				t.Fatalf("unset version=%v; want constructor refusal", err)
			}
			refused, err := absent.Compare(got)
			if refused != ComparisonUnknown || !errors.Is(err, ErrPrimitiveContract) {
				t.Fatalf("unset comparison=%v, %v; want unknown and refusal", refused, err)
			}
		})
	}
}
