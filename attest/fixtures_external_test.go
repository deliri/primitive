package attest_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/sha256"
	"io"
	"strconv"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
)

type testDomain uint8

const (
	testDomainUnknown testDomain = iota
	testDomainPrimary
	testDomainAlternate
)

const (
	testDomainPrimaryText   = "test-primary-2026"
	testDomainAlternateText = "test-alternate-2026"
)

func (d testDomain) Validate() error {
	switch d {
	case testDomainPrimary, testDomainAlternate:
		return nil
	default:
		return core.ErrAttestContract
	}
}

func (d testDomain) MarshalText() ([]byte, error) {
	switch d {
	case testDomainPrimary:
		return []byte(testDomainPrimaryText), nil
	case testDomainAlternate:
		return []byte(testDomainAlternateText), nil
	default:
		return nil, core.ErrAttestContract
	}
}

func (testDomain) ParseCanonicalText(text []byte) (testDomain, error) {
	switch string(text) {
	case testDomainPrimaryText:
		return testDomainPrimary, nil
	case testDomainAlternateText:
		return testDomainAlternate, nil
	default:
		return testDomainUnknown, core.ErrAttestContract
	}
}

type literalBody struct {
	value  []byte
	domain testDomain
}

func (b literalBody) Validate() error {
	return b.domain.Validate()
}

func (b literalBody) AttestationDomain() testDomain {
	return b.domain
}

func (b literalBody) WriteCanonical(destination io.Writer) error {
	_, err := destination.Write(b.value)
	return err
}

type sizedBody struct {
	size      int
	chunkSize int
	domain    testDomain
	ignoreErr bool
}

func (b sizedBody) Validate() error {
	if b.size < 0 || b.chunkSize < 1 {
		return core.ErrAttestContract
	}
	return b.domain.Validate()
}

func (b sizedBody) AttestationDomain() testDomain {
	return b.domain
}

func (b sizedBody) WriteCanonical(destination io.Writer) error {
	var storage [8192]byte
	chunk := storage[:min(b.chunkSize, len(storage))]
	remaining := b.size
	for remaining > 0 {
		count := min(remaining, len(chunk))
		written, err := destination.Write(chunk[:count])
		if err != nil && !b.ignoreErr {
			return err
		}
		if err == nil && written != count {
			return io.ErrShortWrite
		}
		remaining -= count
	}
	return nil
}

type fixtureError uint8

const (
	fixtureErrorValidation fixtureError = iota + 1
	fixtureErrorWrite
	fixtureErrorMarshal
	fixtureErrorSign
)

func (e fixtureError) Error() string {
	return "fixture error " + strconv.Itoa(int(e))
}

type hostileBodyMode uint8

const (
	hostileBodyValidationError hostileBodyMode = iota + 1
	hostileBodyValidationPanic
	hostileBodyDomainPanic
	hostileBodyWriteError
	hostileBodyWritePanic
	hostileBodyZeroWrite
)

type hostileBody struct {
	mode hostileBodyMode
}

func (b hostileBody) Validate() error {
	switch b.mode {
	case hostileBodyValidationError:
		return fixtureErrorValidation
	case hostileBodyValidationPanic:
		panic(fixtureErrorValidation)
	default:
		return nil
	}
}

func (b hostileBody) AttestationDomain() testDomain {
	if b.mode == hostileBodyDomainPanic {
		panic(fixtureErrorValidation)
	}
	return testDomainPrimary
}

func (b hostileBody) WriteCanonical(destination io.Writer) error {
	switch b.mode {
	case hostileBodyWriteError:
		return fixtureErrorWrite
	case hostileBodyWritePanic:
		panic(fixtureErrorWrite)
	case hostileBodyZeroWrite:
		_, err := destination.Write(nil)
		return err
	default:
		_, err := io.WriteString(destination, "x")
		return err
	}
}

type retainingBody struct {
	retained *io.Writer
}

func (retainingBody) Validate() error {
	return nil
}

