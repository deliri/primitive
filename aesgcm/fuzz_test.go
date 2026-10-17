package aesgcm

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

// NewAES256GCM, Cipher.Seal and Cipher.Open are the package's external ingress.
// The oracle compares actual native authentication, not merely round trips.
func FuzzAESGCMNativeAuthentication(f *testing.F) {
	key := make([]byte, core.AES256KeyBytes)
	c, err := NewAES256GCM(key)
	if err != nil {
		f.Fatal(err)
	}
	seed, err := c.Seal(SealRequest{Plaintext: []byte("typed seed secret"), AdditionalData: []byte("owner/path")})
	if err != nil {
		f.Fatal(err)
	}
	if err := c.Validate(); err != nil {
		f.Fatal(err)
	}
	if err := c.Destroy(); err != nil {
		f.Fatal(err)
	}
	f.Add(key, seed.Ciphertext, []byte("owner/path"))
	f.Add(key, []byte{}, []byte{})
	f.Add([]byte{1}, seed.Ciphertext, []byte("owner/path"))
	f.Fuzz(func(t *testing.T, key, envelope, aad []byte) {
		c, err := NewAES256GCM(key)
		if len(key) != core.AES256KeyBytes {
			if !errors.Is(err, core.ErrAESGCMContract) || c.state != nil {
				t.Fatalf("key constructor=(%v,%v), want zero/refusal", c, err)
			}
			return
		}
		if err != nil {
			t.Fatal(err)
		}
		defer c.Destroy()
		got, gotErr := c.Open(OpenRequest{Ciphertext: envelope, AdditionalData: aad})
		block, err := aes.NewCipher(key)
		if err != nil {
			t.Fatal(err)
		}
		native, err := cipher.NewGCMWithRandomNonce(block)
		if err != nil {
			t.Fatal(err)
		}
		want, wantErr := native.Open(nil, nil, envelope, aad)
		if wantErr != nil {
			if !errors.Is(gotErr, core.ErrAESGCMAuthentication) || !errors.Is(gotErr, wantErr) || got.Plaintext != nil {
				t.Fatalf("refused Open=(%x,%v), want zero/native refusal %v", got.Plaintext, gotErr, wantErr)
			}
			return
		}
		if gotErr != nil || !bytes.Equal(got.Plaintext, want) {
			t.Fatalf("accepted Open=(%x,%v), want native %x,nil", got.Plaintext, gotErr, want)
		}
		sealed, err := c.Seal(SealRequest{Plaintext: got.Plaintext, AdditionalData: aad})
		if err != nil {
			t.Fatal(err)
		}
		opened, err := native.Open(nil, nil, sealed.Ciphertext, aad)
		if err != nil || !bytes.Equal(opened, want) {
			t.Fatalf("native reseal Open=(%x,%v), want %x,nil", opened, err, want)
		}
	})
}

// Each callback starts with authentic output and changes exactly one binding;
// arbitrary byte fuzzing alone usually never reaches a genuine GCM envelope.
func FuzzAESGCMBindingMutation(f *testing.F) {
	f.Add([]byte("secret"), []byte("owner/path"), uint8(0))
	f.Add([]byte{}, []byte{}, uint8(2))
	f.Fuzz(func(t *testing.T, plain, aad []byte, selector uint8) {
		c := fixtureCipher(t, make([]byte, 32))
		sealed, err := c.Seal(SealRequest{Plaintext: plain, AdditionalData: aad})
		if err != nil {
			t.Fatal(err)
		}
		boundAAD := aad
		switch selector % 4 {
		case 0:
			sealed.Ciphertext[0] ^= 1
		case 1:
			sealed.Ciphertext[len(sealed.Ciphertext)-1] ^= 1
		case 2:
			boundAAD = append(append([]byte(nil), aad...), 1)
		case 3:
			sealed.Ciphertext = sealed.Ciphertext[:len(sealed.Ciphertext)-1]
		}
		got, err := c.Open(OpenRequest{Ciphertext: sealed.Ciphertext, AdditionalData: boundAAD})
		if !errors.Is(err, core.ErrAESGCMAuthentication) || got.Plaintext != nil {
			t.Fatalf("binding mutation %d=(%x,%v), want zero/auth refusal", selector%4, got.Plaintext, err)
		}
	})
}
