package garble_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"testing"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/garble"
)

func FuzzSeedJSONAgainstDirectStandardLibraryOracle(f *testing.F) {
	exactMaximum := append(
		bytes.Repeat([]byte{' '}, garble.SeedJSONWhitespaceAllowanceBytes),
		[]byte(`"AQIDBAUGBwg"`)...,
	)
	seeds := [][]byte{
		[]byte(`"AAAAAAAAAAA"`),
		[]byte(`"AQIDBAUGBwg"`),
		[]byte(`"//////////8"`),
		[]byte(`"__________8"`),
		[]byte(" \n\t\"AQIDBAUGBwg\"\r "),
		[]byte(`"\u0041QIDBAUGBwg"`),
		[]byte(`"AQIDBAUGBw\/"`),
		[]byte(`"AQIDBAUGBwg="`),
		[]byte(`"AQIDBAUGBwi"`),
		[]byte(`"AQIDBAUGBw"`),
		[]byte(`"AQIDBAUGBwgg"`),
		[]byte(`"AQID-BAUGBw"`),
		[]byte(`"AQID_BAUGBw"`),
		[]byte(`""`),
		[]byte(`null`),
		[]byte(`true`),
		[]byte(`1`),
		[]byte(`{}`),
		[]byte(`[]`),
		[]byte(`"AQIDBAUGBwg" false`),
		[]byte(`"AQIDBAUGBwg`),
		{0xff},
		exactMaximum,
		append(bytes.Clone(exactMaximum), ' '),
	}
	for _, seed := range seeds {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		wantRaw, wantAccept := oracleSeedJSON(data)
		before := garble.NewSeed([garble.SeedBytes]byte{1, 2, 3, 4, 5, 6, 7, 8})
		got := before
		gotErr := got.UnmarshalJSON(data)
		if !wantAccept {
			proveSeedJSONFuzzRejection(t, data, before, got, gotErr)
			return
		}
		proveSeedJSONFuzzAcceptance(t, data, got, gotErr, wantRaw)
	})
}

func proveSeedJSONFuzzRejection(
	t *testing.T,
	data []byte,
	before garble.Seed,
	got garble.Seed,
	gotErr error,
) {
	t.Helper()

	if !errors.Is(gotErr, core.ErrJSONContract) ||
		!errors.Is(gotErr, core.ErrGarbleContract) ||
		!errors.Is(gotErr, core.ErrPrimitiveContract) ||
		got != before {
		t.Fatalf(
			"Seed.UnmarshalJSON(%q) = (%v, %v), want preserved receiver and %v/%v/%v",
			data,
			got,
			gotErr,
			core.ErrJSONContract,
			core.ErrGarbleContract,
			core.ErrPrimitiveContract,
		)
	}
}

func proveSeedJSONFuzzAcceptance(
	t *testing.T,
	data []byte,
	got garble.Seed,
	gotErr error,
	wantRaw [garble.SeedBytes]byte,
) {
	t.Helper()

	if gotErr != nil {
		t.Fatalf("Seed.UnmarshalJSON(%q) error = %v, want nil", data, gotErr)
	}
	gotRaw, gotRawErr := got.Bytes()
	if gotRawErr != nil || gotRaw != wantRaw {
		t.Fatalf("accepted Seed.Bytes() = (%v, %v), want (%v, nil)", gotRaw, gotRawErr, wantRaw)
	}
	canonical, gotMarshalErr := got.MarshalJSON()
	if gotMarshalErr != nil || len(canonical) != garble.SeedCanonicalJSONBytes {
		t.Fatalf(
			"accepted Seed.MarshalJSON() = (%q, %v), want %d bytes",
			canonical,
			gotMarshalErr,
			garble.SeedCanonicalJSONBytes,
		)
	}
	proveSeedJSONFuzzCanonical(t, got, canonical, wantRaw)
}

func proveSeedJSONFuzzCanonical(
	t *testing.T,
	got garble.Seed,
	canonical []byte,
	wantRaw [garble.SeedBytes]byte,
) {
	t.Helper()

	gotCanonicalRaw, gotCanonicalAccept := oracleSeedJSON(canonical)
	if !gotCanonicalAccept || gotCanonicalRaw != wantRaw {
		t.Fatalf(
			"standard-library oracle for canonical Seed = (%v, %t), want (%v, true)",
			gotCanonicalRaw,
			gotCanonicalAccept,
			wantRaw,
		)
	}
	var gotRoundTrip garble.Seed
	gotRoundTripErr := gotRoundTrip.UnmarshalJSON(canonical)
	if gotRoundTripErr != nil || gotRoundTrip != got {
		t.Fatalf(
			"canonical Seed round trip = (%v, %v), want (%v, nil)",
			gotRoundTrip,
			gotRoundTripErr,
			got,
		)
	}
}

func oracleSeedJSON(data []byte) ([garble.SeedBytes]byte, bool) {
	var fixed [garble.SeedBytes]byte
	if len(data) == 0 || len(data) > garble.SeedJSONMaximumBytes ||
		!utf8.Valid(data) {
		return fixed, false
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	var encoded string
	if err := decoder.Decode(&encoded); err != nil {
		return fixed, false
	}
	if _, err := decoder.Token(); !errors.Is(err, io.EOF) {
		return fixed, false
	}
	raw, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil || len(raw) != garble.SeedBytes ||
		base64.RawStdEncoding.EncodeToString(raw) != encoded {
		return fixed, false
	}
	copy(fixed[:], raw)
	return fixed, true
}
