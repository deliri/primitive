package timeproof

import (
	"bytes"
	"encoding/asn1"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

type refusalSourceDocument struct {
	Status refusalSourceStatus
	Token  asn1.RawValue `asn1:"optional"`
}

type refusalSourceStatus struct {
	Status  int
	Text    []string       `asn1:"optional"`
	Failure asn1.BitString `asn1:"optional"`
}

func fuzzRefusalSource(t *testing.T, response []byte, refusal Refusal) {
	t.Helper()
	// An independent standard-library struct decode checks UTF8String contents;
	// the production path walks RawValue spans and therefore must prove that
	// semantic check itself. This secondary model is bounded by Verify's current
	// response admission contract, not used in production stream processing.
	var source refusalSourceDocument
	trailing, err := asn1.Unmarshal(response, &source)
	if err != nil || len(trailing) != 0 || len(source.Token.FullBytes) != 0 {
		t.Fatalf("refusal source decode = (%v, %d trailing, %d token bytes), want well-formed refusal without token", err, len(trailing), len(source.Token.FullBytes))
	}
	wantStatus, err := refusal.Status().rfcValue()
	if err != nil || source.Status.Status != wantStatus {
		t.Fatalf("refusal source status = %d, typed status = %d (%v), want equality", source.Status.Status, wantStatus, err)
	}
	codes := refusal.Codes()
	for _, code := range codes {
		bit, err := code.rfcBit()
		if err != nil || source.Status.Failure.At(int(bit)) != 1 {
			t.Fatalf("refusal code %v bit = %d (%v), want a set source bit", code, bit, err)
		}
	}
	setCount := 0
	for bit := 0; bit < source.Status.Failure.BitLength; bit++ {
		setCount += source.Status.Failure.At(bit)
	}
	if setCount != len(codes) {
		t.Fatalf("refusal source set bits = %d, want %d retained typed codes", setCount, len(codes))
	}
}

func addRefusalResponseSeeds(f *testing.F, fixture authenticFixture) {
	f.Helper()
	status, err := RefusalStatusRejection.rfcValue()
	if err != nil {
		f.Fatalf("RefusalStatus.rfcValue() error = %v, want nil", err)
	}
	response := encodeSequence(encodeSequence(encodeStatusInteger(f, status), encodeStatusText(f, "refused")))
	evidence, err := newAuthorityEvidence(authorityEvidenceInput{Response: response, Request: fixture.request})
	if err != nil {
		f.Fatalf("newAuthorityEvidence(seed) error = %v, want nil", err)
	}
	canonical := evidence.ResponseBytes()
	proof, err := Verify(VerifyRequest{Response: canonical, Request: fixture.request, ExpectedDigest: fixture.digest})
	if !errors.Is(err, core.ErrTimeProofRefused) || !timestampHasNoProof(proof) {
		f.Fatalf("Verify(refusal seed) = (%+v, %v), want zero and typed refusal", proof, err)
	}
	f.Add(canonical)
	malformed := bytes.Clone(canonical)
	index := bytes.Index(malformed, []byte("refused"))
	if index < 0 {
		f.Fatalf("refusal text offset = %d, want present typed fixture text", index)
	}
	malformed[index] = 0xff
	f.Add(malformed)
}
