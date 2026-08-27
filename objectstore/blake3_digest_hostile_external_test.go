package objectstore_test

import (
	"encoding/hex"
	json "encoding/json/v2"
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/objectstore"
)

func TestBLAKE3DigestCanonicalExternalBoundaryTable(t *testing.T) {
	t.Parallel()

	valid := []struct {
		name string
		raw  [objectstore.BLAKE3DigestBytes]byte
	}{
		{name: "constructed all-zero bytes remain a set digest"},
		{name: "first byte is one", raw: blake3DigestWithByte(0, 1)},
		{name: "last byte is one", raw: blake3DigestWithByte(objectstore.BLAKE3DigestBytes-1, 1)},
		{name: "first byte is maximum", raw: blake3DigestWithByte(0, 0xff)},
		{name: "last byte is maximum", raw: blake3DigestWithByte(objectstore.BLAKE3DigestBytes-1, 0xff)},
		{name: "alternating zero and maximum bytes", raw: blake3DigestAlternating(0, 0xff)},
		{name: "alternating bit patterns", raw: blake3DigestAlternating(0x55, 0xaa)},
		{name: "ascending byte sequence", raw: blake3DigestSequence(false)},
		{name: "descending byte sequence", raw: blake3DigestSequence(true)},
		{name: "official empty input digest", raw: mustBLAKE3Vector(t, "af1349b9f5f9a1a6a0404dea36dcc9499bcb25c9adc112b7cc9a93cae41f3262")},
	}
	for _, tc := range valid {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := objectstore.NewBLAKE3Digest(tc.raw)
			gotBytes, gotBytesErr := got.Bytes()
			gotHex, gotHexErr := got.Hex()
			wantHex := hex.EncodeToString(tc.raw[:])
			if got.Validate() != nil || gotBytesErr != nil || gotBytes != tc.raw || gotHexErr != nil || gotHex != wantHex {
				t.Fatalf("BLAKE3 digest projection = (bytes %x, hex %q, errors %v/%v), want (%x, %q, nil/nil)", gotBytes, gotHex, gotBytesErr, gotHexErr, tc.raw, wantHex)
			}

			var gotText objectstore.BLAKE3Digest
			gotTextErr := gotText.UnmarshalText([]byte(wantHex))
			gotJSON, gotJSONErr := json.Marshal(got)
			var gotJSONRoundTrip objectstore.BLAKE3Digest
			gotJSONRoundTripErr := json.Unmarshal(gotJSON, &gotJSONRoundTrip)
			if gotTextErr != nil || gotText != got || gotJSONErr != nil || gotJSONRoundTripErr != nil || gotJSONRoundTrip != got {
				t.Fatalf("BLAKE3 canonical round trips = (text %v/%v, JSON %v/%v), want (%v/nil, %v/nil)", gotText, gotTextErr, gotJSONRoundTrip, gotJSONRoundTripErr, got, got)
			}
		})
	}

	canonicalZero := strings.Repeat("0", objectstore.BLAKE3DigestBytes*2)
	invalid := []struct {
		name string
		wire string
	}{
		{name: "zero nibbles is below the exact extent"},
		{name: "one nibble is below the exact extent", wire: "0"},
		{name: "one byte is below the exact extent", wire: "00"},
		{name: "two nibbles below the exact extent", wire: canonicalZero[:len(canonicalZero)-2]},
		{name: "one nibble below the exact extent", wire: canonicalZero[:len(canonicalZero)-1]},
		{name: "one nibble above the exact extent", wire: canonicalZero + "0"},
		{name: "two nibbles above the exact extent", wire: canonicalZero + "00"},
		{name: "one nibble below twice the exact extent", wire: strings.Repeat("0", len(canonicalZero)*2-1)},
		{name: "twice the exact extent is rejected", wire: canonicalZero + canonicalZero},
		{name: "one nibble above twice the exact extent", wire: strings.Repeat("0", len(canonicalZero)*2+1)},
		{name: "four kilobytes is rejected before decoding", wire: strings.Repeat("0", 4<<10)},
		{name: "uppercase first nibble is not canonical", wire: "A" + canonicalZero[1:]},
		{name: "uppercase final nibble is not canonical", wire: canonicalZero[:len(canonicalZero)-1] + "F"},
		{name: "uppercase middle nibble is not canonical", wire: canonicalZero[:31] + "B" + canonicalZero[32:]},
		{name: "nonhex letter at first nibble is rejected", wire: "g" + canonicalZero[1:]},
		{name: "nonhex letter at middle nibble is rejected", wire: canonicalZero[:31] + "g" + canonicalZero[32:]},
		{name: "slash at first nibble is rejected", wire: "/" + canonicalZero[1:]},
		{name: "colon at final nibble is rejected", wire: canonicalZero[:len(canonicalZero)-1] + ":"},
		{name: "hex prefix is not part of the canonical form", wire: "0x" + canonicalZero[2:]},
		{name: "explicit positive sign is rejected", wire: "+" + canonicalZero[1:]},
		{name: "explicit negative sign is rejected", wire: "-" + canonicalZero[1:]},
		{name: "space prefix is rejected", wire: " " + canonicalZero},
		{name: "space suffix is rejected", wire: canonicalZero + " "},
		{name: "embedded space is rejected", wire: canonicalZero[:32] + " " + canonicalZero[33:]},
		{name: "newline prefix is rejected", wire: "\n" + canonicalZero},
		{name: "newline suffix is rejected", wire: canonicalZero + "\n"},
		{name: "embedded tab is rejected", wire: canonicalZero[:32] + "\t" + canonicalZero[33:]},
		{name: "NUL prefix is rejected", wire: "\x00" + canonicalZero[1:]},
		{name: "Unicode fullwidth zero is rejected", wire: "０" + canonicalZero[1:]},
		{name: "emoji prefix is rejected", wire: "🧪" + canonicalZero[1:]},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			preserved := objectstore.NewBLAKE3Digest(blake3DigestWithByte(0, 1))
			gotText := preserved
			gotTextErr := gotText.UnmarshalText([]byte(tc.wire))
			if !errors.Is(gotTextErr, core.ErrObjectStoreContract) || !errors.Is(gotTextErr, core.ErrPrimitiveContract) || gotText != preserved {
				t.Fatalf("BLAKE3Digest.UnmarshalText(%q) = (%v, %v), want preserved value with %v and %v", tc.wire, gotText, gotTextErr, core.ErrObjectStoreContract, core.ErrPrimitiveContract)
			}

			encoded, setupErr := json.Marshal(tc.wire)
			if setupErr != nil {
				t.Fatalf("json.Marshal(hostile BLAKE3 text) setup error = %v, want nil", setupErr)
			}
			gotJSON := preserved
			gotJSONErr := json.Unmarshal(encoded, &gotJSON)
			if !errors.Is(gotJSONErr, core.ErrJSONContract) || !errors.Is(gotJSONErr, core.ErrObjectStoreContract) || gotJSON != preserved {
				t.Fatalf("BLAKE3Digest.UnmarshalJSON(%q) = (%v, %v), want preserved value with %v and %v", encoded, gotJSON, gotJSONErr, core.ErrJSONContract, core.ErrObjectStoreContract)
			}
		})
	}
}

