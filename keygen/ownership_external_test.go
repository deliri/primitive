package keygen_test

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"fmt"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/keygen"
)

func TestSigningKeyCustodyAndProjectionLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name              string
		active, destroyed bool
		wantErr           error
	}{
		{name: "unissued key refuses every projection", wantErr: core.ErrKeygenContract},
		{name: "active key projects independent Go values", active: true},
		{name: "destroyed copy refuses every projection", active: true, destroyed: true, wantErr: core.ErrKeygenContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var key keygen.SigningKey
			seed := nonZeroSeed()
			if tc.active {
				var err error
				key, err = keygen.AdoptSigningKey(seed)
				if err != nil {
					t.Fatalf("AdoptSigningKey(fixture) error = %v, want nil", err)
				}
				t.Cleanup(func() {
					if err := key.Destroy(); err != nil {
						t.Fatalf("Destroy(cleanup) error = %v, want nil", err)
					}
				})
			}
			copied := key
			if tc.destroyed {
				if err := copied.Destroy(); err != nil {
					t.Fatalf("Destroy(copy) error = %v, want nil", err)
				}
			}
			if err := key.Validate(); !errors.Is(err, tc.wantErr) {
				t.Fatalf("Validate() error = %v, want %v", err, tc.wantErr)
			}
			gotSeed, seedErr := key.Seed()
			defer clear(gotSeed[:])
			gotPublic, publicErr := key.PublicKey()
			gotPrivate, privateErr := key.PrivateKey()
			defer clear(gotPrivate)
			if !errors.Is(seedErr, tc.wantErr) || !errors.Is(publicErr, tc.wantErr) || !errors.Is(privateErr, tc.wantErr) {
				t.Fatalf("projection errors = (%v,%v,%v), want %v", seedErr, publicErr, privateErr, tc.wantErr)
			}
			if tc.wantErr != nil {
				if gotSeed != ([keygen.SeedSize]byte{}) || gotPublic != (core.Ed25519PublicKey{}) || gotPrivate != nil {
					t.Fatalf("refused projections = (%x,%v,%x), want zero,zero,nil", gotSeed, gotPublic, gotPrivate)
				}
				destroyErr := key.Destroy()
				wantDestroyErr := tc.wantErr
				if tc.destroyed {
					wantDestroyErr = nil
				}
				if !errors.Is(destroyErr, wantDestroyErr) {
					t.Fatalf("Destroy() error = %v, want %v", destroyErr, wantDestroyErr)
				}
				return
			}
			wantPrivate := ed25519.NewKeyFromSeed(seed[:])
			defer clear(wantPrivate)
			if gotSeed != seed || !bytes.Equal(gotPrivate, wantPrivate) {
				t.Fatalf("projections = (%x,%x), want (%x,%x)", gotSeed, gotPrivate, seed, wantPrivate)
			}
			for index := range gotPrivate {
				gotPrivate[index] ^= 0xff
			}
			clear(gotSeed[:])
			after, err := key.PrivateKey()
			defer clear(after)
			if err != nil || !bytes.Equal(after, wantPrivate) {
				t.Fatalf("PrivateKey(after caller mutation) = (%x,%v), want (%x,nil)", after, err, wantPrivate)
			}
			if err := copied.Destroy(); err != nil {
				t.Fatalf("Destroy(copy) error = %v, want nil", err)
			}
			// Previously projected caller-owned material remains the caller's, even
			// though every future projection through either handle must refuse.
			if !bytes.Equal(after, wantPrivate) {
				t.Fatalf("caller-owned private bytes = %x, want preserved %x", after, wantPrivate)
			}
			refused, err := key.PrivateKey()
			if refused != nil || !errors.Is(err, core.ErrKeygenContract) {
				t.Fatalf("PrivateKey(after copy destroy) = (%x,%v), want nil and Core refusal", refused, err)
			}
		})
	}
}

func TestSigningKeyFormattingNeverDisclosesCustody(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, format string }{
		{name: "default fields", format: "%v"}, {name: "named fields", format: "%+v"}, {name: "Go field syntax", format: "%#v"},
		{name: "text", format: "%s"}, {name: "quoted text", format: "%q"}, {name: "hex bytes", format: "%x"},
		{name: "width and precision cannot expose backing storage", format: "%100.1v"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			key, err := keygen.AdoptSigningKey(nonZeroSeed())
			if err != nil {
				t.Fatalf("AdoptSigningKey(fixture) error = %v, want nil", err)
			}
			t.Cleanup(func() {
				if err := key.Destroy(); err != nil {
					t.Fatalf("Destroy() error = %v, want nil", err)
				}
			})
			for _, value := range []keygen.SigningKey{key, {}} {
				got := fmt.Sprintf(tc.format, value)
				if got != core.RedactedValueText {
					t.Fatalf("formatted key = %q, want %q", got, core.RedactedValueText)
				}
			}
			if err := key.Destroy(); err != nil {
				t.Fatalf("Destroy() error = %v, want nil", err)
			}
			if got := fmt.Sprintf(tc.format, key); got != core.RedactedValueText {
				t.Fatalf("formatted destroyed key = %q, want %q", got, core.RedactedValueText)
			}
		})
	}
}
