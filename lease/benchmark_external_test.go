package lease_test

import (
	"bytes"

	"testing"

	"github.com/deliri/primitive/v2026/lease"
)

func BenchmarkEvaluate(b *testing.B) {
	authority := fixtureAuthority(b, 131)
	subject := fixtureSubject(b, 132)
	decision := fixtureGrantDecision(b, subject, 1, 1_000, fixtureGrant())
	_, verified := fixtureVerified(b, authority, decision, subject)
	observation := fixtureObservation(b, 2_500)
	request := lease.EvaluateRequest{
		Decision: verified, DurableHighWater: fixtureInstant(1_000),
		StartedAt: observation, ObservedAt: observation,
	}
	b.ReportAllocs()

	var result lease.Assessment
	var err error
	for b.Loop() {
		result, err = lease.Evaluate(request)
	}
	if err != nil || result.Validate() != nil {
		b.Fatalf("Evaluate result/error = %v/%v, want valid assessment", result, err)
	}
	if result.State() != lease.StateCurrent {
		b.Fatalf("assessment state = %v, want current", result.State())
	}
}

func BenchmarkVerify(b *testing.B) {
	authority := fixtureAuthority(b, 141)
	subject := fixtureSubject(b, 142)
	decision := fixtureGrantDecision(b, subject, 1, 1_000, fixtureGrant())
	document, _ := fixtureVerified(b, authority, decision, subject)
	request := lease.VerifyRequest{
		Document: document, TrustedKeys: authority.trusted,
		ExpectedSubject: subject,
	}
	b.ReportAllocs()

	var result lease.Verified
	var err error
	for b.Loop() {
		result, err = lease.Verify(request)
	}
	if err != nil || result.Validate() != nil {
		b.Fatalf("Verify result/error = %v/%v, want authentic decision", result, err)
	}
	got, projectionErr := result.Decision()
	if projectionErr != nil || got != decision {
		b.Fatalf("verified decision/error = %v/%v, want signed fixture", got, projectionErr)
	}
}

func BenchmarkDecisionCanonicalJSON(b *testing.B) {
	subject := fixtureSubject(b, 151)
	decision := fixtureGrantDecision(b, subject, 1, 1_000, fixtureGrant())
	b.ReportAllocs()

	var result []byte
	var err error
	for b.Loop() {
		result, err = decision.MarshalJSON()
	}
	if err != nil || len(result) == 0 {
		b.Fatalf("MarshalJSON bytes/error = %d/%v, want nonempty canonical decision", len(result), err)
	}
	var got lease.Decision
	if err := got.UnmarshalJSON(result); err != nil || got != decision {
		b.Fatalf("canonical decision/error = %v/%v, want fixture", got, err)
	}
}

func BenchmarkDocumentJSONDecode(b *testing.B) {
	b.ReportAllocs()
	authority := fixtureAuthority(b, 161)
	subject := fixtureSubject(b, 162)
	decision := fixtureGrantDecision(b, subject, 1, 1_000, fixtureGrant())
	document, _ := fixtureVerified(b, authority, decision, subject)
	canonical, err := document.MarshalJSON()
	if err != nil {
		b.Fatal(err)
	}
	for _, tc := range []struct {
		name    string
		padding int
	}{
		{name: "canonical"}, {name: "whitespace_256", padding: 256},
	} {
		b.Run(tc.name, func(b *testing.B) {
			data := append(bytes.Repeat([]byte{' '}, tc.padding), canonical...)
			var got lease.Document
			if err := got.UnmarshalJSON(data); err != nil || got != document {
				b.Fatalf("setup document/error = %v/%v, want fixture", got, err)
			}
			b.ReportAllocs()
			b.SetBytes(int64(len(data)))
			for b.Loop() {
				err = got.UnmarshalJSON(data)
			}
			if err != nil || got != document {
				b.Fatalf("decoded document/error = %v/%v, want fixture", got, err)
			}
		})
	}
}

func BenchmarkDecisionCanonicalWriter(b *testing.B) {
	subject := fixtureSubject(b, 171)
	decision := fixtureGrantDecision(b, subject, 1, 1_000, fixtureGrant())
	canonical, err := decision.MarshalJSON()
	if err != nil {
		b.Fatal(err)
	}
	sink := canonicalCountWriter{}
	b.ReportAllocs()
	b.SetBytes(int64(len(canonical)))
	for b.Loop() {
		err = decision.WriteCanonical(&sink)
	}
	if err != nil || sink.bytes != int64(b.N)*int64(len(canonical)) {
		b.Fatalf("written bytes/error = %d/%v, want %d", sink.bytes, err, int64(b.N)*int64(len(canonical)))
	}
}

type canonicalCountWriter struct{ bytes int64 }

func (w *canonicalCountWriter) Write(p []byte) (int, error) {
	w.bytes += int64(len(p))
	return len(p), nil
}
