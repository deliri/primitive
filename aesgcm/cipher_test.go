package aesgcm

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
)

func fixtureCipher(t testing.TB, key []byte) Cipher {
	t.Helper()
	c, err := NewAES256GCM(key)
	if err != nil {
		t.Fatalf("NewAES256GCM() error = %v, want nil", err)
	}
	t.Cleanup(func() {
		if err := c.Destroy(); err != nil {
			t.Errorf("Destroy() error = %v, want nil", err)
		}
	})
	return c
}

func nativeGCM(t testing.TB, key []byte) cipher.AEAD {
	t.Helper()
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatal(err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatal(err)
	}
	return gcm
}

func TestAESGCMKeyAndNativeLayerTriad(t *testing.T) {
	t.Parallel()
	for _, size := range []int{0, 1, 15, 16, 24, 31, 32, 33, 64} {
		t.Run(fmt.Sprintf("key_bytes_%d", size), func(t *testing.T) {
			t.Parallel()
			key := make([]byte, size)
			got, err := NewAES256GCM(key)
			if size != core.AES256KeyBytes {
				if !errors.Is(err, core.ErrAESGCMContract) || got.state != nil {
					t.Fatalf("constructor = (%v,%v), want zero capability and contract refusal", got, err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			defer got.Destroy()
			if err := got.Validate(); err != nil {
				t.Fatal(err)
			}
			plain := []byte("secret survives caller key mutation")
			sealed, err := got.Seal(SealRequest{Plaintext: plain, AdditionalData: []byte("owner:17")})
			if err != nil || len(sealed.Ciphertext) != len(plain)+core.AESGCMOverheadBytes {
				t.Fatalf("Seal = (%d bytes,%v), want %d bytes,nil", len(sealed.Ciphertext), err, len(plain)+core.AESGCMOverheadBytes)
			}
			key[0] = 1
			opened, err := got.Open(OpenRequest{Ciphertext: sealed.Ciphertext, AdditionalData: []byte("owner:17")})
			if err != nil || !bytes.Equal(opened.Plaintext, plain) {
				t.Fatalf("Open after key mutation = (%q,%v), want (%q,nil)", opened.Plaintext, err, plain)
			}
			fromNative, err := nativeGCM(t, make([]byte, 32)).Open(nil, sealed.Ciphertext[:core.AESGCMNonceBytes], sealed.Ciphertext[core.AESGCMNonceBytes:], []byte("owner:17"))
			if err != nil || !bytes.Equal(fromNative, plain) {
				t.Fatalf("native differential = (%q,%v), want original plaintext,nil", fromNative, err)
			}
		})
	}
}

// AES-256-GCM zero-key/zero-IV known answers. The fixed nonce is only used
// for independently supplied Open input; sealing never accepts caller nonces.
func TestAESGCMKnownVectorAuthentication(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, plaintext, ciphertext string }{
		{"empty plaintext authenticates tag", "", "530f8afbc74536b9a963b4f1c4cb738b"},
		{"one block authenticates ciphertext", "00000000000000000000000000000000", "cea7403d4d606b6e074ec5d3baf39d18d0d1c8a799996bf0265b98b5d48ab919"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body, err := hex.DecodeString(tc.ciphertext)
			if err != nil {
				t.Fatal(err)
			}
			want, err := hex.DecodeString(tc.plaintext)
			if err != nil {
				t.Fatal(err)
			}
			envelope := append(make([]byte, 12), body...)
			c := fixtureCipher(t, make([]byte, 32))
			got, err := c.Open(OpenRequest{Ciphertext: envelope})
			if err != nil || !bytes.Equal(got.Plaintext, want) {
				t.Fatalf("known vector Open = (%x,%v), want (%x,nil)", got.Plaintext, err, want)
			}
			envelope[len(envelope)-1] ^= 1
			got, err = c.Open(OpenRequest{Ciphertext: envelope})
			if !errors.Is(err, core.ErrAESGCMAuthentication) || got.Plaintext != nil {
				t.Fatalf("mutated vector = (%x,%v), want no plaintext and authentication refusal", got.Plaintext, err)
			}
		})
	}
}

// NIST GCM-AES256 example #2, pages 21-23 of the official worked examples:
// https://csrc.nist.gov/CSRC/media/Projects/Cryptographic-Standards-and-Guidelines/documents/examples/AES_GCM.pdf
func TestAESGCMNISTOfficialVector(t *testing.T) {
	t.Parallel()
	key, err := hex.DecodeString("feffe9928665731c6d6a8f9467308308feffe9928665731c6d6a8f9467308308")
	if err != nil {
		t.Fatal(err)
	}
	envelope, err := hex.DecodeString("cafebabefacedbaddecaf888522dc1f099567d07f47f37a32a84427d643a8cdcbfe5c0c97598a2bd2555d1aa8cb08e48590dbb3da7b08b1056828838c5f61e6393ba7a0abcc9f662898015adb094dac5d93471bdec1a502270e3cc6c")
	if err != nil {
		t.Fatal(err)
	}
	want, err := hex.DecodeString("d9313225f88406e5a55909c5aff5269a86a7a9531534f7da2e4c303d8a318a721c3c0c95956809532fcf0e2449a6b525b16aedf5aa0de657ba637b391aafd255")
	if err != nil {
		t.Fatal(err)
	}
	c := fixtureCipher(t, key)
	got, err := c.Open(OpenRequest{Ciphertext: envelope})
	if err != nil || !bytes.Equal(got.Plaintext, want) {
		t.Fatalf("NIST Open=(%x,%v), want (%x,nil)", got.Plaintext, err, want)
	}
}

func TestAESGCMRefusalReleasesNoPlaintext(t *testing.T) {
	t.Parallel()
	c := fixtureCipher(t, make([]byte, 32))
	plain, aad := []byte("private account reset token"), []byte("owner/first/token")
	sealed, err := c.Seal(SealRequest{Plaintext: plain, AdditionalData: aad})
	if err != nil {
		t.Fatal(err)
	}
	other, err := c.Seal(SealRequest{Plaintext: []byte("other message"), AdditionalData: aad})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name                 string
		ciphertext, aad, key []byte
	}{
		{"nonce substitution", flipByte(sealed.Ciphertext, 0), aad, nil},
		{"body substitution", flipByte(sealed.Ciphertext, 12), aad, nil},
		{"tag substitution", flipByte(sealed.Ciphertext, len(sealed.Ciphertext)-1), aad, nil},
		{"owner associated data substitution", sealed.Ciphertext, []byte("owner/second/token"), nil},
		{"associated data removed", sealed.Ciphertext, nil, nil},
		{"foreign key", sealed.Ciphertext, aad, bytes.Repeat([]byte{7}, 32)},
		{"cross message tag", append(slices.Clone(sealed.Ciphertext[:len(sealed.Ciphertext)-16]), other.Ciphertext[len(other.Ciphertext)-16:]...), aad, nil},
		{"cross message nonce", append(slices.Clone(other.Ciphertext[:12]), sealed.Ciphertext[12:]...), aad, nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			active := c
			if tc.key != nil {
				active = fixtureCipher(t, tc.key)
			}
			before := slices.Clone(tc.ciphertext)
			got, err := active.Open(OpenRequest{Ciphertext: tc.ciphertext, AdditionalData: tc.aad})
			if !errors.Is(err, core.ErrAESGCMAuthentication) || !errors.Is(err, core.ErrPrimitiveContract) || got.Plaintext != nil || !bytes.Equal(before, tc.ciphertext) {
				t.Fatalf("Open refusal = (%x,%v), input unchanged=%v, want zero plaintext and typed refusal", got.Plaintext, err, bytes.Equal(before, tc.ciphertext))
			}
			// Go's unexported error identity is preserved through errors.Is without
			// treating native diagnostic prose as an identity.
			_, nativeErr := nativeGCM(t, make([]byte, 32)).Open(nil, make([]byte, 12), make([]byte, 16), nil)
			if !errors.Is(err, nativeErr) {
				t.Fatalf("native cause = %v, want retained %v", err, nativeErr)
			}
		})
	}
	for n := 0; n < len(sealed.Ciphertext); n++ {
		t.Run(fmt.Sprintf("truncation_%d", n), func(t *testing.T) {
			t.Parallel()
			got, err := c.Open(OpenRequest{Ciphertext: sealed.Ciphertext[:n], AdditionalData: aad})
			if !errors.Is(err, core.ErrAESGCMAuthentication) || got.Plaintext != nil {
				t.Fatalf("truncated Open = (%x,%v), want zero/auth refusal", got.Plaintext, err)
			}
		})
	}
}

