package keygen_test

import (
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/keygen"
)

func BenchmarkGenerateSigningKey(b *testing.B) {
	b.ReportAllocs()

	for b.Loop() {
		key, gotErr := keygen.GenerateSigningKey()
		if gotErr != nil {
			b.Fatalf("GenerateSigningKey() error = %v, want nil", gotErr)
		}
		if gotDestroyErr := key.Destroy(); gotDestroyErr != nil {
			b.Fatalf("SigningKey.Destroy() error = %v, want nil", gotDestroyErr)
		}
	}
}

func BenchmarkGenerateMaximumSecret(b *testing.B) {
	size, gotSizeErr := core.NewByteCount(core.SecretMaterialMaximumBytes)
	if gotSizeErr != nil {
		b.Fatalf("core.NewByteCount() error = %v, want nil", gotSizeErr)
	}
	request := keygen.SecretRequest{Size: size}
	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		material, gotErr := keygen.GenerateSecret(request)
		if gotErr != nil {
			b.Fatalf("GenerateSecret() error = %v, want nil", gotErr)
		}
		if gotDestroyErr := material.Destroy(); gotDestroyErr != nil {
			b.Fatalf("SecretMaterial.Destroy() error = %v, want nil", gotDestroyErr)
		}
	}
}
