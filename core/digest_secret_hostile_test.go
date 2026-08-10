package core

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"testing"
	"time"
)

const (
	secretMaterialConcurrentWorkerCount = 8
	secretMaterialConcurrencyTimeout    = 10 * time.Second
)

type secretMaterialWorkerPhase uint8

const (
	secretMaterialWorkerPhaseUnknown secretMaterialWorkerPhase = iota
	secretMaterialWorkerPhaseActive
	secretMaterialWorkerPhaseDestroyed
)

type secretMaterialWorkerOutcome struct {
	gotErr   error
	gotBytes []byte
	worker   uint8
	phase    secretMaterialWorkerPhase
}

func TestSHA256DigestHostileCanonicalBoundaryTable(t *testing.T) {
	t.Parallel()

	valid := []struct {
		name string
		raw  [sha256.Size]byte
	}{
		{name: "all zero digest remains a set digest"},
		{name: "first byte set", raw: digestWithByte(0, 1)},
		{name: "last byte set", raw: digestWithByte(sha256.Size-1, 1)},
		{name: "first byte maximum", raw: digestWithByte(0, math.MaxUint8)},
		{name: "last byte maximum", raw: digestWithByte(sha256.Size-1, math.MaxUint8)},
		{name: "alternating low high bytes", raw: digestAlternating(0x00, 0xff)},
		{name: "alternating bit pattern", raw: digestAlternating(0x55, 0xaa)},
		{name: "ascending byte sequence", raw: digestSequence(false)},
		{name: "descending byte sequence", raw: digestSequence(true)},
		{name: "SHA256 of empty input", raw: sha256.Sum256(nil)},
	}
	for _, tc := range valid {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			value := NewSHA256Digest(tc.raw)
			if gotErr := value.Validate(); gotErr != nil {
				t.Fatalf("SHA256Digest.Validate() error = %v, want nil", gotErr)
			}
			gotBytes, gotBytesErr := value.Bytes()
			if gotBytesErr != nil || gotBytes != tc.raw {
				t.Fatalf("SHA256Digest.Bytes() = (%x, %v), want (%x, nil)", gotBytes, gotBytesErr, tc.raw)
			}
			gotHex, gotHexErr := value.Hex()
			wantHex := hex.EncodeToString(tc.raw[:])
			if gotHexErr != nil || gotHex != wantHex {
				t.Fatalf("SHA256Digest.Hex() = (%q, %v), want (%q, nil)", gotHex, gotHexErr, wantHex)
			}
			gotParsed, gotParseErr := parseSHA256Hex(gotHex)
			if gotParseErr != nil || gotParsed != value {
				t.Fatalf("ParseSHA256Hex() = (%v, %v), want (%v, nil)", gotParsed, gotParseErr, value)
			}
			var gotTextRoundTrip SHA256Digest
			gotTextRoundTripErr := gotTextRoundTrip.UnmarshalText([]byte(gotHex))
			if gotTextRoundTripErr != nil || gotTextRoundTrip != value {
				t.Fatalf("SHA256Digest.UnmarshalText(%q) = (%v, %v), want (%v, nil)", gotHex, gotTextRoundTrip, gotTextRoundTripErr, value)
			}
			gotJSON, gotJSONErr := json.Marshal(value)
			if gotJSONErr != nil {
				t.Fatalf("json.Marshal(SHA256Digest) error = %v, want nil", gotJSONErr)
			}
			var gotRoundTrip SHA256Digest
			gotRoundTripErr := json.Unmarshal(gotJSON, &gotRoundTrip)
			if gotRoundTripErr != nil || gotRoundTrip != value {
				t.Fatalf("SHA256Digest JSON round trip = (%v, %v), want (%v, nil)", gotRoundTrip, gotRoundTripErr, value)
			}
		})
	}

	canonicalZero := strings.Repeat("0", sha256.Size*2)
	invalid := []struct {
		name string
		wire string
	}{
		{name: "empty digest is rejected", wire: ""},
		{name: "one nibble is rejected", wire: "0"},
		{name: "one byte is rejected", wire: "00"},
		{name: "one nibble below exact length is rejected", wire: canonicalZero[:len(canonicalZero)-1]},
		{name: "one nibble above exact length is rejected", wire: canonicalZero + "0"},
		{name: "double exact length is rejected", wire: canonicalZero + canonicalZero},
		{name: "uppercase first nibble is rejected", wire: "A" + canonicalZero[1:]},
		{name: "uppercase final nibble is rejected", wire: canonicalZero[:len(canonicalZero)-1] + "F"},
		{name: "nonhex g at start is rejected", wire: "g" + canonicalZero[1:]},
		{name: "nonhex slash at start is rejected", wire: "/" + canonicalZero[1:]},
		{name: "nonhex colon at end is rejected", wire: canonicalZero[:len(canonicalZero)-1] + ":"},
		{name: "hex prefix is rejected", wire: "0x" + canonicalZero[2:]},
		{name: "explicit sign is rejected", wire: "+" + canonicalZero[1:]},
		{name: "space prefix is rejected", wire: " " + canonicalZero},
		{name: "space suffix is rejected", wire: canonicalZero + " "},
		{name: "newline prefix is rejected", wire: "\n" + canonicalZero},
		{name: "newline suffix is rejected", wire: canonicalZero + "\n"},
		{name: "embedded space is rejected", wire: canonicalZero[:32] + " " + canonicalZero[33:]},
		{name: "NUL prefix is rejected", wire: "\x00" + canonicalZero[1:]},
		{name: "Unicode fullwidth zero is rejected", wire: "０" + canonicalZero[1:]},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := parseSHA256Hex(tc.wire)
			if !errors.Is(gotErr, ErrPrimitiveContract) {
				t.Fatalf("ParseSHA256Hex(%q) error = %v, want %v", tc.wire, gotErr, ErrPrimitiveContract)
			}
			if got != (SHA256Digest{}) {
				t.Fatalf("ParseSHA256Hex(%q) value = %v, want zero", tc.wire, got)
			}
			preserved := NewSHA256Digest(sha256.Sum256([]byte("preserved")))
			gotText := preserved
			gotTextErr := gotText.UnmarshalText([]byte(tc.wire))
			if !errors.Is(gotTextErr, ErrPrimitiveContract) {
				t.Fatalf("SHA256Digest.UnmarshalText(%q) error = %v, want %v", tc.wire, gotTextErr, ErrPrimitiveContract)
			}
			if gotText != preserved {
				t.Fatalf("SHA256Digest.UnmarshalText(%q) value = %v, want preserved", tc.wire, gotText)
			}
		})
	}
	if gotErr := (*SHA256Digest)(nil).UnmarshalText(nil); !errors.Is(gotErr, ErrPrimitiveContract) {
		t.Fatalf("nil SHA256Digest.UnmarshalText() error = %v, want %v", gotErr, ErrPrimitiveContract)
	}
}