func (retainingBody) AttestationDomain() testDomain {
	return testDomainPrimary
}

func (b retainingBody) WriteCanonical(destination io.Writer) error {
	*b.retained = destination
	_, err := io.WriteString(destination, "x")
	return err
}

type keyMutatingBody struct {
	key ed25519.PrivateKey
}

func (keyMutatingBody) Validate() error {
	return nil
}

func (keyMutatingBody) AttestationDomain() testDomain {
	return testDomainPrimary
}

func (b keyMutatingBody) WriteCanonical(destination io.Writer) error {
	clear(b.key)
	_, err := io.WriteString(destination, "x")
	return err
}

func deterministicPrivateKey(t testing.TB, label string) ed25519.PrivateKey {
	t.Helper()
	seed := sha256.Sum256([]byte(label))
	return ed25519.NewKeyFromSeed(seed[:])
}

func mustPublicKey(t testing.TB, privateKey ed25519.PrivateKey) core.Ed25519PublicKey {
	t.Helper()
	publicKey, err := core.NewEd25519PublicKey(privateKey.Public().(ed25519.PublicKey))
	if err != nil {
		t.Fatalf("core.NewEd25519PublicKey() error = %v, want nil", err)
	}
	return publicKey
}

func mustTrustedKeys(t testing.TB, keys ...core.Ed25519PublicKey) attest.TrustedKeys {
	t.Helper()
	trusted, err := attest.NewTrustedKeys(attest.TrustedKeysRequest{
		Keys: append([]core.Ed25519PublicKey(nil), keys...),
	})
	if err != nil {
		t.Fatalf("attest.NewTrustedKeys() error = %v, want nil", err)
	}
	return trusted
}

func mustEnvelope(
	t testing.TB,
	body attest.CanonicalBody[testDomain],
	privateKey ed25519.PrivateKey,
) attest.Envelope[testDomain] {
	t.Helper()
	envelope, err := attest.Sign(attest.SignRequest[testDomain]{
		Body:   body,
		Signer: append(ed25519.PrivateKey(nil), privateKey...),
	})
	if err != nil {
		t.Fatalf("attest.Sign() error = %v, want nil", err)
	}
	return envelope
}

func mustSignature(t testing.TB, hexadecimal string) attest.Signature {
	t.Helper()
	var signature attest.Signature
	wire := strconv.AppendQuote(nil, hexadecimal)
	if err := signature.UnmarshalJSON(wire); err != nil {
		t.Fatalf("Signature.UnmarshalJSON() error = %v, want nil", err)
	}
	return signature
}

func mutateDigest(t testing.TB, digest core.SHA256Digest) core.SHA256Digest {
	t.Helper()
	raw, err := digest.Bytes()
	if err != nil {
		t.Fatalf("SHA256Digest.Bytes() error = %v, want nil", err)
	}
	raw[0] ^= 1
	return core.NewSHA256Digest(raw)
}

func mutateSignature(t testing.TB, signature attest.Signature) attest.Signature {
	t.Helper()
	hexadecimal, err := signature.Hex()
	if err != nil {
		t.Fatalf("Signature.Hex() error = %v, want nil", err)
	}
	replacement := byte('0')
	if hexadecimal[0] == replacement {
		replacement = '1'
	}
	return mustSignature(t, string(replacement)+hexadecimal[1:])
}

func mutateLength(t testing.TB, length core.ByteCount) core.ByteCount {
	t.Helper()
	value, err := length.Uint64()
	if err != nil {
		t.Fatalf("ByteCount.Uint64() error = %v, want nil", err)
	}
	mutated, err := core.NewByteCount(value + 1)
	if err != nil {
		t.Fatalf("core.NewByteCount() error = %v, want nil", err)
	}
	return mutated
}

func copyLiteralBody(body literalBody) literalBody {
	return literalBody{
		value:  bytes.Clone(body.value),
		domain: body.domain,
	}
}
