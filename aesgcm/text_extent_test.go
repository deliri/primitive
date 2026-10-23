package aesgcm

import (
	"bytes"
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/testserial"
	"strings"
	"testing"
)

func TestAESGCMTextExtentIdentityLayerTriad(t *testing.T) {
	t.Parallel()
	c := fixtureCipher(t, make([]byte, core.AES256KeyBytes))
	maximum := textMaximum(t, 4096)
	seed, err := c.SealText(SealTextRequest{Plaintext: []byte("owned"), MaximumTextBytes: maximum})
	if err != nil {
		t.Fatal(err)
	}
	size := uint64(len(seed.String()))
	for _, row := range []struct {
		name    string
		maximum uint64
		want    bool
	}{{"one below encoded extent", size - 1, true}, {"exact encoded extent", size, false}, {"one above encoded extent", size + 1, false}} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			limit := textMaximum(t, row.maximum)
			sealed, sealErr := c.SealText(SealTextRequest{Plaintext: []byte("owned"), MaximumTextBytes: limit})
			opened, openErr := c.OpenText(OpenTextRequest{Ciphertext: seed.String(), MaximumTextBytes: limit})
			if errors.Is(sealErr, core.ErrAESGCMTextExtent) != row.want || errors.Is(openErr, core.ErrAESGCMTextExtent) != row.want {
				t.Fatalf("text extent identities=(%v,%v),want extent%t", sealErr, openErr, row.want)
			}
			if row.want {
				if sealed != (SealedText{}) || opened.Plaintext != nil || !errors.Is(sealErr, core.ErrAESGCMContract) || !errors.Is(openErr, core.ErrPrimitiveContract) {
					t.Fatalf("extent refusal=(%+v,%x,%v,%v),want zero and parent error chain", sealed, opened.Plaintext, sealErr, openErr)
				}
			} else if sealErr != nil || openErr != nil || !bytes.Equal(opened.Plaintext, []byte("owned")) {
				t.Fatalf("fitting text=(%v,%v,%x),want exact roundtrip", sealErr, openErr, opened.Plaintext)
			}
		})
	}
	for _, row := range []struct {
		name    string
		request OpenTextRequest
		want    error
	}{{"empty representation is contract failure", OpenTextRequest{MaximumTextBytes: maximum}, core.ErrAESGCMContract}, {"missing capacity is contract failure", OpenTextRequest{Ciphertext: seed.String()}, core.ErrAESGCMContract}, {"malformed representation is contract failure", OpenTextRequest{Ciphertext: "!", MaximumTextBytes: maximum}, core.ErrAESGCMContract}, {"wrong AAD is authentication failure", OpenTextRequest{Ciphertext: seed.String(), AdditionalData: []byte("foreign"), MaximumTextBytes: maximum}, core.ErrAESGCMAuthentication}} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			got, err := c.OpenText(row.request)
			if !errors.Is(err, row.want) || errors.Is(err, core.ErrAESGCMTextExtent) || got.Plaintext != nil {
				t.Fatalf("other refusal=(%x,%v),want%v and no extent identity", got.Plaintext, err, row.want)
			}
		})
	}
	t.Run("encoded native integer capacity has exact identity", func(t *testing.T) {
		t.Parallel()
		got := validateSealedTextExtent(0, textMaximum(t, 100), 37)
		if !errors.Is(got, core.ErrAESGCMTextExtent) {
			t.Fatalf("native text capacity=%v,want extent identity", got)
		}
	})
	t.Run("algorithm ceiling is not transport extent", func(t *testing.T) {
		t.Parallel()
		got := validateSealedTextExtent(core.AESGCMMaximumPlaintextBytes+1, textMaximum(t, 1<<63-1), 1<<63-1)
		if !errors.Is(got, core.ErrAESGCMContract) || errors.Is(got, core.ErrAESGCMTextExtent) {
			t.Fatalf("algorithm ceiling=%v,want contract without transport identity", got)
		}
	})
	t.Run("destroyed key is not transport extent", func(t *testing.T) {
		t.Parallel()
		cipher := fixtureCipher(t, make([]byte, core.AES256KeyBytes))
		if err := cipher.Destroy(); err != nil {
			t.Fatal(err)
		}
		got, err := cipher.OpenText(OpenTextRequest{Ciphertext: seed.String(), MaximumTextBytes: maximum})
		if !errors.Is(err, core.ErrAESGCMContract) || errors.Is(err, core.ErrAESGCMTextExtent) || got.Plaintext != nil {
			t.Fatalf("destroyed key=(%x,%v),want contract without extent", got.Plaintext, err)
		}
	})
}

func TestAESGCMTextExtentRefusalAllocatesNothing(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	c := fixtureCipher(t, make([]byte, core.AES256KeyBytes))
	text := strings.Repeat("A", 1<<20)
	maximum := textMaximum(t, 1)
	var got OpenResult
	var err error
	allocations := testing.AllocsPerRun(1, func() { got, err = c.OpenText(OpenTextRequest{Ciphertext: text, MaximumTextBytes: maximum}) })
	if allocations != 0 || got.Plaintext != nil || !errors.Is(err, core.ErrAESGCMTextExtent) || !errors.Is(err, core.ErrAESGCMContract) || !errors.Is(err, core.ErrPrimitiveContract) {
		t.Fatalf("extent refusal=(%gallocations,%x,%v),want 0/zero/exact typed parent chain", allocations, got.Plaintext, err)
	}
}

func FuzzAESGCMTextExtentIdentity(f *testing.F) {
	c := fixtureCipher(f, make([]byte, core.AES256KeyBytes))
	seed, err := c.SealText(SealTextRequest{Plaintext: []byte("owned"), MaximumTextBytes: textMaximum(f, 4096)})
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seed.String(), uint16(len(seed.String())-2))
	f.Add(seed.String(), uint16(len(seed.String())-1))
	f.Add("", uint16(0))
	f.Add("!", uint16(0))
	f.Fuzz(func(t *testing.T, text string, extent uint16) {
		maximum := uint64(extent) + 1
		got, err := c.OpenText(OpenTextRequest{Ciphertext: text, MaximumTextBytes: textMaximum(t, maximum)})
		over := uint64(len(text)) > maximum
		if errors.Is(err, core.ErrAESGCMTextExtent) != over {
			t.Fatalf("extent identity=%v,want overlimit%t", err, over)
		}
		if over && (!errors.Is(err, core.ErrAESGCMContract) || !errors.Is(err, core.ErrPrimitiveContract) || got.Plaintext != nil) {
			t.Fatalf("extent refusal=(%x,%v),want zero and parent chain", got.Plaintext, err)
		}
		if err == nil {
			encoded, err := c.SealText(SealTextRequest{Plaintext: got.Plaintext, MaximumTextBytes: textMaximum(t, maximum)})
			if err != nil || len(encoded.String()) != len(text) {
				t.Fatalf("admitted extent=(%d,%v),want exact encoded size%d", len(encoded.String()), err, len(text))
			}
		}
	})
}