func TestCRC32CCanonicalBoundaryTable(t *testing.T) {
	t.Parallel()

	if got := base64.StdEncoding.EncodedLen(crc32CBytes); got != crc32CBase64Bytes {
		t.Fatalf("base64 encoded CRC32C bytes = %d, want %d", got, crc32CBase64Bytes)
	}

	values := [...]uint32{
		0, 1, 2, math.MaxUint8, math.MaxUint8 + 1,
		math.MaxUint16, math.MaxUint16 + 1, 1 << 31, math.MaxUint32 - 1, math.MaxUint32,
	}
	for _, value := range values {
		t.Run(fmt.Sprintf("uint32 boundary %d", value), func(t *testing.T) {
			t.Parallel()

			checksum := NewCRC32C(value)
			gotWire, gotWireErr := checksum.Base64()
			if gotWireErr != nil {
				t.Fatalf("CRC32C.Base64() error = %v, want nil", gotWireErr)
			}
			var gotParsed CRC32C
			gotParseErr := gotParsed.UnmarshalText([]byte(gotWire))
			if gotParseErr != nil || gotParsed != checksum {
				t.Fatalf("CRC32C.UnmarshalText(%q) = (%v, %v), want (%v, nil)", gotWire, gotParsed, gotParseErr, checksum)
			}
			gotUint32, gotUint32Err := gotParsed.Uint32()
			if gotUint32Err != nil || gotUint32 != value {
				t.Fatalf("CRC32C.Uint32() = (%d, %v), want (%d, nil)", gotUint32, gotUint32Err, value)
			}
			gotJSON, gotJSONErr := json.Marshal(checksum)
			if gotJSONErr != nil {
				t.Fatalf("json.Marshal(CRC32C(%d)) error = %v, want nil", value, gotJSONErr)
			}
			var gotRoundTrip CRC32C
			gotRoundTripErr := json.Unmarshal(gotJSON, &gotRoundTrip)
			if gotRoundTripErr != nil || gotRoundTrip != checksum {
				t.Fatalf(
					"CRC32C(%d) JSON round trip = (%v, %v), want (%v, nil)",
					value,
					gotRoundTrip,
					gotRoundTripErr,
					checksum,
				)
			}
		})
	}

	invalid := []struct {
		name string
		wire string
	}{
		{name: "empty checksum is rejected", wire: ""},
		{name: "one character is rejected", wire: "A"},
		{name: "one below encoded length is rejected", wire: "AAAAAA="},
		{name: "one above encoded length is rejected", wire: "AAAAAA==="},
		{name: "unpadded encoding is rejected", wire: "AAAAAA"},
		{name: "single padding character is rejected", wire: "AAAAAAA="},
		{name: "padding in first position is rejected", wire: "=AAAAA=="},
		{name: "padding in middle is rejected", wire: "AAA=AA=="},
		{name: "space prefix is rejected", wire: " AAAAAA=="},
		{name: "space suffix is rejected", wire: "AAAAAA== "},
		{name: "newline suffix is rejected", wire: "AAAAAA==\n"},
		{name: "URL alphabet hyphen is rejected", wire: "-AAAAA=="},
		{name: "URL alphabet underscore is rejected", wire: "_AAAAA=="},
		{name: "nonalphabet punctuation is rejected", wire: "!AAAAA=="},
		{name: "NUL prefix is rejected", wire: "\x00AAAAA=="},
		{name: "Unicode prefix is rejected", wire: "０AAAAA=="},
		{name: "JSON null spelling is rejected", wire: "null"},
		{name: "JSON string spelling is rejected", wire: `"AAAAAA=="`},
		{name: "double encoded length is rejected", wire: "AAAAAAAAAAAAAAAA"},
		{name: "far oversized input is rejected", wire: strings.Repeat("A", 1024)},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := NewCRC32C(7)
			gotErr := got.UnmarshalText([]byte(tc.wire))
			if !errors.Is(gotErr, ErrPrimitiveContract) {
				t.Fatalf("CRC32C.UnmarshalText(%q) error = %v, want %v", tc.wire, gotErr, ErrPrimitiveContract)
			}
			if got != NewCRC32C(7) {
				t.Fatalf("CRC32C.UnmarshalText(%q) value = %v, want unchanged", tc.wire, got)
			}
		})
	}
}

