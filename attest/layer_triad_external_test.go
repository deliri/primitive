package attest_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/core"
)

func TestAttestProducerLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive canonical producer emits exact typed facts", func(t *testing.T) {
		t.Parallel()

		body := literalBody{domain: testDomainPrimary, value: []byte("producer")}
		gotEnvelope := mustEnvelope(t, body, deterministicPrivateKey(t, "producer-positive"))
		gotLength, gotLengthErr := gotEnvelope.BodyLength.Uint64()
		if gotLengthErr != nil || gotLength != uint64(len(body.value)) {
			t.Fatalf(
				"Envelope.BodyLength.Uint64() = (%d, %v), want (%d, nil)",
				gotLength,
				gotLengthErr,
				len(body.value),
			)
		}
		if gotEnvelope.Domain != body.domain {
			t.Fatalf("Envelope.Domain = %v, want %v", gotEnvelope.Domain, body.domain)
		}
	})

	t.Run("negative oversized producer emits no envelope", func(t *testing.T) {
		t.Parallel()

		gotEnvelope, gotErr := attest.Sign(attest.SignRequest[testDomain]{
			Body: sizedBody{
				size:      attest.CanonicalBodyMaximumBytes + 1,
				chunkSize: 8192,
				domain:    testDomainPrimary,
			},
			Key: deterministicPrivateKey(t, "producer-negative"),
		})
		if !errors.Is(gotErr, core.ErrAttestContract) {
			t.Fatalf("attest.Sign() error = %v, want %v", gotErr, core.ErrAttestContract)
		}
		if gotEnvelope != (attest.Envelope[testDomain]{}) {
			t.Fatalf("attest.Sign() envelope = %+v, want zero", gotEnvelope)
		}
	})

	t.Run("neutral empty producer emits no envelope", func(t *testing.T) {
		t.Parallel()

		gotEnvelope, gotErr := attest.Sign(attest.SignRequest[testDomain]{
			Body: literalBody{domain: testDomainPrimary},
			Key:  deterministicPrivateKey(t, "producer-neutral"),
		})
		if !errors.Is(gotErr, core.ErrAttestContract) {
			t.Fatalf("attest.Sign() error = %v, want %v", gotErr, core.ErrAttestContract)
		}
		if gotEnvelope != (attest.Envelope[testDomain]{}) {
			t.Fatalf("attest.Sign() envelope = %+v, want zero", gotEnvelope)
		}
	})
}

func TestAttestEnvelopeSchemaLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive complete envelope schema validates", func(t *testing.T) {
		t.Parallel()

		envelope := mustEnvelope(
			t,
			literalBody{domain: testDomainPrimary, value: []byte("schema")},
			deterministicPrivateKey(t, "schema-positive"),
		)
		if gotErr := envelope.Validate(); gotErr != nil {
			t.Fatalf("Envelope.Validate() error = %v, want nil", gotErr)
		}
	})

	t.Run("negative impossible envelope extent fails schema", func(t *testing.T) {
		t.Parallel()

		envelope := mustEnvelope(
			t,
			literalBody{domain: testDomainPrimary, value: []byte("schema")},
			deterministicPrivateKey(t, "schema-negative"),
		)
		length, gotLengthErr := core.NewByteCount(attest.CanonicalBodyMaximumBytes + 1)
		if gotLengthErr != nil {
			t.Fatalf("core.NewByteCount() error = %v, want nil", gotLengthErr)
		}
		envelope.BodyLength = length
		gotErr := envelope.Validate()
		if !errors.Is(gotErr, core.ErrAttestContract) {
			t.Fatalf("Envelope.Validate() error = %v, want %v", gotErr, core.ErrAttestContract)
		}
	})

	t.Run("neutral absent envelope cannot become valid schema", func(t *testing.T) {
		t.Parallel()

		var envelope attest.Envelope[testDomain]
		gotErr := envelope.Validate()
		if !errors.Is(gotErr, core.ErrAttestContract) {
			t.Fatalf("zero Envelope.Validate() error = %v, want %v", gotErr, core.ErrAttestContract)
		}
		if envelope != (attest.Envelope[testDomain]{}) {
			t.Fatalf("Envelope after Validate() = %+v, want zero", envelope)
		}
	})
}

func TestAttestJSONProjectionLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive typed envelope projects and reconstructs exactly", func(t *testing.T) {
		t.Parallel()

		envelope := mustEnvelope(
			t,
			literalBody{domain: testDomainPrimary, value: []byte("projection")},
			deterministicPrivateKey(t, "projection-positive"),
		)
		gotJSON, gotMarshalErr := envelope.MarshalJSON()
		if gotMarshalErr != nil {
			t.Fatalf("Envelope.MarshalJSON() error = %v, want nil", gotMarshalErr)
		}
		var gotEnvelope attest.Envelope[testDomain]
		gotUnmarshalErr := gotEnvelope.UnmarshalJSON(gotJSON)
		if gotUnmarshalErr != nil || gotEnvelope != envelope {
			t.Fatalf(
				"Envelope.UnmarshalJSON() = (%+v, %v), want (%+v, nil)",
				gotEnvelope,
				gotUnmarshalErr,
				envelope,
			)
		}
	})

	t.Run("negative unknown projection member fails and preserves receiver", func(t *testing.T) {
		t.Parallel()

		wantEnvelope := mustEnvelope(
			t,
			literalBody{domain: testDomainPrimary, value: []byte("projection")},
			deterministicPrivateKey(t, "projection-negative"),
		)
		canonical, gotMarshalErr := wantEnvelope.MarshalJSON()
		if gotMarshalErr != nil {
			t.Fatalf("Envelope.MarshalJSON() error = %v, want nil", gotMarshalErr)
		}
		hostile := append(bytes.Clone(canonical[:len(canonical)-1]), []byte(`,"unknown":true}`)...)
		gotEnvelope := wantEnvelope
		gotErr := gotEnvelope.UnmarshalJSON(hostile)
		if !errors.Is(gotErr, core.ErrJSONContract) {
			t.Fatalf("Envelope.UnmarshalJSON() error = %v, want %v", gotErr, core.ErrJSONContract)
		}
		if gotEnvelope != wantEnvelope {
			t.Fatalf("Envelope after rejection = %+v, want preserved %+v", gotEnvelope, wantEnvelope)
		}
	})

	t.Run("neutral absent envelope projects no JSON", func(t *testing.T) {
		t.Parallel()

		var envelope attest.Envelope[testDomain]
		gotJSON, gotErr := envelope.MarshalJSON()
		if !errors.Is(gotErr, core.ErrJSONContract) {
			t.Fatalf("zero Envelope.MarshalJSON() error = %v, want %v", gotErr, core.ErrJSONContract)
		}
		if gotJSON != nil {
			t.Fatalf("zero Envelope.MarshalJSON() bytes = %s, want nil", gotJSON)
		}
	})
}
