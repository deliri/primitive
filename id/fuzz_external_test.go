package id

import (
	"bytes"
	"encoding/binary"
	json "encoding/json/v2"
	"errors"
	"math"
	"math/big"
	"strings"
	"testing"
	"uuid"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// Go parses and renders independently of ID's lexical gate.
func oracleUUID(text string) (uuid.UUID, bool) {
	value, err := uuid.Parse(text)
	return value, err == nil && value.String() == text && value[6]>>4 == 7 && value[8]>>6 == 2
}

// The alphabet is the wire contract; big.Int replaces production's shifts.
func oracleULID(text string) ([identityBytes]byte, bool) {
	var raw [identityBytes]byte
	if len(text) != ulidTextBytes {
		return raw, false
	}
	var number big.Int
	base := big.NewInt(int64(len(crockfordAlphabet)))
	for _, character := range []byte(text) {
		digit := strings.IndexByte(crockfordAlphabet, character)
		if digit < 0 {
			return raw, false
		}
		number.Mul(&number, base)
		number.Add(&number, big.NewInt(int64(digit)))
	}
	if number.Sign() == 0 || number.BitLen() > identityBytes*8 {
		return raw, false
	}
	number.FillBytes(raw[:])
	return raw, true
}

func oracleULIDText(raw [identityBytes]byte) string {
	var number big.Int
	number.SetBytes(raw[:])
	digits := number.Text(len(crockfordAlphabet))
	var text [ulidTextBytes]byte
	for i := range text {
		text[i] = crockfordAlphabet[0]
	}
	for i, c := range []byte(digits) {
		index := int(c - '0')
		if c >= 'a' {
			index = int(c-'a') + 10
		}
		text[len(text)-len(digits)+i] = crockfordAlphabet[index]
	}
	return string(text[:])
}

func identityRequestForTest(t testing.TB, nanos int64, entropy []byte) Request {
	t.Helper()
	instant, err := temporal.InstantFromNanoseconds(nanos).Time()
	if err != nil {
		t.Fatal(err)
	}
	observation, err := temporal.NewObservation(instant)
	if err != nil {
		t.Fatal(err)
	}
	material, err := core.NewSecretMaterial(entropy)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := material.Destroy(); err != nil {
			t.Error(err)
		}
	})
	return Request{Observation: observation, Entropy: material}
}

func identitySeedRequest(t testing.TB) Request {
	t.Helper()
	entropy := make([]byte, core.SecretMaterialMinimumBytes)
	entropy[entropyBytes-1] = 1
	return identityRequestForTest(t, int64(temporal.NanosecondsPerMillisecond), entropy)
}

func checkUUIDText(t testing.TB, text string) {
	t.Helper()
	want, valid := oracleUUID(text)
	got, err := ParseUUIDv7(text)
	if (err == nil) != valid {
		t.Fatalf("UUID admission for %q = %v, oracle valid=%t", text, err, valid)
	}
	if !valid {
		if got != (UUIDv7{}) || !errors.Is(err, core.ErrIDContract) {
			t.Fatalf("UUID refusal = (%v,%v), want zero/ID identity", got, err)
		}
		return
	}
	if got.value != want || got.String() != text || got.Validate() != nil {
		t.Fatalf("UUID projection = %v, want %v", got, want)
	}
}

func checkULIDText(t testing.TB, text string) {
	t.Helper()
	want, valid := oracleULID(text)
	got, err := ParseULID(text)
	if (err == nil) != valid {
		t.Fatalf("ULID admission for %q = %v, oracle valid=%t", text, err, valid)
	}
	if !valid {
		if got != (ULID{}) || !errors.Is(err, core.ErrIDContract) {
			t.Fatalf("ULID refusal = (%v,%v), want zero/ID identity", got, err)
		}
		return
	}
	raw, err := got.Bytes()
	if err != nil || raw != want || got.String() != oracleULIDText(want) {
		t.Fatalf("ULID projection = (%x,%v), want %x", raw, err, want)
	}
}

func FuzzParseUUIDv7(f *testing.F) {
	value, err := NewUUIDv7(identitySeedRequest(f))
	if err != nil {
		f.Fatal(err)
	}
	for _, text := range []string{value.String(), "ffffffff-ffff-7fff-bfff-ffffffffffff", "", strings.ToUpper(value.String()), value.String() + "0", value.String()[:uuidTextBytes-1]} {
		f.Add(text)
	}
	f.Fuzz(func(t *testing.T, text string) { checkUUIDText(t, text) })
}