func TestEd25519PublicKeyOwnershipAndLengthBoundaries(t *testing.T) {
	t.Parallel()

	for seed := byte(1); seed <= 10; seed++ {
		seed := seed
		t.Run(fmt.Sprintf("distinct deterministic key seed %d", seed), func(t *testing.T) {
			t.Parallel()

			input := bytes.Repeat([]byte{seed}, ed25519.PublicKeySize)
			got, gotErr := NewEd25519PublicKey(ed25519.PublicKey(input))
			if gotErr != nil {
				t.Fatalf("NewEd25519PublicKey() error = %v, want nil", gotErr)
			}
			input[0]++
			gotBytes, gotBytesErr := got.Bytes()
			if gotBytesErr != nil || gotBytes[0] != seed {
				t.Fatalf("Ed25519PublicKey.Bytes()[0] = (%d, %v), want (%d, nil)", gotBytes[0], gotBytesErr, seed)
			}
			gotBytes[0]++
			gotSecond, gotSecondErr := got.Bytes()
			if gotSecondErr != nil || gotSecond[0] != seed {
				t.Fatalf("second Ed25519PublicKey.Bytes()[0] = (%d, %v), want (%d, nil)", gotSecond[0], gotSecondErr, seed)
			}
			wantHex := hex.EncodeToString(bytes.Repeat([]byte{seed}, ed25519.PublicKeySize))
			gotHex, gotHexErr := got.Hex()
			if gotHexErr != nil || gotHex != wantHex {
				t.Fatalf("Ed25519PublicKey.Hex() = (%q, %v), want (%q, nil)", gotHex, gotHexErr, wantHex)
			}
			gotParsed, gotParseErr := parseEd25519PublicKeyHex(gotHex)
			if gotParseErr != nil || gotParsed != got {
				t.Fatalf(
					"ParseEd25519PublicKeyHex(%q) = (%v, %v), want (%v, nil)",
					gotHex,
					gotParsed,
					gotParseErr,
					got,
				)
			}
			var gotTextRoundTrip Ed25519PublicKey
			gotTextRoundTripErr := gotTextRoundTrip.UnmarshalText([]byte(gotHex))
			if gotTextRoundTripErr != nil || gotTextRoundTrip != got {
				t.Fatalf(
					"Ed25519PublicKey.UnmarshalText(%q) = (%v, %v), want (%v, nil)",
					gotHex,
					gotTextRoundTrip,
					gotTextRoundTripErr,
					got,
				)
			}
			gotJSON, gotJSONErr := json.Marshal(got)
			if gotJSONErr != nil {
				t.Fatalf("json.Marshal(Ed25519PublicKey) error = %v, want nil", gotJSONErr)
			}
			var gotRoundTrip Ed25519PublicKey
			gotRoundTripErr := json.Unmarshal(gotJSON, &gotRoundTrip)
			if gotRoundTripErr != nil || gotRoundTrip != got {
				t.Fatalf(
					"Ed25519PublicKey JSON round trip = (%v, %v), want (%v, nil)",
					gotRoundTrip,
					gotRoundTripErr,
					got,
				)
			}
		})
	}

	invalidLengths := [...]int{
		0, 1, ed25519.PublicKeySize / 2, ed25519.PublicKeySize - 2,
		ed25519.PublicKeySize - 1, ed25519.PublicKeySize + 1,
		ed25519.PublicKeySize + 2, ed25519.PublicKeySize * 2,
		1024, JSONDocumentMaximumBytes,
	}
	for _, length := range invalidLengths {
		t.Run(fmt.Sprintf("reject key length %d", length), func(t *testing.T) {
			t.Parallel()

			got, gotErr := NewEd25519PublicKey(make(ed25519.PublicKey, length))
			if !errors.Is(gotErr, ErrPrimitiveContract) {
				t.Fatalf("NewEd25519PublicKey(length %d) error = %v, want %v", length, gotErr, ErrPrimitiveContract)
			}
			if got != (Ed25519PublicKey{}) {
				t.Fatalf("NewEd25519PublicKey(length %d) value = %v, want zero", length, got)
			}
		})
	}

	preserved, err := NewEd25519PublicKey(ed25519.PublicKey(bytes.Repeat([]byte{1}, ed25519.PublicKeySize)))
	if err != nil {
		t.Fatalf("NewEd25519PublicKey() error = %v, want nil", err)
	}
	invalidText := [...]string{
		"",
		strings.Repeat("0", ed25519.PublicKeySize*2-1),
		strings.Repeat("0", ed25519.PublicKeySize*2+1),
		strings.Repeat("G", ed25519.PublicKeySize*2),
		strings.Repeat("A", ed25519.PublicKeySize*2),
		strings.Repeat("0", 1024),
	}
	for _, text := range invalidText {
		got := preserved
		gotErr := got.UnmarshalText([]byte(text))
		if !errors.Is(gotErr, ErrPrimitiveContract) {
			t.Fatalf("Ed25519PublicKey.UnmarshalText(%q) error = %v, want %v", text, gotErr, ErrPrimitiveContract)
		}
		if got != preserved {
			t.Fatalf("Ed25519PublicKey.UnmarshalText(%q) value = %v, want preserved", text, got)
		}
	}
	if gotErr := (*Ed25519PublicKey)(nil).UnmarshalText(nil); !errors.Is(gotErr, ErrPrimitiveContract) {
		t.Fatalf("nil Ed25519PublicKey.UnmarshalText() error = %v, want %v", gotErr, ErrPrimitiveContract)
	}
}

