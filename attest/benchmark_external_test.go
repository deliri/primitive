package attest_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"io"
	"math"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
)

const benchmarkCanonicalChunkBytes = 8192

// benchmarkCanonicalBody reuses a fixed chunk and the standard library's
// reader. The previous sizedBody fixture allocated its chunk on every timed
// callback, accounting for most of the reported allocation volume.
type benchmarkCanonicalBody struct {
	reader bytes.Reader
	chunk  [benchmarkCanonicalChunkBytes]byte
	size   int
}

func (b *benchmarkCanonicalBody) Validate() error {
	if b.size < 1 || b.size > attest.CanonicalBodyMaximumBytes {
		return core.ErrAttestContract
	}
	return nil
}
func (*benchmarkCanonicalBody) AttestationDomain() testDomain { return testDomainPrimary }
func (b *benchmarkCanonicalBody) WriteCanonical(destination io.Writer) error {
	for remaining := b.size; remaining > 0; {
		count := min(remaining, len(b.chunk))
		b.reader.Reset(b.chunk[:count])
		if _, err := io.Copy(destination, &b.reader); err != nil {
			return err
		}
		remaining -= count
	}
	return nil
}

func BenchmarkCanonicalObjectReusedScalarBuffer(b *testing.B) {
	destination := make([]byte, 0, 128)
	facts := canonicalScalarFacts{Signed: math.MinInt64, Unsigned: math.MaxUint64, Flag: true}
	// Independent typed standard-library projection checks the exact three
	// scalars without adding verification to the timed work.
	want, err := core.MarshalCanonicalJSONDocument(struct {
		Minimum  int64  `json:"minimum"`
		Maximum  uint64 `json:"maximum"`
		Accepted bool   `json:"accepted"`
	}{Minimum: facts.Signed, Maximum: facts.Unsigned, Accepted: facts.Flag})
	if err != nil {
		b.Fatalf("scalar oracle error = %v, want nil", err)
	}
	b.ReportAllocs()
	for b.Loop() {
		object := attest.BeginCanonicalObject(destination[:0])
		object.Int64("minimum", facts.Signed)
		object.Uint64("maximum", facts.Unsigned)
		object.Bool("accepted", facts.Flag)
		destination, err = object.End()
		if err != nil {
			b.Fatalf("CanonicalObject.End() error = %v, want nil", err)
		}
	}
	if !bytes.Equal(destination, want) {
		b.Fatalf("canonical scalars = %q, want %q", destination, want)
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
	b.Helper()
	privateKey := deterministicPrivateKey(b, "benchmark-sign")
	body := &benchmarkCanonicalBody{size: size}
	request := attest.SignRequest[testDomain]{Body: body, Signer: privateKey}
	if err := request.Validate(); err != nil {
		b.Fatalf("SignRequest.Validate() error = %v, want nil", err)
	}
	digest := sha256.New()
	if err := body.WriteCanonical(digest); err != nil {
		b.Fatalf("hash fixture error = %v, want nil", err)
	}
	var wantDigest [sha256.Size]byte
	copy(wantDigest[:], digest.Sum(nil))
	b.SetBytes(int64(size))
	var got attest.Envelope[testDomain]
	var err error
	for b.Loop() {
		got, err = attest.Sign(request)
		if err != nil {
			b.Fatalf("Sign() error = %v, want nil", err)
		}
	}
	length, err := got.BodyLength.Uint64()
	if err != nil || length != uint64(size) || got.BodySHA256 != core.NewSHA256Digest(wantDigest) {
		b.Fatalf("signed facts = (%d, %v, %v), want %d bytes with independent digest", length, got.BodySHA256, err, size)
	}
	signature, signatureErr := got.Signature.Bytes()
	wantSignature := ed25519.Sign(privateKey, independentAttestationFrame(b, got))
	if signatureErr != nil || !bytes.Equal(signature[:], wantSignature) || got.Domain != testDomainPrimary || got.Signer != mustPublicKey(b, privateKey) {
		b.Fatalf("signed frame = %x, %v; want independent Ed25519 signature %x and exact domain/signer", signature, signatureErr, wantSignature)
	}
	proof, err := attest.Verify(attest.VerifyRequest[testDomain]{Body: body, Envelope: got, TrustedKeys: mustTrustedKeys(b, mustPublicKey(b, privateKey))})
	if err != nil {
		b.Fatalf("Verify(benchmark result) error = %v, want nil", err)
	}
	retained, err := proof.Envelope()
	if err != nil || retained != got {
		b.Fatalf("verified result = (%+v, %v), want %+v", retained, err, got)
	}
}

func BenchmarkVerifyCanonicalBody64KiB(b *testing.B) {
	b.ReportAllocs()
	benchmarkVerifyCanonicalBody(b, 64<<10, 1)
}
func BenchmarkVerifyCanonicalBodyMaximum(b *testing.B) {
	b.ReportAllocs()
	benchmarkVerifyCanonicalBody(b, attest.CanonicalBodyMaximumBytes, 1)
}
func benchmarkVerifyCanonicalBody(b *testing.B, size, trustCount int) {
	b.Helper()
	key := deterministicPrivateKey(b, "benchmark-verify")
	body := &benchmarkCanonicalBody{size: size}
	envelope := mustEnvelope(b, body, key)
	keys := make([]core.Ed25519PublicKey, trustCount)
	for index := range trustCount - 1 {
		keys[index] = mustPublicKey(b, deterministicPrivateKey(b, "benchmark-trust-"+strconv.Itoa(index)))
	}
	keys[trustCount-1] = mustPublicKey(b, key)
	request := attest.VerifyRequest[testDomain]{Body: body, Envelope: envelope, TrustedKeys: mustTrustedKeys(b, keys...)}
	if err := request.Validate(); err != nil {
		b.Fatalf("VerifyRequest.Validate() error = %v, want nil", err)
	}
	b.SetBytes(int64(size))
	var got attest.Verified[testDomain]
	var err error
	for b.Loop() {
		got, err = attest.Verify(request)
		if err != nil {
			b.Fatalf("Verify() error = %v, want nil", err)
		}
	}
	retained, err := got.Envelope()
	if err != nil || retained != envelope {
		b.Fatalf("verified result = (%+v, %v), want %+v", retained, err, envelope)
	}
}

func BenchmarkEnvelopeMarshalJSON(b *testing.B) {
	envelope := mustEnvelope(b, builtBody{commit: "benchmark-json", count: 1}, deterministicPrivateKey(b, "benchmark-json"))
	var got []byte
	var err error
	b.ReportAllocs()
	for b.Loop() {
		got, err = envelope.MarshalJSON()
		if err != nil {
			b.Fatalf("MarshalJSON() error = %v, want nil", err)
		}
	}
	var decoded attest.Envelope[testDomain]
	if err := decoded.UnmarshalJSON(got); err != nil || decoded != envelope {
		b.Fatalf("decoded benchmark result = (%+v, %v), want %+v", decoded, err, envelope)
	}
}

func BenchmarkEnvelopeUnmarshalJSON(b *testing.B) {
	envelope := mustEnvelope(b, builtBody{commit: "benchmark-json", count: 1}, deterministicPrivateKey(b, "benchmark-json"))
	encoded, err := envelope.MarshalJSON()
	if err != nil {
		b.Fatalf("MarshalJSON(setup) error = %v, want nil", err)
	}
	b.SetBytes(int64(len(encoded)))
	b.ReportAllocs()
	var got attest.Envelope[testDomain]
	for b.Loop() {
		if err := got.UnmarshalJSON(encoded); err != nil {
			b.Fatalf("UnmarshalJSON() error = %v, want nil", err)
		}
	}
	if got != envelope {
		b.Fatalf("decoded benchmark result = %+v, want %+v", got, envelope)
	}
}

func BenchmarkSignCanonicalBodyMinimum(b *testing.B) {
	b.ReportAllocs()
	benchmarkSignCanonicalBody(b, 1)
}

func BenchmarkVerifyCanonicalBodyMinimum(b *testing.B) {
	b.ReportAllocs()
	benchmarkVerifyCanonicalBody(b, 1, 1)
}

func BenchmarkVerifyCanonicalBodyMaximumTrust(b *testing.B) {
	b.ReportAllocs()
	benchmarkVerifyCanonicalBody(b, 1, attest.TrustedKeyMaximumCount)
}
