package attest_test

import (
	"bytes"
	"crypto/ed25519"
	"encoding/binary"
	"encoding/hex"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
)

const (
	fixedVectorFrameHex     = "7072696d69746976652d6174746573746174696f6e2d32303236000011746573742d7072696d6172792d3230323693e65b7f828c1414d0445a2c265555a95ba4a7d0f701942cba9496402149e81b000000000000000b48849437e72bb54e2c969c845facb59a92c116ff231097b0a173bb89ad5fdb7e"
	fixedVectorSignatureHex = "419f53e73e7dc10ebd695d79db9fd9434c07bf4c3e6b03fb4cce26b7dc70132ba3035d26d754a8debe6315995909fe2b216062ae9c0aaad67f10f6e42630f505"
)

func TestSignPublicMatchesIndependentEd25519FrameOracleAndFixedVector(t *testing.T) {
	t.Parallel()

	privateKey := deterministicPrivateKey(t, "attest-fixed-vector-2026")
	body := literalBody{
		domain: testDomainPrimary,
		value:  []byte{0, 1, 2, 3, 0xff, 'a', 't', 't', 'e', 's', 't'},
	}
	envelope := mustEnvelope(t, body, privateKey)
	gotFrame := independentAttestationFrame(t, envelope)
	gotSignature, gotSignatureErr := envelope.Signature.Bytes()
	if gotSignatureErr != nil {
		t.Fatalf("Signature.Bytes() error = %v, want nil", gotSignatureErr)
	}
	gotDirect := ed25519.Sign(privateKey, gotFrame)
	if !bytes.Equal(gotSignature[:], gotDirect) {
		t.Fatalf(
			"attest signature = %x, want direct ed25519.Sign(frame) %x",
			gotSignature,
			gotDirect,
		)
	}
	publicKey, gotPublicKeyErr := envelope.Signer.Bytes()
	if gotPublicKeyErr != nil {
		t.Fatalf("Ed25519PublicKey.Bytes() error = %v, want nil", gotPublicKeyErr)
	}
	if !ed25519.Verify(publicKey, gotFrame, gotSignature[:]) {
		t.Fatal("ed25519.Verify(independent frame) = false, want true")
	}

	gotFrameHex := hex.EncodeToString(gotFrame)
	gotSignatureHex := hex.EncodeToString(gotSignature[:])
	if gotFrameHex != fixedVectorFrameHex || gotSignatureHex != fixedVectorSignatureHex {
		t.Fatalf(
			"fixed vector =\nframe %s\nsignature %s\nwant\nframe %s\nsignature %s",
			gotFrameHex,
			gotSignatureHex,
			fixedVectorFrameHex,
			fixedVectorSignatureHex,
		)
	}
}

func independentAttestationFrame(
	t testing.TB,
	envelope attest.Envelope[testDomain],
) []byte {
	t.Helper()
	domain, gotDomainErr := envelope.Domain.MarshalText()
	if gotDomainErr != nil {
		t.Fatalf("SigningDomain.MarshalText() error = %v, want nil", gotDomainErr)
	}
	publicKey, gotPublicKeyErr := envelope.Signer.Bytes()
	if gotPublicKeyErr != nil {
		t.Fatalf("Ed25519PublicKey.Bytes() error = %v, want nil", gotPublicKeyErr)
	}
	bodyLength, gotBodyLengthErr := envelope.BodyLength.Uint64()
	if gotBodyLengthErr != nil {
		t.Fatalf("ByteCount.Uint64() error = %v, want nil", gotBodyLengthErr)
	}
	bodyDigest, gotBodyDigestErr := envelope.BodySHA256.Bytes()
	if gotBodyDigestErr != nil {
		t.Fatalf("SHA256Digest.Bytes() error = %v, want nil", gotBodyDigestErr)
	}
	frame := []byte("primitive-attestation-2026")
	frame = append(frame, 0)
	frame = binary.BigEndian.AppendUint16(frame, uint16(len(domain)))
	frame = append(frame, domain...)
	frame = append(frame, publicKey...)
	frame = binary.BigEndian.AppendUint64(frame, bodyLength)
	return append(frame, bodyDigest[:]...)
}