func TestSecretMaterialHostileBoundaryAndRedactionTable(t *testing.T) {
	t.Parallel()

	valid := []struct {
		name string
		size int
		fill byte
	}{
		{name: "minimum length with low bit", size: SecretMaterialMinimumBytes, fill: 1},
		{name: "minimum length with high bit", size: SecretMaterialMinimumBytes, fill: 0x80},
		{name: "minimum plus one", size: SecretMaterialMinimumBytes + 1, fill: 2},
		{name: "quarter range", size: 24, fill: 3},
		{name: "half range", size: 32, fill: 4},
		{name: "half range plus one", size: 33, fill: 5},
		{name: "three quarter range", size: 48, fill: 6},
		{name: "maximum minus two", size: SecretMaterialMaximumBytes - 2, fill: 7},
		{name: "maximum minus one", size: SecretMaterialMaximumBytes - 1, fill: 8},
		{name: "exact maximum", size: SecretMaterialMaximumBytes, fill: 0xff},
	}
	for _, tc := range valid {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			input := bytes.Repeat([]byte{tc.fill}, tc.size)
			got, gotErr := NewSecretMaterial(input)
			if gotErr != nil {
				t.Fatalf("NewSecretMaterial(length %d) error = %v, want nil", tc.size, gotErr)
			}
			input[0] ^= math.MaxUint8
			gotBytes, gotBytesErr := got.CopyBytes()
			if gotBytesErr != nil {
				t.Fatalf("SecretMaterial.CopyBytes() error = %v, want nil", gotBytesErr)
			}
			if len(gotBytes) != tc.size || gotBytes[0] != tc.fill {
				t.Fatalf("SecretMaterial.CopyBytes() = length %d first %d, want length %d first %d", len(gotBytes), gotBytes[0], tc.size, tc.fill)
			}
			gotBytes[0] ^= math.MaxUint8
			gotSecondBytes, gotSecondErr := got.CopyBytes()
			if gotSecondErr != nil {
				t.Fatalf("SecretMaterial.CopyBytes() after caller mutation error = %v, want nil", gotSecondErr)
			}
			if gotSecond := gotSecondBytes[0]; gotSecond != tc.fill {
				t.Fatalf("SecretMaterial.CopyBytes() after caller mutation first byte = %d, want %d", gotSecond, tc.fill)
			}
			for _, format := range [...]string{"%v", "%+v", "%#v", "%s", "%q", "%x", "%X"} {
				if gotRendered := fmt.Sprintf(format, got); gotRendered != RedactedValueText {
					t.Fatalf("fmt.Sprintf(%q, SecretMaterial) = %q, want %q", format, gotRendered, RedactedValueText)
				}
			}
			gotWire, gotEncodeErr := EncodeValidatedJSON(got, DefaultStrictJSONLimits())
			if gotWire != nil || !errors.Is(gotEncodeErr, ErrJSONContract) {
				t.Fatalf(
					"EncodeValidatedJSON(SecretMaterial length %d) = (%q, %v), want (nil, %v)",
					tc.size,
					gotWire,
					gotEncodeErr,
					ErrJSONContract,
				)
			}
		})
	}

	invalid := []struct {
		name  string
		value []byte
	}{
		{name: "nil material is rejected"},
		{name: "empty material is rejected", value: []byte{}},
		{name: "one byte material is rejected", value: []byte{1}},
		{name: "minimum minus two is rejected", value: bytes.Repeat([]byte{1}, SecretMaterialMinimumBytes-2)},
		{name: "minimum minus one is rejected", value: bytes.Repeat([]byte{1}, SecretMaterialMinimumBytes-1)},
		{name: "minimum all-zero material is rejected", value: make([]byte, SecretMaterialMinimumBytes)},
		{name: "middle all-zero material is rejected", value: make([]byte, 32)},
		{name: "maximum all-zero material is rejected", value: make([]byte, SecretMaterialMaximumBytes)},
		{name: "maximum plus one is rejected", value: bytes.Repeat([]byte{1}, SecretMaterialMaximumBytes+1)},
		{name: "far oversized material is rejected", value: bytes.Repeat([]byte{1}, JSONDocumentMaximumBytes)},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := NewSecretMaterial(tc.value)
			if !errors.Is(gotErr, ErrPrimitiveContract) {
				t.Fatalf("NewSecretMaterial(length %d) error = %v, want %v", len(tc.value), gotErr, ErrPrimitiveContract)
			}
			if got != (SecretMaterial{}) {
				t.Fatalf("NewSecretMaterial(length %d) value = %v, want zero", len(tc.value), got)
			}
		})
	}
}

