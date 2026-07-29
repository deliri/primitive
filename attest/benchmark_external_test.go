package attest_test

import (
	"testing"

	"github.com/deliri/primitive/v2026/attest"
)

func BenchmarkSignCanonicalBody64KiB(b *testing.B) {
	b.ReportAllocs()
	benchmarkSignCanonicalBody(b, 64<<10)
}

func BenchmarkSignCanonicalBodyMaximum(b *testing.B) {
	b.ReportAllocs()
	benchmarkSignCanonicalBody(b, attest.CanonicalBodyMaximumBytes)
}

func benchmarkSignCanonicalBody(b *testing.B, size int) {
	privateKey := deterministicPrivateKey(b, "benchmark-sign")
	body := sizedBody{
		size:      size,
		chunkSize: 8192,
		domain:    testDomainPrimary,
	}
	request := attest.SignRequest[testDomain]{Body: body, Key: privateKey}
	b.ResetTimer()

	for b.Loop() {
		if _, err := attest.Sign(request); err != nil {
			b.Fatalf("attest.Sign() error = %v, want nil", err)
		}
	}
}
