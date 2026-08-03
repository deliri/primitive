package attest_test

import (
	"crypto"
	"crypto/ed25519"
	"crypto/sha256"
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
)

type externalSignerMode uint8

const (
	externalSignerValid externalSignerMode = iota + 1
	externalSignerPublicPanic
	externalSignerPublicWrongType
	externalSignerPublicShort
	externalSignerPublicLong
	externalSignerPublicDifferent
	externalSignerSignError
	externalSignerSignPanic
	externalSignerSignatureNil
	externalSignerSignatureShort
	externalSignerSignatureLong
	externalSignerSignatureCorrupt
	externalSignerMutatesFrame
)

type externalSignerObservation struct {
	frameExtent int
	hash        crypto.Hash
	randomSet   bool
}

type externalSigner struct {
	observation *externalSignerObservation
	key         ed25519.PrivateKey
	mode        externalSignerMode
}

func (s externalSigner) Public() crypto.PublicKey {
	switch s.mode {
	case externalSignerPublicPanic:
		panic(fixtureErrorValidation)
	case externalSignerPublicWrongType:
		return "not an ed25519 public key"
	case externalSignerPublicShort:
		return ed25519.PublicKey(make([]byte, ed25519.PublicKeySize-1))
	case externalSignerPublicLong:
		return ed25519.PublicKey(make([]byte, ed25519.PublicKeySize+1))
	case externalSignerPublicDifferent:
		return deterministicPrivateKeyForLabel("different-public-key").Public()
	default:
		return s.key.Public()
	}
}

func (s externalSigner) Sign(
	random io.Reader,
	frame []byte,
	opts crypto.SignerOpts,
) ([]byte, error) {
	if s.observation != nil {
		s.observation.frameExtent = len(frame)
		s.observation.hash = opts.HashFunc()
		s.observation.randomSet = random != nil
	}
	switch s.mode {
	case externalSignerSignError:
		return nil, fixtureErrorSign
	case externalSignerSignPanic:
		panic(fixtureErrorSign)
	case externalSignerSignatureNil:
		return nil, nil
	case externalSignerSignatureShort:
		return make([]byte, ed25519.SignatureSize-1), nil
	case externalSignerSignatureLong:
		return make([]byte, ed25519.SignatureSize+1), nil
	case externalSignerSignatureCorrupt:
		return make([]byte, ed25519.SignatureSize), nil
	case externalSignerMutatesFrame:
		frame[0] ^= 0xff
	}
	return s.key.Sign(random, frame, opts)
}