func TestSecretMaterialDestroyInvalidatesEveryWrapperCopyAndZerosOwnedStorage(t *testing.T) {
	t.Parallel()

	input := bytes.Repeat([]byte{0xa5}, SecretMaterialMaximumBytes)
	material, gotConstructionErr := NewSecretMaterial(input)
	if gotConstructionErr != nil {
		t.Fatalf("NewSecretMaterial() error = %v, want nil", gotConstructionErr)
	}
	wrapperCopy := material
	callerCopy, gotCopyErr := material.CopyBytes()
	if gotCopyErr != nil {
		t.Fatalf("SecretMaterial.CopyBytes() error = %v, want nil", gotCopyErr)
	}
	ownedState := material.state

	gotDestroyErr := wrapperCopy.Destroy()
	if gotDestroyErr != nil {
		t.Fatalf("copied SecretMaterial.Destroy() error = %v, want nil", gotDestroyErr)
	}
	if gotSecondDestroyErr := material.Destroy(); gotSecondDestroyErr != nil {
		t.Fatalf("second shared SecretMaterial.Destroy() error = %v, want nil", gotSecondDestroyErr)
	}
	for index, candidate := range [...]SecretMaterial{material, wrapperCopy} {
		if gotErr := candidate.Validate(); !errors.Is(gotErr, ErrPrimitiveContract) {
			t.Fatalf("destroyed wrapper %d Validate() error = %v, want %v", index, gotErr, ErrPrimitiveContract)
		}
		gotBytes, gotErr := candidate.CopyBytes()
		if gotBytes != nil || !errors.Is(gotErr, ErrPrimitiveContract) {
			t.Fatalf(
				"destroyed wrapper %d CopyBytes() = (%v, %v), want (nil, %v)",
				index,
				gotBytes,
				gotErr,
				ErrPrimitiveContract,
			)
		}
		gotCount, gotErr := candidate.ByteCount()
		if gotCount != (ByteCount{}) || !errors.Is(gotErr, ErrPrimitiveContract) {
			t.Fatalf(
				"destroyed wrapper %d ByteCount() = (%v, %v), want (zero, %v)",
				index,
				gotCount,
				gotErr,
				ErrPrimitiveContract,
			)
		}
	}

	ownedState.mutex.RLock()
	gotDestroyed := ownedState.destroyed
	gotSize := ownedState.size
	gotZero := allZeroBytes(ownedState.value[:])
	ownedState.mutex.RUnlock()
	if !gotDestroyed || gotSize != 0 || !gotZero {
		t.Fatalf(
			"destroyed owned state = (destroyed %t, size %d, zero %t), want (true, 0, true)",
			gotDestroyed,
			gotSize,
			gotZero,
		)
	}
	if !bytes.Equal(callerCopy, input) {
		t.Fatalf("caller-owned pre-destruction copy changed = %x, want %x", callerCopy, input)
	}
	if gotRendered := fmt.Sprintf("%v", material); gotRendered != RedactedValueText {
		t.Fatalf("destroyed SecretMaterial formatting = %q, want %q", gotRendered, RedactedValueText)
	}
}

