package attest_test

import (
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
)

func BenchmarkCanonicalObjectReusedScalarBuffer(b *testing.B) {
	destination := make([]byte, 0, 128)
	write := func() {
		object := attest.BeginCanonicalObject(destination[:0])
		object.Int64("minimum", math.MinInt64)
		object.Uint64("maximum", math.MaxUint64)
		object.Bool("accepted", true)
		encoded, err := object.End()
		if err != nil {
			b.Fatalf("CanonicalObject.End() error = %v, want nil", err)
		}
		destination = encoded[:0]
	}
	if got := testing.AllocsPerRun(100, write); got != 0 {
		b.Fatalf("reused scalar CanonicalObject allocations = %.0f, want 0", got)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		write()
	}
}

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
	request := attest.SignRequest[testDomain]{Body: body, Signer: privateKey}
	b.ResetTimer()

	for b.Loop() {
		if _, err := attest.Sign(request); err != nil {
			b.Fatalf("attest.Sign() error = %v, want nil", err)
		}
	}
}
