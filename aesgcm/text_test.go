package aesgcm

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/testserial"
)

var (
	_ func(Cipher, SealTextRequest) (SealedText, error) = Cipher.SealText
	_ func(Cipher, OpenTextRequest) (OpenResult, error) = Cipher.OpenText
)

func textMaximum(t testing.TB, size uint64) core.ByteCount {
	t.Helper()
	value, err := core.NewByteCount(size)
	if err != nil {
		t.Fatalf("text transport capacity = %v, want positive byte count", err)
	}
	return value
}

func TestAESGCMTextLayerTriad(t *testing.T) {
	t.Parallel()
	for _, size := range []int{0, 1, 2, 3, 15, 16, 17, 4096} {
		t.Run(fmt.Sprintf("plaintext_%d_bytes", size), func(t *testing.T) {
			t.Parallel()
			c := fixtureCipher(t, make([]byte, core.AES256KeyBytes))
			plain := bytes.Repeat([]byte{0x61}, size)
			aad := []byte("owner/provider/callback")
			encodedSize := base64.RawURLEncoding.EncodedLen(size + core.AESGCMOverheadBytes)
			for _, allowance := range []int{encodedSize - 1, encodedSize, encodedSize + 1} {
				maximum := textMaximum(t, uint64(allowance))
				got, err := c.SealText(SealTextRequest{Plaintext: plain, AdditionalData: aad, MaximumTextBytes: maximum})
				if allowance < encodedSize {
					if !errors.Is(err, core.ErrAESGCMContract) || got != (SealedText{}) {
						t.Fatalf("SealText capacity%d = (%+v,%v), want zero refusal before encryption", allowance, got, err)
					}
					continue
				}
				if err != nil || got.Validate() != nil || len(got.String()) != encodedSize || !bytes.Equal(plain, bytes.Repeat([]byte{0x61}, size)) {
					t.Fatalf("SealText = (%dbytes,%v), want exact%dbytes, valid nominal and unchanged plaintext", len(got.String()), err, encodedSize)
				}
				decoded, decodeErr := base64.RawURLEncoding.Strict().DecodeString(got.String())
				if decodeErr != nil || base64.RawURLEncoding.EncodeToString(decoded) != got.String() {
					t.Fatalf("sealed text native representation = %v, want canonical RawURL", decodeErr)
				}
				opened, openErr := c.OpenText(OpenTextRequest{Ciphertext: got.String(), AdditionalData: aad, MaximumTextBytes: maximum})
				if openErr != nil || !bytes.Equal(opened.Plaintext, plain) {
					t.Fatalf("OpenText = (%x,%v), want original plaintext%x", opened.Plaintext, openErr, plain)
				}
				limited, limitErr := c.OpenText(OpenTextRequest{Ciphertext: got.String(), AdditionalData: aad, MaximumTextBytes: textMaximum(t, uint64(encodedSize-1))})
				if !errors.Is(limitErr, core.ErrAESGCMContract) || limited.Plaintext != nil {
					t.Fatalf("over-capacity OpenText = (%x,%v), want zero typed refusal", limited.Plaintext, limitErr)
				}
				if text := fmt.Sprintf("%v/%+v/%#v", got, got, got); strings.Contains(text, got.String()) {
					t.Fatalf("sealed text formatting = %q, want redacted transport credential", text)
				}
			}
		})
	}
}