func TestSecretMaterialConcurrentCopiesObserveOnlyActiveOrDestroyedState(t *testing.T) {
	t.Parallel()

	wantBytes := bytes.Repeat([]byte{0x5a}, SecretMaterialMaximumBytes)
	material, gotConstructionErr := NewSecretMaterial(wantBytes)
	if gotConstructionErr != nil {
		t.Fatalf("NewSecretMaterial() error = %v, want nil", gotConstructionErr)
	}

	ctx, cancel := context.WithCancel(context.Background())
	activeOutcomes := make(chan secretMaterialWorkerOutcome, secretMaterialConcurrentWorkerCount)
	destroyedOutcomes := make(chan secretMaterialWorkerOutcome, secretMaterialConcurrentWorkerCount)
	destroyed := make(chan struct{})
	var workers sync.WaitGroup
	workers.Add(secretMaterialConcurrentWorkerCount)
	for worker := range secretMaterialConcurrentWorkerCount {
		wrapperCopy := material
		worker := uint8(worker)
		go func() {
			defer workers.Done()

			gotBytes, gotErr := wrapperCopy.CopyBytes()
			select {
			case activeOutcomes <- secretMaterialWorkerOutcome{
				gotErr:   gotErr,
				gotBytes: gotBytes,
				worker:   worker,
				phase:    secretMaterialWorkerPhaseActive,
			}:
			case <-ctx.Done():
				return
			}
			select {
			case <-destroyed:
			case <-ctx.Done():
				return
			}
			gotBytes, gotErr = wrapperCopy.CopyBytes()
			select {
			case destroyedOutcomes <- secretMaterialWorkerOutcome{
				gotErr:   gotErr,
				gotBytes: gotBytes,
				worker:   worker,
				phase:    secretMaterialWorkerPhaseDestroyed,
			}:
			case <-ctx.Done():
			}
		}()
	}

	done := make(chan struct{})
	go func() {
		workers.Wait()
		close(done)
	}()
	defer func() {
		cancel()
		select {
		case <-done:
		case <-time.After(secretMaterialConcurrencyTimeout):
			t.Errorf(
				"concurrent SecretMaterial cleanup got timeout after %v, want all workers stopped",
				secretMaterialConcurrencyTimeout,
			)
		}
	}()
	deadline := time.NewTimer(secretMaterialConcurrencyTimeout)
	defer deadline.Stop()

	var gotActiveWorkers [secretMaterialConcurrentWorkerCount]bool
	for range secretMaterialConcurrentWorkerCount {
		select {
		case got := <-activeOutcomes:
			if got.phase != secretMaterialWorkerPhaseActive {
				t.Fatalf("active worker %d phase = %v, want %v", got.worker, got.phase, secretMaterialWorkerPhaseActive)
			}
			if int(got.worker) >= len(gotActiveWorkers) || gotActiveWorkers[got.worker] {
				t.Fatalf("active worker identity = %d, want one unique in 0..%d", got.worker, len(gotActiveWorkers)-1)
			}
			gotActiveWorkers[got.worker] = true
			if got.gotErr != nil || !bytes.Equal(got.gotBytes, wantBytes) {
				t.Fatalf(
					"active worker %d CopyBytes() = (%x, %v), want (%x, nil)",
					got.worker,
					got.gotBytes,
					got.gotErr,
					wantBytes,
				)
			}
		case <-deadline.C:
			t.Fatalf(
				"active SecretMaterial phase got timeout after %v, want %d outcomes",
				secretMaterialConcurrencyTimeout,
				secretMaterialConcurrentWorkerCount,
			)
		}
	}
	gotDestroyErr := material.Destroy()
	if gotDestroyErr != nil {
		t.Fatalf("SecretMaterial.Destroy() error = %v, want nil", gotDestroyErr)
	}
	close(destroyed)

	var gotDestroyedWorkers [secretMaterialConcurrentWorkerCount]bool
	for range secretMaterialConcurrentWorkerCount {
		select {
		case got := <-destroyedOutcomes:
			if got.phase != secretMaterialWorkerPhaseDestroyed {
				t.Fatalf("destroyed worker %d phase = %v, want %v", got.worker, got.phase, secretMaterialWorkerPhaseDestroyed)
			}
			if int(got.worker) >= len(gotDestroyedWorkers) || gotDestroyedWorkers[got.worker] {
				t.Fatalf("destroyed worker identity = %d, want one unique in 0..%d", got.worker, len(gotDestroyedWorkers)-1)
			}
			gotDestroyedWorkers[got.worker] = true
			if got.gotBytes != nil || !errors.Is(got.gotErr, ErrPrimitiveContract) {
				t.Fatalf(
					"destroyed worker %d CopyBytes() = (%v, %v), want (nil, %v)",
					got.worker,
					got.gotBytes,
					got.gotErr,
					ErrPrimitiveContract,
				)
			}
		case <-deadline.C:
			t.Fatalf(
				"destroyed SecretMaterial phase got timeout after %v, want %d outcomes",
				secretMaterialConcurrencyTimeout,
				secretMaterialConcurrentWorkerCount,
			)
		}
	}
	if gotErr := material.Validate(); !errors.Is(gotErr, ErrPrimitiveContract) {
		t.Fatalf("destroyed SecretMaterial.Validate() error = %v, want %v", gotErr, ErrPrimitiveContract)
	}
}