func TestBLAKE3DigestJSONFramingBoundaryTable(t *testing.T) {
	t.Parallel()

	canonical := strings.Repeat("0", objectstore.BLAKE3DigestBytes*2)
	cases := []struct {
		name string
		wire []byte
	}{
		{name: "nil document is rejected"},
		{name: "empty document is rejected", wire: []byte{}},
		{name: "null is rejected", wire: []byte(`null`)},
		{name: "number is rejected", wire: []byte(`0`)},
		{name: "boolean is rejected", wire: []byte(`true`)},
		{name: "array is rejected", wire: []byte(`[]`)},
		{name: "object is rejected", wire: []byte(`{}`)},
		{name: "unterminated string is rejected", wire: []byte(`"`)},
		{name: "trailing number is rejected", wire: []byte(`"` + canonical + `" 0`)},
		{name: "trailing string is rejected", wire: []byte(`"` + canonical + `" "x"`)},
		{name: "leading whitespace and trailing data is rejected", wire: []byte(" \t\n\"" + canonical + "\" false")},
		{name: "invalid UTF-8 string is rejected", wire: []byte{'"', 0xff, '"'}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			want := objectstore.NewBLAKE3Digest(blake3DigestWithByte(0, 1))
			got := want
			gotErr := got.UnmarshalJSON(tc.wire)
			if !errors.Is(gotErr, core.ErrJSONContract) || !errors.Is(gotErr, core.ErrObjectStoreContract) || got != want {
				t.Fatalf("BLAKE3Digest.UnmarshalJSON(%q) = (%v, %v), want preserved %v with %v and %v", tc.wire, got, gotErr, want, core.ErrJSONContract, core.ErrObjectStoreContract)
			}
		})
	}
}