func TestAESGCMTextCanonicalAndAuthenticationRefusals(t *testing.T) {
	t.Parallel()
	c := fixtureCipher(t, make([]byte, core.AES256KeyBytes))
	maximum := textMaximum(t, 4096)
	seed, err := c.SealText(SealTextRequest{AdditionalData: []byte("owner/provider"), MaximumTextBytes: maximum})
	if err != nil {
		t.Fatal(err)
	}
	alphabet := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"
	last := strings.IndexByte(alphabet, seed.String()[len(seed.String())-1])
	noncanonical := seed.String()[:len(seed.String())-1] + string(alphabet[last|1])
	for _, row := range []struct {
		name string
		text string
		aad  []byte
		want error
	}{
		{name: "empty text is no envelope", want: core.ErrAESGCMContract},
		{name: "padding is forbidden", text: seed.String() + "==", aad: []byte("owner/provider"), want: core.ErrAESGCMContract},
		{name: "newline is not an alternative spelling", text: seed.String() + "\n", aad: []byte("owner/provider"), want: core.ErrAESGCMContract},
		{name: "carriage return is not an alternative spelling", text: "\r" + seed.String(), aad: []byte("owner/provider"), want: core.ErrAESGCMContract},
		{name: "nonzero spare bits are forbidden", text: noncanonical, aad: []byte("owner/provider"), want: core.ErrAESGCMContract},
		{name: "foreign alphabet is forbidden", text: "!" + seed.String()[1:], aad: []byte("owner/provider"), want: core.ErrAESGCMContract},
		{name: "foreign AAD yields no plaintext", text: seed.String(), aad: []byte("foreign/provider"), want: core.ErrAESGCMAuthentication},
		{name: "truncated native envelope is unauthenticated", text: base64.RawURLEncoding.EncodeToString(make([]byte, core.AESGCMOverheadBytes-1)), want: core.ErrAESGCMAuthentication},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			got, err := c.OpenText(OpenTextRequest{Ciphertext: row.text, AdditionalData: row.aad, MaximumTextBytes: maximum})
			if !errors.Is(err, row.want) || got.Plaintext != nil {
				t.Fatalf("OpenText refusal = (%x,%v), want no plaintext and%v", got.Plaintext, err, row.want)
			}
		})
	}
	if err := (SealedText{}).Validate(); !errors.Is(err, core.ErrAESGCMContract) {
		t.Fatalf("unissued SealedText.Validate() = %v, want contract refusal", err)
	}
}

func TestAESGCMTextCapacityPrecedesAllocation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	c := fixtureCipher(t, make([]byte, core.AES256KeyBytes))
	large := strings.Repeat("A", 1<<20)
	maximum := textMaximum(t, 1)
	var got OpenResult
	var gotErr error
	allocations := testing.AllocsPerRun(1, func() { got, gotErr = c.OpenText(OpenTextRequest{Ciphertext: large, MaximumTextBytes: maximum}) })
	if allocations != 0 || got.Plaintext != nil || !errors.Is(gotErr, core.ErrAESGCMContract) {
		t.Fatalf("over-capacity decode = (%gallocations,%x,%v), want zero allocation before decoding and no plaintext", allocations, got.Plaintext, gotErr)
	}
}

func TestAESGCMTextNativeExtentAndDestroyedCapability(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name   string
		size   uint64
		native uint64
		limit  uint64
		refuse bool
	}{
		{name: "empty fits exact native text extent", native: 38, limit: 38},
		{name: "native text extent one below", native: 37, limit: 38, refuse: true},
		{name: "native text extent one above", native: 39, limit: 38},
		{name: "native ciphertext overhead cannot fit", native: 27, limit: 100, refuse: true},
		{name: "caller text extent one below", native: 100, limit: 37, refuse: true},
		{name: "algorithm maximum is still admitted with wide extent", size: core.AESGCMMaximumPlaintextBytes, native: 1<<63 - 1, limit: 1<<63 - 1},
		{name: "algorithm maximum plus one refuses without allocation", size: core.AESGCMMaximumPlaintextBytes + 1, native: 1<<63 - 1, limit: 1<<63 - 1, refuse: true},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			err := validateSealedTextExtent(row.size, textMaximum(t, row.limit), row.native)
			if (err != nil) != row.refuse || (err != nil && !errors.Is(err, core.ErrAESGCMContract)) {
				t.Fatalf("encoded native extent = %v, want refusal%t", err, row.refuse)
			}
		})
	}
	c := fixtureCipher(t, make([]byte, core.AES256KeyBytes))
	maximum := textMaximum(t, 4096)
	seed, err := c.SealText(SealTextRequest{MaximumTextBytes: maximum})
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Destroy(); err != nil {
		t.Fatal(err)
	}
	if got, err := c.SealText(SealTextRequest{MaximumTextBytes: maximum}); !errors.Is(err, core.ErrAESGCMContract) || got != (SealedText{}) {
		t.Fatalf("destroyed SealText = (%+v,%v), want no issued text", got, err)
	}
	if got, err := c.OpenText(OpenTextRequest{Ciphertext: seed.String(), MaximumTextBytes: maximum}); !errors.Is(err, core.ErrAESGCMContract) || got.Plaintext != nil {
		t.Fatalf("destroyed OpenText = (%x,%v), want no plaintext", got.Plaintext, err)
	}
	if got, err := c.OpenText(OpenTextRequest{Ciphertext: seed.String()}); !errors.Is(err, core.ErrAESGCMContract) || got.Plaintext != nil {
		t.Fatalf("missing caller capacity = (%x,%v), want typed refusal", got.Plaintext, err)
	}
}