func FuzzParseULID(f *testing.F) {
	value, err := NewULID(identitySeedRequest(f))
	if err != nil {
		f.Fatal(err)
	}
	for _, text := range []string{value.String(), "7" + strings.Repeat("Z", ulidTextBytes-1), "8" + strings.Repeat("0", ulidTextBytes-1), strings.Repeat("0", ulidTextBytes), "", strings.ToLower(value.String()), "01ARZ3NDEKTSV4RRFFQ69G5FAV"} {
		f.Add(text)
	}
	f.Fuzz(func(t *testing.T, text string) { checkULIDText(t, text) })
}

func checkUUIDv7JSON(t testing.TB, data []byte, survivor UUIDv7) {
	t.Helper()
	var text string
	var decodeErr error = core.ErrJSONContract
	if len(data) <= UUIDv7JSONMaximumBytes {
		decodeErr = json.Unmarshal(data, &text)
	}
	want, valid := oracleUUID(text)
	valid = valid && decodeErr == nil && len(data) <= UUIDv7JSONMaximumBytes
	for _, initial := range []UUIDv7{{}, survivor} {
		got := initial
		err := got.UnmarshalJSON(data)
		if (err == nil) != valid {
			t.Fatalf("UUIDv7 JSON admission = %v, oracle valid=%t", err, valid)
		}
		if !valid {
			if got != initial || !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, core.ErrIDContract) {
				t.Fatalf("UUIDv7 JSON refusal = (%v,%v), want unchanged/typed", got, err)
			}
			continue
		}
		encoded, err := got.MarshalJSON()
		canonical, canonicalErr := json.Marshal(want.String())
		if err != nil || canonicalErr != nil || got.value != want || !bytes.Equal(encoded, canonical) {
			t.Fatalf("UUIDv7 JSON projection = (%q,%v), want %q", encoded, err, canonical)
		}
	}
}

func FuzzUUIDv7JSON(f *testing.F) {
	survivor, err := NewUUIDv7(identitySeedRequest(f))
	if err != nil {
		f.Fatal(err)
	}
	wire, err := survivor.MarshalJSON()
	if err != nil {
		f.Fatal(err)
	}
	for _, data := range [][]byte{wire, nil, []byte("null"), []byte("{}"), append(bytes.Clone(wire), wire...), append(bytes.Repeat([]byte(" "), UUIDv7JSONMaximumBytes-len(wire)+1), wire...)} {
		f.Add(data)
	}
	f.Fuzz(func(t *testing.T, data []byte) { checkUUIDv7JSON(t, data, survivor) })
}

func checkULIDJSON(t testing.TB, data []byte, survivor ULID) {
	t.Helper()
	var text string
	var decodeErr error = core.ErrJSONContract
	if len(data) <= ULIDJSONMaximumBytes {
		decodeErr = json.Unmarshal(data, &text)
	}
	want, valid := oracleULID(text)
	valid = valid && decodeErr == nil && len(data) <= ULIDJSONMaximumBytes
	for _, initial := range []ULID{{}, survivor} {
		got := initial
		err := got.UnmarshalJSON(data)
		if (err == nil) != valid {
			t.Fatalf("ULID JSON admission = %v, oracle valid=%t", err, valid)
		}
		if !valid {
			if got != initial || !errors.Is(err, core.ErrJSONContract) || !errors.Is(err, core.ErrIDContract) {
				t.Fatalf("ULID JSON refusal = (%v,%v), want unchanged/typed", got, err)
			}
			continue
		}
		encoded, err := got.MarshalJSON()
		canonical, canonicalErr := json.Marshal(oracleULIDText(want))
		if err != nil || canonicalErr != nil || got.value != want || !bytes.Equal(encoded, canonical) {
			t.Fatalf("ULID JSON projection = (%q,%v), want %q", encoded, err, canonical)
		}
	}
}

func FuzzULIDJSON(f *testing.F) {
	survivor, err := NewULID(identitySeedRequest(f))
	if err != nil {
		f.Fatal(err)
	}
	wire, err := survivor.MarshalJSON()
	if err != nil {
		f.Fatal(err)
	}
	for _, data := range [][]byte{wire, nil, []byte("null"), []byte("{}"), append(bytes.Clone(wire), wire...), append(bytes.Repeat([]byte(" "), ULIDJSONMaximumBytes-len(wire)+1), wire...)} {
		f.Add(data)
	}
	f.Fuzz(func(t *testing.T, data []byte) { checkULIDJSON(t, data, survivor) })
}

func checkRawULID(t testing.TB, raw [identityBytes]byte) {
	t.Helper()
	input := raw
	got, err := NewULIDFromBytes(input)
	if raw == ([identityBytes]byte{}) {
		if got != (ULID{}) || !errors.Is(err, core.ErrIDContract) {
			t.Fatalf("zero bytes = (%v,%v), want refusal", got, err)
		}
		return
	}
	if err != nil {
		t.Fatal(err)
	}
	input[0] ^= 0xff
	back, err := got.Bytes()
	if err != nil || back != raw || got.String() != oracleULIDText(raw) {
		t.Fatalf("ULID bytes = (%x,%v), want %x", back, err, raw)
	}
	back[0] ^= 0xff
	again, err := got.Bytes()
	if err != nil || again != raw {
		t.Fatalf("ULID bytes alias = (%x,%v), want %x", again, err, raw)
	}
	parsed, err := ParseULID(oracleULIDText(raw))
	if err != nil || parsed != got {
		t.Fatalf("ULID text round trip = (%v,%v), want %v", parsed, err, got)
	}
}

