package keygen_test

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/keygen"
)

func TestSigningKeyFormattingIsAlwaysExactlyRedacted(t *testing.T) {
	t.Parallel()

	key, gotErr := keygen.GenerateSigningKey()
	if gotErr != nil {
		t.Fatalf("GenerateSigningKey() error = %v, want nil", gotErr)
	}
	cases := []struct {
		name   string
		format string
	}{
		{name: "default verb redacts", format: "%v"},
		{name: "field verb redacts", format: "%+v"},
		{name: "Go syntax verb redacts", format: "%#v"},
		{name: "string verb redacts", format: "%s"},
		{name: "quoted string verb redacts", format: "%q"},
		{name: "lower hexadecimal verb redacts", format: "%x"},
		{name: "upper hexadecimal verb redacts", format: "%X"},
		{name: "decimal verb redacts", format: "%d"},
		{name: "binary verb redacts", format: "%b"},
		{name: "Unicode verb redacts", format: "%U"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := fmt.Sprintf(tc.format, key)
			if got != core.RedactedValueText {
				t.Fatalf(
					"fmt.Sprintf(%q, SigningKey) = %q, want %q",
					tc.format,
					got,
					core.RedactedValueText,
				)
			}
		})
	}
}

func TestPrivateKeyProjectionIsAnIndependentCallerOwnedCopy(t *testing.T) {
	t.Parallel()

	key, gotErr := keygen.GenerateSigningKey()
	if gotErr != nil {
		t.Fatalf("GenerateSigningKey() error = %v, want nil", gotErr)
	}
	first, gotFirstErr := key.PrivateKey()
	if gotFirstErr != nil {
		t.Fatalf("SigningKey.PrivateKey(first) error = %v, want nil", gotFirstErr)
	}
	second, gotSecondErr := key.PrivateKey()
	if gotSecondErr != nil {
		t.Fatalf("SigningKey.PrivateKey(second) error = %v, want nil", gotSecondErr)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("independent private-key equality before mutation = false, want true")
	}
	for index := range first {
		first[index] ^= 0xff
	}
	third, gotThirdErr := key.PrivateKey()
	if gotThirdErr != nil {
		t.Fatalf("SigningKey.PrivateKey(after mutation) error = %v, want nil", gotThirdErr)
	}
	if !bytes.Equal(second, third) {
		t.Fatal("owned private-key equality after caller mutation = false, want true")
	}
	clear(first)
	clear(second)
	clear(third)
}

func TestSigningKeyCopiesShareCoreOwnedDestruction(t *testing.T) {
	t.Parallel()

	key, gotErr := keygen.GenerateSigningKey()
	if gotErr != nil {
		t.Fatalf("GenerateSigningKey() error = %v, want nil", gotErr)
	}
	copyOfKey := key
	privateBeforeDestroy, gotPrivateErr := copyOfKey.PrivateKey()
	if gotPrivateErr != nil {
		t.Fatalf("SigningKey.PrivateKey() error = %v, want nil", gotPrivateErr)
	}
	defer clear(privateBeforeDestroy)
	if gotDestroyErr := key.Destroy(); gotDestroyErr != nil {
		t.Fatalf("SigningKey.Destroy() error = %v, want nil", gotDestroyErr)
	}
	for _, candidate := range []keygen.SigningKey{key, copyOfKey} {
		if gotValidateErr := candidate.Validate(); !errors.Is(gotValidateErr, core.ErrKeygenContract) ||
			!errors.Is(gotValidateErr, core.ErrPrimitiveContract) {
			t.Fatalf("destroyed SigningKey.Validate() error = %v, want %v and %v", gotValidateErr, core.ErrKeygenContract, core.ErrPrimitiveContract)
		}
		if gotPrivate, gotPrivateErr := candidate.PrivateKey(); gotPrivate != nil ||
			!errors.Is(gotPrivateErr, core.ErrKeygenContract) {
			t.Fatalf("destroyed SigningKey.PrivateKey() = (%v, %v), want (nil, %v)", gotPrivate, gotPrivateErr, core.ErrKeygenContract)
		}
	}
	if gotDestroyErr := copyOfKey.Destroy(); gotDestroyErr != nil {
		t.Fatalf("repeated SigningKey.Destroy() error = %v, want nil", gotDestroyErr)
	}
}

func TestZeroSigningKeyRejectsEveryOwnedBoundary(t *testing.T) {
	t.Parallel()

	var key keygen.SigningKey
	cases := []struct {
		run  func() error
		name string
	}{
		{name: "validation rejects", run: key.Validate},
		{name: "public projection rejects", run: func() error { _, err := key.PublicKey(); return err }},
		{name: "private projection rejects", run: func() error { _, err := key.PrivateKey(); return err }},
		{name: "destruction rejects unset state", run: key.Destroy},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			gotErr := tc.run()
			if !errors.Is(gotErr, core.ErrKeygenContract) ||
				!errors.Is(gotErr, core.ErrPrimitiveContract) {
				t.Fatalf("zero SigningKey boundary error = %v, want %v and %v", gotErr, core.ErrKeygenContract, core.ErrPrimitiveContract)
			}
		})
	}
}
