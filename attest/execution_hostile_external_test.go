package attest_test

import (
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
)

type bodyExit uint8

const (
	bodyExitComplete bodyExit = iota
	bodyExitPartialError
	bodyExitPartialPanic
	bodyExitIgnoredOverflow
)

type bodyExecution struct {
	retained io.Writer
	writes   int
}
type observedExecutionBody struct {
	observation *bodyExecution
	exit        bodyExit
}

func (observedExecutionBody) Validate() error               { return nil }
func (observedExecutionBody) AttestationDomain() testDomain { return testDomainPrimary }
func (b observedExecutionBody) WriteCanonical(destination io.Writer) error {
	b.observation.writes++
	b.observation.retained = destination
	if _, err := io.WriteString(destination, "x"); err != nil {
		return err
	}
	switch b.exit {
	case bodyExitComplete:
		return nil
	case bodyExitPartialError:
		return fixtureErrorWrite
	case bodyExitPartialPanic:
		panic(fixtureErrorWrite)
	case bodyExitIgnoredOverflow:
		// The first byte plus this exact maximum exceeds the contract. The
		// defective owner ignores the writer's refusal and attempts recovery.
		body := sizedBody{size: attest.CanonicalBodyMaximumBytes, chunkSize: 8192, domain: testDomainPrimary, ignoreErr: true}
		_ = body.WriteCanonical(destination)
		_, _ = io.WriteString(destination, "late")
		return nil
	default:
		return core.ErrAttestContract
	}
}

func TestSignCanonicalWriterTerminalExitMatrix(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name          string
		exit          bodyExit
		wantErr       error
		wantNative    error
		wantSignCalls int
	}{
		{name: "completed stream signs once and closes capability", exit: bodyExitComplete, wantSignCalls: 1},
		{name: "partial writer error preserves identity and never signs", exit: bodyExitPartialError, wantErr: core.ErrAttestContract, wantNative: fixtureErrorWrite},
		{name: "partial writer panic creates no envelope", exit: bodyExitPartialPanic, wantErr: core.ErrAttestContract},
		{name: "ignored overflow remains terminal after smaller writes", exit: bodyExitIgnoredOverflow, wantErr: core.ErrAttestContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			observation := &bodyExecution{}
			signerObservation := &externalSignerObservation{}
			key := deterministicPrivateKey(t, "writer-terminal")
			got, gotErr := attest.Sign(attest.SignRequest[testDomain]{
				Body:   observedExecutionBody{observation: observation, exit: tc.exit},
				Signer: externalSigner{observation: signerObservation, key: key, mode: externalSignerValid},
			})
			if !errors.Is(gotErr, tc.wantErr) || tc.wantNative != nil && !errors.Is(gotErr, tc.wantNative) {
				t.Fatalf("Sign() error = %v, want %v with native %v", gotErr, tc.wantErr, tc.wantNative)
			}
			if observation.writes != 1 || observation.retained == nil {
				t.Fatalf("body execution = %+v, want one write callback and retained writer", observation)
			}
			if signerObservation.signCalls != tc.wantSignCalls {
				t.Fatalf("signer calls = %d, want %d", signerObservation.signCalls, tc.wantSignCalls)
			}
			written, err := observation.retained.Write([]byte("after return"))
			if written != 0 || !errors.Is(err, core.ErrAttestContract) {
				t.Fatalf("retained writer = (%d, %v), want 0 and %v", written, err, core.ErrAttestContract)
			}
			if tc.wantErr != nil {
				if got != (attest.Envelope[testDomain]{}) {
					t.Fatalf("failed Sign() envelope = %+v, want zero", got)
				}
				return
			}
			proof, err := attest.Verify(attest.VerifyRequest[testDomain]{Body: literalBody{value: []byte("x"), domain: testDomainPrimary}, Envelope: got, TrustedKeys: mustTrustedKeys(t, mustPublicKey(t, key))})
			if err != nil {
				t.Fatalf("Verify() after late write error = %v, want nil", err)
			}
			retained, err := proof.Envelope()
			if err != nil || retained != got {
				t.Fatalf("proof = (%+v, %v), want %+v", retained, err, got)
			}
		})
	}
}

func TestRequestValidationDoesNotExecuteBodyOrSigner(t *testing.T) {
	t.Parallel()
	key := deterministicPrivateKey(t, "shape-only")
	observation := &bodyExecution{}
	signerObservation := &externalSignerObservation{}
	body := observedExecutionBody{observation: observation, exit: bodyExitPartialPanic}
	signRequest := attest.SignRequest[testDomain]{Body: body, Signer: externalSigner{observation: signerObservation, key: key, mode: externalSignerValid}}
	if err := signRequest.Validate(); err != nil {
		t.Fatalf("SignRequest.Validate() error = %v, want nil", err)
	}
	envelope := mustEnvelope(t, literalBody{value: []byte("x"), domain: testDomainPrimary}, key)
	verifyRequest := attest.VerifyRequest[testDomain]{Body: body, Envelope: envelope, TrustedKeys: mustTrustedKeys(t, mustPublicKey(t, key))}
	if err := verifyRequest.Validate(); err != nil {
		t.Fatalf("VerifyRequest.Validate() error = %v, want nil", err)
	}
	if observation.writes != 0 || observation.retained != nil || signerObservation.signCalls != 0 {
		t.Fatalf("shape validation execution = (%+v, %+v), want no body or signer execution", observation, signerObservation)
	}
	// The same body must fail when execution is actually requested.
	got, err := attest.Verify(verifyRequest)
	if !errors.Is(err, core.ErrAttestContract) || got != (attest.Verified[testDomain]{}) || observation.writes != 1 {
		t.Fatalf("Verify() = (%+v, %v, %d writes), want zero proof, typed refusal, one callback", got, err, observation.writes)
	}
}