func FuzzULIDFromBytes(f *testing.F) {
	for _, pair := range [][2]uint64{{0, 0}, {0, 1}, {1, 0}, {math.MaxUint64, math.MaxUint64}, {1 << 63, 1 << 63}} {
		f.Add(pair[0], pair[1])
	}
	f.Fuzz(func(t *testing.T, high, low uint64) {
		var raw [identityBytes]byte
		binary.BigEndian.PutUint64(raw[:8], high)
		binary.BigEndian.PutUint64(raw[8:], low)
		checkRawULID(t, raw)
	})
}

func FuzzIdentityRequest(f *testing.F) {
	entropy := make([]byte, core.SecretMaterialMinimumBytes)
	entropy[entropyBytes-1] = 1
	for _, ns := range []int64{math.MinInt64, -1, 0, 1, int64(temporal.NanosecondsPerMillisecond) - 1, int64(temporal.NanosecondsPerMillisecond), math.MaxInt64} {
		f.Add(ns, entropy, false)
	}
	f.Add(int64(0), entropy, true)
	f.Add(int64(0), []byte{}, false)
	for _, extent := range []int{core.SecretMaterialMinimumBytes - 1, core.SecretMaterialMinimumBytes + 1, core.SecretMaterialMaximumBytes, core.SecretMaterialMaximumBytes + 1} {
		seed := make([]byte, extent)
		seed[0] = 1
		f.Add(int64(0), seed, false)
	}
	zeroHead := make([]byte, core.SecretMaterialMinimumBytes)
	zeroHead[len(zeroHead)-1] = 1
	f.Add(int64(0), zeroHead, false)

	f.Fuzz(func(t *testing.T, nanos int64, raw []byte, destroyed bool) {
		instant, err := temporal.InstantFromNanoseconds(nanos).Time()
		if err != nil {
			t.Fatal(err)
		}
		observation, err := temporal.NewObservation(instant)
		if err != nil {
			t.Fatal(err)
		}
		material, materialErr := core.NewSecretMaterial(raw)
		if materialErr == nil {
			t.Cleanup(func() {
				if err := material.Destroy(); err != nil {
					t.Error(err)
				}
			})
			if destroyed {
				if err := material.Destroy(); err != nil {
					t.Fatal(err)
				}
			}
		}
		request := Request{Observation: observation, Entropy: material}
		valid := nanos >= 0 && len(raw) == core.SecretMaterialMinimumBytes && materialErr == nil && !destroyed
		validation := request.Validate()
		if (validation == nil) != valid {
			t.Fatalf("request admission = %v, want valid=%t", validation, valid)
		}
		u, ue := NewUUIDv7(request)
		l, le := NewULID(request)
		if !valid {
			if u != (UUIDv7{}) || l != (ULID{}) || !errors.Is(ue, core.ErrIDContract) || !errors.Is(le, core.ErrIDContract) || !errors.Is(validation, core.ErrIDContract) {
				t.Fatalf("request refusal = (%v,%v,%v,%v,%v)", u, ue, l, le, validation)
			}
			return
		}
		var expected [identityBytes]byte
		stamp := big.NewInt(nanos / int64(temporal.NanosecondsPerMillisecond))
		stamp.Lsh(stamp, uint(entropyBytes*8))
		var tail big.Int
		tail.SetBytes(raw[:entropyBytes])
		stamp.Or(stamp, &tail).FillBytes(expected[:])
		if expected == ([identityBytes]byte{}) {
			if l != (ULID{}) || !errors.Is(le, core.ErrIDContract) {
				t.Fatalf("unset ULID = (%v,%v), want refusal", l, le)
			}
		} else if le != nil || l.value != expected {
			t.Fatalf("ULID construction = (%x,%v), want %x", l.value, le, expected)
		}
		expected[6] = expected[6]&0x0f | 0x70
		expected[8] = expected[8]&0x3f | 0x80
		if ue != nil || u.value != uuid.UUID(expected) {
			t.Fatalf("UUID construction = (%x,%v), want %x", u.value, ue, expected)
		}
		retained, err := material.CopyBytes()
		if err != nil || !bytes.Equal(retained, raw) {
			t.Fatalf("construction mutated entropy: %v", err)
		}
		clear(retained)
	})
}