func flipByte(in []byte, index int) []byte { out := slices.Clone(in); out[index] ^= 1; return out }

func TestAESGCMWholeValueOwnership(t *testing.T) {
	t.Parallel()
	c := fixtureCipher(t, make([]byte, 32))
	for _, size := range []int{0, 1, 15, 16, 17, 65535, 65536, 65537, 1048577} {
		t.Run(fmt.Sprintf("caller_owned_bytes_%d", size), func(t *testing.T) {
			t.Parallel()
			plain := bytes.Repeat([]byte{0x61}, size)
			aad := []byte("owner/version/purpose")
			sealed, err := c.Seal(SealRequest{Plaintext: plain, AdditionalData: aad})
			if err != nil {
				t.Fatal(err)
			}
			if len(sealed.Ciphertext) != size+core.AESGCMOverheadBytes || !bytes.Equal(plain, bytes.Repeat([]byte{0x61}, size)) {
				t.Fatalf("Seal extent=%d, want %d and unchanged input", len(sealed.Ciphertext), size+core.AESGCMOverheadBytes)
			}
			clear(plain)
			got, err := c.Open(OpenRequest{Ciphertext: sealed.Ciphertext, AdditionalData: aad})
			if err != nil || !bytes.Equal(got.Plaintext, bytes.Repeat([]byte{0x61}, size)) {
				t.Fatalf("Open after caller input clear=(%d bytes,%v), want original %d bytes,nil", len(got.Plaintext), err, size)
			}
			clear(got.Plaintext)
			second, err := c.Open(OpenRequest{Ciphertext: sealed.Ciphertext, AdditionalData: aad})
			if err != nil || !bytes.Equal(second.Plaintext, bytes.Repeat([]byte{0x61}, size)) {
				t.Fatalf("second Open after result clear=(%d bytes,%v), want independent original bytes,nil", len(second.Plaintext), err)
			}
		})
	}
}

