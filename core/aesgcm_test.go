package core

import (
	"errors"
	"testing"
)

func TestAESGCMCoreLayerTriad(t *testing.T) {
	t.Parallel()
	if AES256KeyBytes != 32 || AESGCMNonceBytes != 12 || AESGCMTagBytes != 16 || AESGCMOverheadBytes != 28 || AESGCMMaximumPlaintextBytes != 68719476704 {
		t.Fatalf("AES-GCM extents=(%d,%d,%d,%d,%d), want native (32,12,16,28,68719476704)", AES256KeyBytes, AESGCMNonceBytes, AESGCMTagBytes, AESGCMOverheadBytes, AESGCMMaximumPlaintextBytes)
	}
	if !errors.Is(ErrAESGCMAuthentication, ErrAESGCMContract) || !errors.Is(ErrAESGCMContract, ErrPrimitiveContract) {
		t.Fatalf("AES-GCM auth-to-contract/contract-to-Primitive = %t/%t, want true/true", errors.Is(ErrAESGCMAuthentication, ErrAESGCMContract), errors.Is(ErrAESGCMContract, ErrPrimitiveContract))
	}
	if errors.Is(ErrAESGCMContract, ErrAESGCMAuthentication) || errors.Is(ErrAttestVerification, ErrAESGCMAuthentication) {
		t.Fatalf("AES-GCM contract/foreign match authentication = %t/%t, want false/false", errors.Is(ErrAESGCMContract, ErrAESGCMAuthentication), errors.Is(ErrAttestVerification, ErrAESGCMAuthentication))
	}
	if err := PackageAESGCM.Validate(); err != nil {
		t.Fatal(err)
	}
	if got := PackageAESGCM.String(); got != "aesgcm" {
		t.Fatalf("package=%q, want aesgcm", got)
	}
}
