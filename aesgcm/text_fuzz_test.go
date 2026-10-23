package aesgcm

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// Both public text doors execute here. The independent Go decoder and native
// AEAD decide representation/authentication; no remembered plaintext is proof.
func FuzzAESGCMTextSemanticClosure(f *testing.F) {
	key := make([]byte, core.AES256KeyBytes)
	c, err := NewAES256GCM(key)
	if err != nil {
		f.Fatal(err)
	}
	maximum, err := core.NewByteCount(4096)
	if err != nil {
		f.Fatal(err)
	}
	seed, err := c.SealText(SealTextRequest{Plaintext: []byte("typed cookie seed"), AdditionalData: []byte("owner/provider"), MaximumTextBytes: maximum})
	if err != nil || seed.Validate() != nil {
		f.Fatalf("text seed = (%+v,%v), want issued canonical ciphertext", seed, err)
	}
	plain, err := c.OpenText(OpenTextRequest{Ciphertext: seed.String(), AdditionalData: []byte("owner/provider"), MaximumTextBytes: maximum})
	if err != nil {
		f.Fatal(err)
	}
	if err := c.Destroy(); err != nil {
		f.Fatal(err)
	}
	f.Add([]byte(seed.String()), []byte("owner/provider"), uint64(4096), uint8(0))
	f.Add(plain.Plaintext, []byte("owner/provider"), uint64(4096), uint8(1))
	f.Add([]byte{}, []byte{}, uint64(0), uint8(0))
	f.Add([]byte("="), []byte{}, uint64(1), uint8(0))
	f.Fuzz(func(t *testing.T, data, aad []byte, capacity uint64, operation uint8) {
		c := fixtureCipher(t, key)
		maximum, maximumErr := core.NewByteCount(capacity)
		if operation%2 == 1 {
			got, err := c.SealText(SealTextRequest{Plaintext: data, AdditionalData: aad, MaximumTextBytes: maximum})
			required := uint64(base64.RawURLEncoding.EncodedLen(len(data) + core.AESGCMOverheadBytes))
			if maximumErr != nil || required > capacity {
				if !errors.Is(err, core.ErrAESGCMContract) || got != (SealedText{}) {
					t.Fatalf("over-capacity seal = (%+v,%v), want zero typed refusal", got, err)
				}
				return
			}
			if err != nil || got.Validate() != nil || uint64(len(got.String())) != required {
				t.Fatalf("text seal = (%dbytes,%v), want canonical exact%dbytes", len(got.String()), err, required)
			}
			decoded, decodeErr := base64.RawURLEncoding.Strict().DecodeString(got.String())
			if decodeErr != nil || base64.RawURLEncoding.EncodeToString(decoded) != got.String() {
				t.Fatalf("sealed representation = %v, want exact canonical fixed point", decodeErr)
			}
			want, nativeErr := nativeGCM(t, key).Open(nil, decoded[:core.AESGCMNonceBytes], decoded[core.AESGCMNonceBytes:], aad)
			if nativeErr != nil || !bytes.Equal(want, data) {
				t.Fatalf("native open of text seal = (%x,%v), want exact input%x", want, nativeErr, data)
			}
			return
		}
		text := string(data)
		got, err := c.OpenText(OpenTextRequest{Ciphertext: text, AdditionalData: aad, MaximumTextBytes: maximum})
		if maximumErr != nil || text == "" || uint64(len(text)) > capacity || strings.ContainsAny(text, "\r\n") {
			if !errors.Is(err, core.ErrAESGCMContract) || got.Plaintext != nil {
				t.Fatalf("text framing refusal = (%x,%v), want zero typed refusal", got.Plaintext, err)
			}
			return
		}
		decoded, decodeErr := base64.RawURLEncoding.Strict().DecodeString(text)
		if decodeErr != nil || base64.RawURLEncoding.EncodeToString(decoded) != text {
			if !errors.Is(err, core.ErrAESGCMContract) || got.Plaintext != nil || (decodeErr != nil && !errors.Is(err, decodeErr)) {
				t.Fatalf("noncanonical text = (%x,%v), want zero typed refusal", got.Plaintext, err)
			}
			return
		}
		block, blockErr := aes.NewCipher(key)
		if blockErr != nil {
			t.Fatal(blockErr)
		}
		native, nativeErr := cipher.NewGCMWithRandomNonce(block)
		if nativeErr != nil {
			t.Fatal(nativeErr)
		}
		want, nativeErr := native.Open(nil, nil, decoded, aad)
		if nativeErr != nil {
			if !errors.Is(err, core.ErrAESGCMAuthentication) || !errors.Is(err, nativeErr) || got.Plaintext != nil {
				t.Fatalf("text authentication refusal = (%x,%v), want no plaintext and native cause%v", got.Plaintext, err, nativeErr)
			}
			return
		}
		if err != nil || !bytes.Equal(got.Plaintext, want) {
			t.Fatalf("text authentication = (%x,%v), want native plaintext%x", got.Plaintext, err, want)
		}
	})
}