func TestAESGCMLifecycleLayerTriad(t *testing.T) {
	t.Parallel()
	var zero Cipher
	if err := zero.Validate(); !errors.Is(err, core.ErrAESGCMContract) {
		t.Fatalf("zero Validate=%v, want contract refusal", err)
	}
	if _, err := zero.Seal(SealRequest{}); !errors.Is(err, core.ErrAESGCMContract) {
		t.Fatalf("zero Seal=%v, want contract refusal", err)
	}
	if got, err := zero.Open(OpenRequest{}); !errors.Is(err, core.ErrAESGCMContract) || got.Plaintext != nil {
		t.Fatalf("zero Open=(%v,%v), want zero/refusal", got, err)
	}
	if err := zero.Destroy(); !errors.Is(err, core.ErrAESGCMContract) {
		t.Fatalf("zero Destroy=%v, want contract refusal", err)
	}
	c := fixtureCipher(t, bytes.Repeat([]byte{42}, 32))
	alias := c
	for _, verb := range []string{"%v", "%+v", "%#v", "%s", "%q"} {
		if got := fmt.Sprintf(verb, c); got != core.RedactedValueText {
			t.Fatalf("format %s=%q, want redacted", verb, got)
		}
	}
	if got, err := c.MarshalJSON(); got != nil || !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("MarshalJSON=(%q,%v), want nil/refusal", got, err)
	}
	sealed, err := c.Seal(SealRequest{})
	if err != nil || len(sealed.Ciphertext) != 28 {
		t.Fatalf("empty Seal=(%v,%v), want authenticated empty envelope", sealed, err)
	}
	got, err := alias.Open(OpenRequest{Ciphertext: sealed.Ciphertext})
	if err != nil || len(got.Plaintext) != 0 {
		t.Fatalf("empty Open=(%v,%v), want empty,nil", got, err)
	}
	if err := c.Destroy(); err != nil {
		t.Fatal(err)
	}
	if err := alias.Validate(); !errors.Is(err, core.ErrAESGCMContract) {
		t.Fatalf("destroyed alias Validate=%v, want refusal", err)
	}
	if _, err := alias.Seal(SealRequest{}); !errors.Is(err, core.ErrAESGCMContract) {
		t.Fatalf("destroyed alias Seal=%v, want refusal", err)
	}
	if got, err := alias.Open(OpenRequest{Ciphertext: sealed.Ciphertext}); !errors.Is(err, core.ErrAESGCMContract) || got.Plaintext != nil {
		t.Fatalf("destroyed Open=(%v,%v), want zero/refusal", got, err)
	}
	if err := alias.Destroy(); err != nil {
		t.Fatalf("repeat Destroy=%v, want nil", err)
	}
}