func TestBLAKE3DigestSchemaLayerTriadPreservesContentIdentity(t *testing.T) {
	t.Parallel()

	t.Run("positive canonical content identity crosses both external forms unchanged", func(t *testing.T) {
		t.Parallel()

		want := objectstore.NewBLAKE3Digest(mustBLAKE3Vector(t, "6437b3ac38465133ffb63b75273a8db548c558465d79db03fd359c6cd5bd9d85"))
		wire, gotMarshalErr := want.MarshalJSON()
		var got objectstore.BLAKE3Digest
		gotUnmarshalErr := got.UnmarshalJSON(wire)
		if gotMarshalErr != nil || gotUnmarshalErr != nil || got != want || got.Validate() != nil {
			t.Fatalf("BLAKE3 JSON boundary = (%v, marshal %v, unmarshal %v), want (%v, nil, nil)", got, gotMarshalErr, gotUnmarshalErr, want)
		}
	})

	t.Run("negative malformed content identity preserves the populated receiver", func(t *testing.T) {
		t.Parallel()

		want := objectstore.NewBLAKE3Digest(blake3DigestWithByte(objectstore.BLAKE3DigestBytes-1, 1))
		got := want
		gotErr := got.UnmarshalText([]byte(strings.Repeat("f", objectstore.BLAKE3DigestBytes*2-1)))
		if !errors.Is(gotErr, core.ErrObjectStoreContract) || !errors.Is(gotErr, core.ErrPrimitiveContract) || got != want {
			t.Fatalf("BLAKE3 malformed boundary = (%v, %v), want preserved %v with typed contract identities", got, gotErr, want)
		}
	})

	t.Run("neutral unset identity emits no bytes or external projection", func(t *testing.T) {
		t.Parallel()

		got := objectstore.BLAKE3Digest{}
		gotBytes, gotBytesErr := got.Bytes()
		gotHex, gotHexErr := got.Hex()
		gotJSON, gotJSONErr := got.MarshalJSON()
		if !errors.Is(gotBytesErr, core.ErrObjectStoreContract) || gotBytes != ([objectstore.BLAKE3DigestBytes]byte{}) || !errors.Is(gotHexErr, core.ErrObjectStoreContract) || gotHex != "" || !errors.Is(gotJSONErr, core.ErrJSONContract) || gotJSON != nil {
			t.Fatalf("zero BLAKE3 projections = (bytes %x/%v, hex %q/%v, JSON %q/%v), want zero outputs with typed contract identities", gotBytes, gotBytesErr, gotHex, gotHexErr, gotJSON, gotJSONErr)
		}
	})
}

func blake3DigestWithByte(index int, value byte) [objectstore.BLAKE3DigestBytes]byte {
	var digest [objectstore.BLAKE3DigestBytes]byte
	digest[index] = value
	return digest
}

func blake3DigestAlternating(first, second byte) [objectstore.BLAKE3DigestBytes]byte {
	var digest [objectstore.BLAKE3DigestBytes]byte
	for index := range digest {
		if index%2 == 0 {
			digest[index] = first
			continue
		}
		digest[index] = second
	}
	return digest
}

func blake3DigestSequence(descending bool) [objectstore.BLAKE3DigestBytes]byte {
	var digest [objectstore.BLAKE3DigestBytes]byte
	for index := range digest {
		if descending {
			digest[index] = byte(objectstore.BLAKE3DigestBytes - index)
			continue
		}
		digest[index] = byte(index + 1)
	}
	return digest
}

func mustBLAKE3Vector(t testing.TB, value string) [objectstore.BLAKE3DigestBytes]byte {
	t.Helper()

	decoded, err := hex.DecodeString(value)
	if err != nil || len(decoded) != objectstore.BLAKE3DigestBytes {
		t.Fatalf("BLAKE3 test vector decode = (%d bytes, %v), want (%d, nil)", len(decoded), err, objectstore.BLAKE3DigestBytes)
	}
	var digest [objectstore.BLAKE3DigestBytes]byte
	copy(digest[:], decoded)
	return digest
}
