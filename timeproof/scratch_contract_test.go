package timeproof

import (
	"bytes"
	"encoding/asn1"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestRawDecodeScratchLayerTriad(t *testing.T) {
	t.Parallel()
	first, err := asn1.Marshal(oidSHA256())
	if err != nil {
		t.Fatalf("asn1.Marshal(identifier) error = %v, want nil", err)
	}
	second, err := asn1.Marshal([]byte("different bytes"))
	if err != nil {
		t.Fatalf("asn1.Marshal(octets) error = %v, want nil", err)
	}
	t.Run("reusing scratch preserves earlier borrowed values", func(t *testing.T) {
		t.Parallel()
		var scratch asn1.RawValue
		one, rest, err := consumeRawInto(first, &scratch)
		if err != nil || len(rest) != 0 || !bytes.Equal(one.FullBytes, first) {
			t.Fatalf("first decode = (%x, %x, %v), want exact first value", one.FullBytes, rest, err)
		}
		two, rest, err := consumeRawInto(second, &scratch)
		if err != nil || len(rest) != 0 || !bytes.Equal(two.FullBytes, second) || !bytes.Equal(one.FullBytes, first) || &one.FullBytes[0] != &first[0] || &two.FullBytes[0] != &second[0] {
			t.Fatalf("reused decode = (%x, %x, %v), want independent borrowed value identities", one.FullBytes, two.FullBytes, err)
		}
	})
	t.Run("malformed input clears a populated scratch destination", func(t *testing.T) {
		t.Parallel()
		scratch := rawValueFromDER(t, first)
		got, rest, err := consumeRawInto([]byte{0x30, 0x81}, &scratch)
		if !errors.Is(err, core.ErrTimeProofInvalid) || len(rest) != 0 || got.Class != 0 || got.Tag != 0 || got.IsCompound || len(got.Bytes) != 0 || len(got.FullBytes) != 0 || scratch.Class != 0 || scratch.Tag != 0 || scratch.IsCompound || len(scratch.Bytes) != 0 || len(scratch.FullBytes) != 0 {
			t.Fatalf("rejected decode = (%+v, %+v, %x, %v), want zero result and scratch with typed rejection", got, scratch, rest, err)
		}
	})
	t.Run("empty octets replace preceding populated bytes", func(t *testing.T) {
		t.Parallel()
		scratch := rawValueFromDER(t, second)
		empty, err := asn1.Marshal([]byte{})
		if err != nil {
			t.Fatalf("asn1.Marshal(empty octets) error = %v, want nil", err)
		}
		got, rest, err := consumeRawInto(empty, &scratch)
		if err != nil || len(rest) != 0 || !isUniversal(got, asn1.TagOctetString, false) || len(got.Bytes) != 0 || !bytes.Equal(got.FullBytes, empty) || len(scratch.Bytes) != 0 || !bytes.Equal(scratch.FullBytes, empty) {
			t.Fatalf("empty decode = (%+v, %x, %v), want empty octets with exact framing and no old bytes", got, rest, err)
		}
	})
}