func TestAESGCMNativeExtentBoundary(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		size, max  uint64
		wantRefuse bool
	}{
		{"empty", 0, 1<<63 - 1, false}, {"algorithm below", core.AESGCMMaximumPlaintextBytes - 1, 1<<63 - 1, false}, {"algorithm exact", core.AESGCMMaximumPlaintextBytes, 1<<63 - 1, false}, {"algorithm above", core.AESGCMMaximumPlaintextBytes + 1, 1<<63 - 1, true},
		{"32bit allocation exact", (1<<31 - 1) - 28, 1<<31 - 1, false}, {"32bit allocation above", (1<<31 - 1) - 27, 1<<31 - 1, true}, {"overhead unrepresentable", 0, 27, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := validatePlaintextExtent(tc.size, tc.max)
			if errors.Is(err, core.ErrAESGCMContract) != tc.wantRefuse {
				t.Fatalf("extent(%d,%d)=%v, want refusal=%v", tc.size, tc.max, err, tc.wantRefuse)
			}
		})
	}
}

func TestAESGCMConcurrentDestroyAliases(t *testing.T) {
	t.Parallel()
	c := fixtureCipher(t, make([]byte, 32))
	start := make(chan struct{})
	done := make(chan struct{})
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			<-start
			for range 32 {
				out, err := c.Seal(SealRequest{Plaintext: []byte("secret")})
				if err == nil {
					got, err := c.Open(OpenRequest{Ciphertext: out.Ciphertext})
					if err == nil && !bytes.Equal(got.Plaintext, []byte("secret")) {
						t.Errorf("concurrent plaintext=%q, want exact secret", got.Plaintext)
					}
					if err != nil && !errors.Is(err, core.ErrAESGCMContract) {
						t.Errorf("concurrent Open error=%v, want lifecycle refusal", err)
					}
				} else if !errors.Is(err, core.ErrAESGCMContract) {
					t.Errorf("concurrent Seal error=%v, want lifecycle refusal", err)
				}
			}
		})
	}
	wg.Go(func() {
		<-start
		if err := c.Destroy(); err != nil {
			t.Errorf("concurrent Destroy=%v, want nil", err)
		}
	})
	close(start)
	go func() { wg.Wait(); close(done) }()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("concurrent operations did not complete")
	}
	if err := c.Validate(); !errors.Is(err, core.ErrAESGCMContract) {
		t.Fatalf("after joined Destroy Validate=%v, want refusal", err)
	}
}