func TestSignPublicStandardSignerBoundaryMatrix(t *testing.T) {
	t.Parallel()

	cases := []struct {
		wantErr    error
		wantNative error
		name       string
		mode       externalSignerMode
	}{
		{name: "standard signer seals", mode: externalSignerValid},
		{name: "public callback panic rejects", mode: externalSignerPublicPanic, wantErr: core.ErrAttestContract},
		{name: "non ed25519 public key rejects", mode: externalSignerPublicWrongType, wantErr: core.ErrAttestContract},
		{name: "short ed25519 public key rejects", mode: externalSignerPublicShort, wantErr: core.ErrAttestContract},
		{name: "long ed25519 public key rejects", mode: externalSignerPublicLong, wantErr: core.ErrAttestContract},
		{name: "different public key rejects signature", mode: externalSignerPublicDifferent, wantErr: core.ErrAttestContract},
		{name: "provider error remains reachable", mode: externalSignerSignError, wantErr: core.ErrAttestContract, wantNative: fixtureErrorSign},
		{name: "provider panic rejects", mode: externalSignerSignPanic, wantErr: core.ErrAttestContract},
		{name: "nil signature rejects", mode: externalSignerSignatureNil, wantErr: core.ErrAttestContract},
		{name: "short signature rejects", mode: externalSignerSignatureShort, wantErr: core.ErrAttestContract},
		{name: "long signature rejects", mode: externalSignerSignatureLong, wantErr: core.ErrAttestContract},
		{name: "corrupt exact signature rejects", mode: externalSignerSignatureCorrupt, wantErr: core.ErrAttestContract},
		{name: "provider frame mutation rejects", mode: externalSignerMutatesFrame, wantErr: core.ErrAttestContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			key := deterministicPrivateKey(t, "external-signer")
			observation := &externalSignerObservation{}
			gotEnvelope, gotErr := attest.Sign(attest.SignRequest[testDomain]{
				Body: literalBody{domain: testDomainPrimary, value: []byte("external signer")},
				Signer: externalSigner{
					key:         key,
					observation: observation,
					mode:        tc.mode,
				},
			})
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("attest.Sign() error = %v, want %v", gotErr, tc.wantErr)
			}
			if tc.wantNative != nil && !errors.Is(gotErr, tc.wantNative) {
				t.Fatalf("attest.Sign() native error = %v, want %v", gotErr, tc.wantNative)
			}
			if tc.wantErr != nil {
				if gotEnvelope != (attest.Envelope[testDomain]{}) {
					t.Fatalf("attest.Sign() envelope = %+v, want zero", gotEnvelope)
				}
				return
			}
			if observation.frameExtent == 0 || observation.hash != crypto.Hash(0) || !observation.randomSet {
				t.Fatalf(
					"crypto.Signer.Sign() observed = {frame:%d hash:%v random:%t}, want nonempty frame, hash zero, random reader",
					observation.frameExtent,
					observation.hash,
					observation.randomSet,
				)
			}
			gotVerified, gotVerifyErr := attest.Verify(attest.VerifyRequest[testDomain]{
				Body:        literalBody{domain: testDomainPrimary, value: []byte("external signer")},
				Envelope:    gotEnvelope,
				TrustedKeys: mustTrustedKeys(t, mustPublicKey(t, key)),
			})
			if gotVerifyErr != nil {
				t.Fatalf("attest.Verify() error = %v, want nil", gotVerifyErr)
			}
			if gotValidateErr := gotVerified.Validate(); gotValidateErr != nil {
				t.Fatalf("Verified.Validate() error = %v, want nil", gotValidateErr)
			}
		})
	}
}

func TestSignRequestStandardSignerValidationBoundary(t *testing.T) {
	t.Parallel()

	key := deterministicPrivateKey(t, "signer-validation")
	cases := []struct {
		signer  crypto.Signer
		wantErr error
		name    string
	}{
		{name: "private key value accepts", signer: key},
		{name: "private key pointer accepts", signer: &key},
		{name: "standard signer accepts", signer: externalSigner{key: key, mode: externalSignerValid}},
		{name: "missing signer rejects", wantErr: core.ErrAttestContract},
		{name: "typed nil private key rejects", signer: (*ed25519.PrivateKey)(nil), wantErr: core.ErrAttestContract},
		{name: "public callback panic rejects", signer: externalSigner{key: key, mode: externalSignerPublicPanic}, wantErr: core.ErrAttestContract},
		{name: "non ed25519 public key rejects", signer: externalSigner{key: key, mode: externalSignerPublicWrongType}, wantErr: core.ErrAttestContract},
		{name: "short public key rejects", signer: externalSigner{key: key, mode: externalSignerPublicShort}, wantErr: core.ErrAttestContract},
		{name: "long public key rejects", signer: externalSigner{key: key, mode: externalSignerPublicLong}, wantErr: core.ErrAttestContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			request := attest.SignRequest[testDomain]{
				Body:   literalBody{domain: testDomainPrimary, value: []byte("validate signer")},
				Signer: tc.signer,
			}
			gotErr := request.Validate()
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("SignRequest.Validate() error = %v, want %v", gotErr, tc.wantErr)
			}
		})
	}
}

func deterministicPrivateKeyForLabel(label string) ed25519.PrivateKey {
	seed := sha256.Sum256([]byte(label))
	return ed25519.NewKeyFromSeed(seed[:])
}
