package passwordhash

import (
	"bytes"
	"testing"

	"golang.org/x/crypto/argon2"
)

var benchmarkDeriveSink []byte

// Derive at the fixed fixture cost: admission, validation and one native
// Argon2id derivation. The wrapper must add no work beyond the native engine.
func BenchmarkDerive(b *testing.B) {
	engine, err := New(fixtureLimits())
	if err != nil {
		b.Fatalf("New() error = %v, want nil", err)
	}
	request := fixtureRequest()
	if err := request.Validate(engine.limits); err != nil {
		b.Fatalf("Request.Validate() error = %v, want nil", err)
	}
	want := argon2.IDKey(request.Material, request.Salt, request.Parameters.Iterations, request.Parameters.MemoryKiB, request.Parameters.Parallelism, request.KeyBytes)
	b.ReportAllocs()
	for b.Loop() {
		key, err := engine.Derive(b.Context(), request)
		if err != nil {
			b.Fatalf("Derive() error = %v, want nil", err)
		}
		benchmarkDeriveSink = key
	}
	if !bytes.Equal(benchmarkDeriveSink, want) {
		b.Fatal("Derive() key differs from native Argon2id")
	}
}