func digestWithByte(index int, value byte) [sha256.Size]byte {
	var digest [sha256.Size]byte
	digest[index] = value
	return digest
}

func digestAlternating(left, right byte) [sha256.Size]byte {
	var digest [sha256.Size]byte
	for index := range digest {
		if index%2 == 0 {
			digest[index] = left
		} else {
			digest[index] = right
		}
	}
	return digest
}

func digestSequence(descending bool) [sha256.Size]byte {
	var digest [sha256.Size]byte
	for index := range digest {
		if descending {
			digest[index] = byte(len(digest) - index)
		} else {
			digest[index] = byte(index)
		}
	}
	return digest
}

// TestAllZeroSecretMaterialCarriesItsOwnCoreIdentity makes the all-zero
// rejection answerable through errors.Is.
//
// Callers that acquire secret material need to tell a failed entropy source
// from a structural violation. Without a distinct identity the only way to
// separate them is to re-run an all-zero predicate over the same buffer, which
// puts one rule in two homes that can disagree silently. The identity must also
// stay inside the Primitive family so existing contract assertions keep
// matching, and stay distinct from unrelated Core identities so the
// classification cannot widen by accident.
func TestAllZeroSecretMaterialCarriesItsOwnCoreIdentity(t *testing.T) {
	t.Parallel()

	sizes := []struct {
		name string
		size int
	}{
		{name: "minimum admitted extent", size: SecretMaterialMinimumBytes},
		{name: "one above the minimum extent", size: SecretMaterialMinimumBytes + 1},
		{name: "one below the maximum extent", size: SecretMaterialMaximumBytes - 1},
		{name: "maximum admitted extent", size: SecretMaterialMaximumBytes},
	}
	for _, tc := range sizes {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := NewSecretMaterial(make([]byte, tc.size))
			if !errors.Is(gotErr, ErrSecretMaterialAllZero) {
				t.Fatalf(
					"NewSecretMaterial(%d all-zero bytes) error = %v, want %v",
					tc.size,
					gotErr,
					ErrSecretMaterialAllZero,
				)
			}
			if !errors.Is(gotErr, ErrPrimitiveContract) {
				t.Fatalf(
					"NewSecretMaterial(%d all-zero bytes) error = %v, want parent %v",
					tc.size,
					gotErr,
					ErrPrimitiveContract,
				)
			}
			if got != (SecretMaterial{}) {
				t.Fatalf("NewSecretMaterial(%d all-zero bytes) = %v, want the zero handle", tc.size, got)
			}

			// One nonzero byte anywhere must clear the rejection, so the
			// identity tracks the real rule rather than the extent.
			for index := range tc.size {
				value := make([]byte, tc.size)
				value[index] = 1
				accepted, acceptedErr := NewSecretMaterial(value)
				if acceptedErr != nil {
					t.Fatalf("NewSecretMaterial(nonzero at %d) error = %v, want nil", index, acceptedErr)
				}
				if gotValidateErr := accepted.Validate(); gotValidateErr != nil {
					t.Fatalf("NewSecretMaterial(nonzero at %d).Validate() error = %v, want nil", index, gotValidateErr)
				}
			}
		})
	}

	// A rejection that also matched an unrelated identity would let a caller's
	// classification widen without any code changing.
	unrelated := []struct {
		name     string
		identity ErrorIdentity
	}{
		{name: "numeric overflow", identity: ErrNumericOverflow},
		{name: "json contract", identity: ErrJSONContract},
		{name: "keygen entropy", identity: ErrKeygenEntropy},
	}
	_, rejection := NewSecretMaterial(make([]byte, SecretMaterialMinimumBytes))
	for _, tc := range unrelated {
		t.Run("all-zero rejection is not "+tc.name, func(t *testing.T) {
			t.Parallel()

			if errors.Is(rejection, tc.identity) {
				t.Fatalf("all-zero rejection %v matches unrelated identity %v, want no match", rejection, tc.identity)
			}
		})
	}

	// Extent violations must stay outside the all-zero identity, otherwise a
	// caller classifying on it would report a structural bug as bad entropy.
	extents := []struct {
		name string
		size int
	}{
		{name: "empty value", size: 0},
		{name: "one below the minimum extent", size: SecretMaterialMinimumBytes - 1},
		{name: "one above the maximum extent", size: SecretMaterialMaximumBytes + 1},
	}
	for _, tc := range extents {
		t.Run("extent rejection is not the all-zero identity: "+tc.name, func(t *testing.T) {
			t.Parallel()

			value := make([]byte, tc.size)
			for index := range value {
				value[index] = 1
			}
			_, gotErr := NewSecretMaterial(value)
			if !errors.Is(gotErr, ErrPrimitiveContract) {
				t.Fatalf("NewSecretMaterial(%d bytes) error = %v, want %v", tc.size, gotErr, ErrPrimitiveContract)
			}
			if errors.Is(gotErr, ErrSecretMaterialAllZero) {
				t.Fatalf(
					"NewSecretMaterial(%d nonzero bytes) error = %v, want no %v match",
					tc.size,
					gotErr,
					ErrSecretMaterialAllZero,
				)
			}
		})
	}
}
