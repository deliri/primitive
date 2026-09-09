package distribution_test

import (
	"bytes"
	"encoding/json/jsontext"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/distribution"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/temporal"
)

const distributionFuzzMaximumBytes = distribution.ResponseDocumentJSONMaximumBytes

type distributionExternalDoor[T comparable] struct {
	Seed         T
	Mutation     T
	Marshal      func(T) ([]byte, error)
	Unmarshal    func(*T, []byte) error
	Validate     func(T) error
	Authenticate func(T, bool) error
}

func FuzzSigningDomainExternalDecoders(f *testing.F) {
	domains := []distribution.SigningDomain{
		distribution.SigningDomainPublicationRequestV1, distribution.SigningDomainPublicationGrantV1,
		distribution.SigningDomainPublicationCompletionV1, distribution.SigningDomainUpdateRequestV1,
		distribution.SigningDomainUpdateResponseV1, distribution.SigningDomainUpgradeRequestV1,
		distribution.SigningDomainUpgradeGrantV1}
	var canonical [][]byte
	for _, domain := range domains {
		encoded, err := domain.MarshalJSON()
		if err != nil {
			f.Fatalf("MarshalJSON(seed)=%v, want nil", err)
		}
		canonical = append(canonical, encoded)
		f.Add(encoded)
		f.Add([]byte(domain.String()))
	}
	for _, data := range distributionHostileSeeds() {
		f.Add(data)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		wantText, wantJSON := distribution.SigningDomainUnknown, distribution.SigningDomainUnknown
		for i, domain := range domains {
			if string(data) == domain.String() {
				wantText = domain
			}
			if bytes.Equal(data, canonical[i]) {
				wantJSON = domain
			}
		}
		parsed, parseErr := distribution.ParseSigningDomain(string(data))
		textValue, textErr := distribution.SigningDomainUnknown.ParseCanonicalText(data)
		wantErr := error(nil)
		if wantText == distribution.SigningDomainUnknown {
			wantErr = core.ErrDistributionContract
		}
		if parsed != wantText || textValue != wantText || !errors.Is(parseErr, wantErr) || !errors.Is(textErr, wantErr) {
			t.Fatalf("text decoders=(%v,%v,%v,%v), want (%v,%v)", parsed, textValue, parseErr, textErr, wantText, wantErr)
		}
		for _, before := range []distribution.SigningDomain{distribution.SigningDomainUnknown, distribution.SigningDomainUpdateRequestV1} {
			got := before
			err := got.UnmarshalJSON(data)
			want := wantJSON
			wantErr := error(nil)
			if wantJSON == distribution.SigningDomainUnknown {
				want = before
				wantErr = core.ErrJSONContract
			}
			if got != want || !errors.Is(err, wantErr) {
				t.Fatalf("UnmarshalJSON()=(%v,%v), want (%v,%v)", got, err, want, wantErr)
			}
		}
	})
}

func FuzzPublicationRequestPayloadExternalDecoder(f *testing.F) {
	fixture := newPublicationExchangeFixture(f)
	expectedOffering := distributionOffering(f, 1)
	mutation := fixture.request
	mutation.Nonce = requestNonce(f, 42)
	fuzzDistributionDoor(f, distributionExternalDoor[distribution.PublicationRequestPayload]{
		Seed: fixture.request, Mutation: mutation,
		Marshal:   func(v distribution.PublicationRequestPayload) ([]byte, error) { return v.MarshalJSON() },
		Unmarshal: func(v *distribution.PublicationRequestPayload, data []byte) error { return v.UnmarshalJSON(data) },
		Validate:  func(v distribution.PublicationRequestPayload) error { return v.Validate() },
		Authenticate: func(v distribution.PublicationRequestPayload, authentic bool) error {
			document := fixture.requestDocument
			document.Payload = v
			proof, err := distribution.VerifyPublicationRequest(distribution.PublicationRequestVerification{
				Document: document, RequestKeys: fixture.callerKeys, ManifestKeys: fixture.releaseKeys,
				ExpectedOffering: expectedOffering,
			})
			return distributionProofOracle(proof.Validate(), err, authentic)
		},
	})
}

func FuzzPublicationRequestDocumentExternalDecoder(f *testing.F) {
	fixture := newPublicationExchangeFixture(f)
	expectedOffering := distributionOffering(f, 1)
	mutation := fixture.requestDocument
	mutation.Payload.Nonce = requestNonce(f, 42)
	fuzzDistributionDoor(f, distributionExternalDoor[distribution.PublicationRequestDocument]{
		Seed: fixture.requestDocument, Mutation: mutation,
		Marshal:   func(v distribution.PublicationRequestDocument) ([]byte, error) { return v.MarshalJSON() },
		Unmarshal: func(v *distribution.PublicationRequestDocument, data []byte) error { return v.UnmarshalJSON(data) },
		Validate:  func(v distribution.PublicationRequestDocument) error { return v.Validate() },
		Authenticate: func(v distribution.PublicationRequestDocument, authentic bool) error {
			proof, err := distribution.VerifyPublicationRequest(distribution.PublicationRequestVerification{
				Document: v, RequestKeys: fixture.callerKeys, ManifestKeys: fixture.releaseKeys,
				ExpectedOffering: expectedOffering,
			})
			return distributionProofOracle(proof.Validate(), err, authentic)
		},
	})
}

func FuzzRequestCommitmentExternalDecoder(f *testing.F) {
	fixture := newPublicationExchangeFixture(f)
	seed, err := distribution.CommitRequest(fixture.request)
	if err != nil {
		f.Fatalf("CommitRequest(seed) error = %v, want nil", err)
	}
	other := fixture.request
	other.Nonce = requestNonce(f, 42)
	mutation, err := distribution.CommitRequest(other)
	if err != nil {
		f.Fatalf("CommitRequest(mutation) error = %v, want nil", err)
	}
	fuzzDistributionDoor(f, distributionExternalDoor[distribution.RequestCommitment]{
		Seed: seed, Mutation: mutation,
		Marshal:   func(v distribution.RequestCommitment) ([]byte, error) { return v.MarshalJSON() },
		Unmarshal: func(v *distribution.RequestCommitment, data []byte) error { return v.UnmarshalJSON(data) },
		Validate:  func(v distribution.RequestCommitment) error { return v.Validate() },
	})
}

func FuzzPublicationGrantPayloadExternalDecoder(f *testing.F) {
	fixture := newPublicationExchangeFixture(f)
	mutation := fixture.grantPayload
	mutation.Authorization = authorityNonce(f, 52)
	fuzzDistributionDoor(f, distributionExternalDoor[distribution.PublicationGrantPayload]{
		Seed: fixture.grantPayload, Mutation: mutation,
		Marshal:   func(v distribution.PublicationGrantPayload) ([]byte, error) { return v.MarshalJSON() },
		Unmarshal: func(v *distribution.PublicationGrantPayload, data []byte) error { return v.UnmarshalJSON(data) },
		Validate:  func(v distribution.PublicationGrantPayload) error { return v.Validate() },
		Authenticate: func(v distribution.PublicationGrantPayload, authentic bool) error {
			document := fixture.grantDocument
			document.Payload = v
			proof, err := distribution.VerifyPublicationGrant(distribution.PublicationGrantExpectation{
				Document: document, Request: fixture.request, TrustedKeys: fixture.authorityKeys,
				ObservedAt: temporal.InstantFromNanoseconds(3_000),
			})
			return distributionProofOracle(proof.Validate(), err, authentic)
		},
	})
}

func FuzzPublicationGrantDocumentExternalDecoder(f *testing.F) {
	fixture := newPublicationExchangeFixture(f)
	mutation := fixture.grantProjection
	mutation.Payload.Authorization = authorityNonce(f, 52)
	fuzzPublicationGrantDocument(f, fixture, mutation)
}

func FuzzPublicationCompletionPayloadExternalDecoder(f *testing.F) {
	fixture := newPublicationExchangeFixture(f)
	document := completedPublicationDocument(f, fixture, 0)
	mutation := document.Payload
	mutation.Authorization = authorityNonce(f, 52)
	fuzzDistributionDoor(f, distributionExternalDoor[distribution.PublicationCompletionPayload]{
		Seed: document.Payload, Mutation: mutation,
		Marshal:   func(v distribution.PublicationCompletionPayload) ([]byte, error) { return v.MarshalJSON() },
		Unmarshal: func(v *distribution.PublicationCompletionPayload, data []byte) error { return v.UnmarshalJSON(data) },
		Validate:  func(v distribution.PublicationCompletionPayload) error { return v.Validate() },
		Authenticate: func(v distribution.PublicationCompletionPayload, authentic bool) error {
			candidate := document
			candidate.Payload = v
			proof, err := distribution.VerifyPublicationCompletion(publicationCompletionExpectation(fixture, candidate))
			return distributionProofOracle(proof.Validate(), err, authentic)
		},
	})
}

func FuzzPublicationCompletionDocumentExternalDecoder(f *testing.F) {
	fixture := newPublicationExchangeFixture(f)
	document := completedPublicationDocument(f, fixture, 0)
	mutation := document
	mutation.Payload.Authorization = authorityNonce(f, 52)
	fuzzDistributionDoor(f, distributionExternalDoor[distribution.PublicationCompletionDocument]{
		Seed: document, Mutation: mutation,
		Marshal:   func(v distribution.PublicationCompletionDocument) ([]byte, error) { return v.MarshalJSON() },
		Unmarshal: func(v *distribution.PublicationCompletionDocument, data []byte) error { return v.UnmarshalJSON(data) },
		Validate:  func(v distribution.PublicationCompletionDocument) error { return v.Validate() },
		Authenticate: func(v distribution.PublicationCompletionDocument, authentic bool) error {
			proof, err := distribution.VerifyPublicationCompletion(publicationCompletionExpectation(fixture, v))
			return distributionProofOracle(proof.Validate(), err, authentic)
		},
	})
}

func FuzzUpdateRequestPayloadExternalDecoder(f *testing.F) {
	fixture := newUpdateExchangeFixture(f)
	mutation := fixture.request
	mutation.Nonce = requestNonce(f, 12)
	fuzzDistributionDoor(f, distributionExternalDoor[distribution.UpdateRequestPayload]{
		Seed: fixture.request, Mutation: mutation,
		Marshal:   func(v distribution.UpdateRequestPayload) ([]byte, error) { return v.MarshalJSON() },
		Unmarshal: func(v *distribution.UpdateRequestPayload, data []byte) error { return v.UnmarshalJSON(data) },
		Validate:  func(v distribution.UpdateRequestPayload) error { return v.Validate() },
		Authenticate: func(v distribution.UpdateRequestPayload, authentic bool) error {
			document := fixture.requestDoc
			document.Payload = v
			proof, err := distribution.VerifyUpdateRequest(distribution.UpdateRequestVerification{Document: document, TrustedKeys: fixture.callerKeys})
			return distributionProofOracle(proof.Validate(), err, authentic)
		},
	})
}

func FuzzUpdateRequestDocumentExternalDecoder(f *testing.F) {
	fixture := newUpdateExchangeFixture(f)
	mutation := fixture.requestDoc
	mutation.Payload.Nonce = requestNonce(f, 12)
	fuzzDistributionDoor(f, distributionExternalDoor[distribution.UpdateRequestDocument]{
		Seed: fixture.requestDoc, Mutation: mutation,
		Marshal:   func(v distribution.UpdateRequestDocument) ([]byte, error) { return v.MarshalJSON() },
		Unmarshal: func(v *distribution.UpdateRequestDocument, data []byte) error { return v.UnmarshalJSON(data) },
		Validate:  func(v distribution.UpdateRequestDocument) error { return v.Validate() },
		Authenticate: func(v distribution.UpdateRequestDocument, authentic bool) error {
			proof, err := distribution.VerifyUpdateRequest(distribution.UpdateRequestVerification{Document: v, TrustedKeys: fixture.callerKeys})
			return distributionProofOracle(proof.Validate(), err, authentic)
		},
	})
}

func FuzzUpdateResponsePayloadExternalDecoder(f *testing.F) {
	fixture := newUpdateExchangeFixture(f)
	mutation := fixture.responseDoc.Payload
	mutation.IssuedAt = temporal.InstantFromNanoseconds(2_501)
	fuzzDistributionDoor(f, distributionExternalDoor[distribution.UpdateResponsePayload]{
		Seed: fixture.responseDoc.Payload, Mutation: mutation,
		Marshal:   func(v distribution.UpdateResponsePayload) ([]byte, error) { return v.MarshalJSON() },
		Unmarshal: func(v *distribution.UpdateResponsePayload, data []byte) error { return v.UnmarshalJSON(data) },
		Validate:  func(v distribution.UpdateResponsePayload) error { return v.Validate() },
		Authenticate: func(v distribution.UpdateResponsePayload, authentic bool) error {
			document := fixture.responseDoc
			document.Payload = v
			proof, err := distribution.VerifyUpdateResponse(updateResponseVerification(fixture, document))
			return distributionProofOracle(proof.Validate(), err, authentic)
		},
	})
}

func FuzzUpdateResponseDocumentExternalDecoder(f *testing.F) {
	fixture := newUpdateExchangeFixture(f)
	mutation := fixture.responseDoc
	mutation.Payload.IssuedAt = temporal.InstantFromNanoseconds(2_501)
	fuzzDistributionDoor(f, distributionExternalDoor[distribution.UpdateResponseDocument]{
		Seed: fixture.responseDoc, Mutation: mutation,
		Marshal:   func(v distribution.UpdateResponseDocument) ([]byte, error) { return v.MarshalJSON() },
		Unmarshal: func(v *distribution.UpdateResponseDocument, data []byte) error { return v.UnmarshalJSON(data) },
		Validate:  func(v distribution.UpdateResponseDocument) error { return v.Validate() },
		Authenticate: func(v distribution.UpdateResponseDocument, authentic bool) error {
			proof, err := distribution.VerifyUpdateResponse(updateResponseVerification(fixture, v))
			return distributionProofOracle(proof.Validate(), err, authentic)
		},
	})
}

func FuzzUpgradeRequestPayloadExternalDecoder(f *testing.F) {
	fixture := newUpgradeExchangeFixture(f)
	mutation := fixture.request
	mutation.Nonce = requestNonce(f, 63)
	fuzzDistributionDoor(f, distributionExternalDoor[distribution.UpgradeRequestPayload]{
		Seed: fixture.request, Mutation: mutation,
		Marshal:   func(v distribution.UpgradeRequestPayload) ([]byte, error) { return v.MarshalJSON() },
		Unmarshal: func(v *distribution.UpgradeRequestPayload, data []byte) error { return v.UnmarshalJSON(data) },
		Validate:  func(v distribution.UpgradeRequestPayload) error { return v.Validate() },
		Authenticate: func(v distribution.UpgradeRequestPayload, authentic bool) error {
			document := fixture.requestDoc
			document.Payload = v
			proof, err := distribution.VerifyUpgradeRequest(distribution.UpgradeRequestVerification{Document: document, TrustedKeys: fixture.callerKeys})
			return distributionProofOracle(proof.Validate(), err, authentic)
		},
	})
}

func FuzzUpgradeRequestDocumentExternalDecoder(f *testing.F) {
	fixture := newUpgradeExchangeFixture(f)
	mutation := fixture.requestDoc
	mutation.Payload.Nonce = requestNonce(f, 63)
	fuzzDistributionDoor(f, distributionExternalDoor[distribution.UpgradeRequestDocument]{
		Seed: fixture.requestDoc, Mutation: mutation,
		Marshal:   func(v distribution.UpgradeRequestDocument) ([]byte, error) { return v.MarshalJSON() },
		Unmarshal: func(v *distribution.UpgradeRequestDocument, data []byte) error { return v.UnmarshalJSON(data) },
		Validate:  func(v distribution.UpgradeRequestDocument) error { return v.Validate() },
		Authenticate: func(v distribution.UpgradeRequestDocument, authentic bool) error {
			proof, err := distribution.VerifyUpgradeRequest(distribution.UpgradeRequestVerification{Document: v, TrustedKeys: fixture.callerKeys})
			return distributionProofOracle(proof.Validate(), err, authentic)
		},
	})
}

func FuzzUpgradeGrantPayloadExternalDecoder(f *testing.F) {
	fixture := newUpgradeExchangeFixture(f)
	mutation := fixture.grantDoc.Payload
	mutation.Authorization = authorityNonce(f, 63)
	fuzzDistributionDoor(f, distributionExternalDoor[distribution.UpgradeGrantPayload]{
		Seed: fixture.grantDoc.Payload, Mutation: mutation,
		Marshal:   func(v distribution.UpgradeGrantPayload) ([]byte, error) { return v.MarshalJSON() },
		Unmarshal: func(v *distribution.UpgradeGrantPayload, data []byte) error { return v.UnmarshalJSON(data) },
		Validate:  func(v distribution.UpgradeGrantPayload) error { return v.Validate() },
		Authenticate: func(v distribution.UpgradeGrantPayload, authentic bool) error {
			document := fixture.grantDoc
			document.Payload = v
			proof, err := distribution.VerifyUpgradeGrant(upgradeGrantExpectation(fixture, document))
			return distributionProofOracle(proof.Validate(), err, authentic)
		},
	})
}

func FuzzUpgradeGrantDocumentExternalDecoder(f *testing.F) {
	fixture := newUpgradeExchangeFixture(f)
	fuzzUpgradeGrantDocument(f, fixture)
}

func fuzzDistributionDoor[T comparable](f *testing.F, door distributionExternalDoor[T]) {
	f.Helper()
	if door.Seed == door.Mutation {
		f.Fatalf("fuzz mutation=%v, want changed typed fact", door.Mutation)
	}
	canonical := mustDistributionProjection(f, door, door.Seed)
	f.Add(canonical)
	f.Add(mustDistributionProjection(f, door, door.Mutation))
	for _, data := range distributionHostileSeeds() {
		f.Add(data)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		candidate := door.Seed
		decodeErr := door.Unmarshal(&candidate, data)
		if decodeErr != nil {
			if !errors.Is(decodeErr, core.ErrJSONContract) || !errors.Is(decodeErr, core.ErrDistributionContract) || candidate != door.Seed {
				t.Fatalf("distribution decoder refusal = (%v, preserved %t), want %v/%v and exact receiver",
					decodeErr, candidate == door.Seed, core.ErrJSONContract, core.ErrDistributionContract)
			}
			if bytes.Equal(data, canonical) {
				t.Fatalf("canonical seed refused=%v, want admission", decodeErr)
			}
			return
		}
		if err := door.Validate(candidate); err != nil {
			t.Fatalf("accepted distribution value Validate() error = %v, want nil", err)
		}
		encoded := mustDistributionProjection(t, door, candidate)
		if len(encoded) > distributionFuzzMaximumBytes || !distributionJSONFactsEqual(data, encoded) {
			t.Fatalf("accepted JSON projection=%d bytes, want bounded exact input facts", len(encoded))
		}
		var roundTrip T
		roundTripErr := door.Unmarshal(&roundTrip, encoded)
		second := mustDistributionProjection(t, door, roundTrip)
		if roundTripErr != nil || roundTrip != candidate || !bytes.Equal(second, encoded) {
			t.Fatalf("accepted distribution canonical closure = (%v, %x, %v), want exact fixed point", roundTrip, second, roundTripErr)
		}
		if door.Authenticate != nil {
			if err := door.Authenticate(candidate, bytes.Equal(encoded, canonical)); err != nil {
				t.Fatalf("accepted distribution authentication oracle error = %v, want nil", err)
			}
		}
	})
}

func mustDistributionProjection[T comparable](t testing.TB, door distributionExternalDoor[T], value T) []byte {
	t.Helper()
	encoded, err := door.Marshal(value)
	if err != nil {
		t.Fatalf("distribution MarshalJSON() error = %v, want nil", err)
	}
	return encoded
}

func distributionProofOracle(proofErr, verifyErr error, authentic bool) error {
	if authentic {
		return errors.Join(verifyErr, proofErr)
	}
	if !errors.Is(verifyErr, core.ErrDistributionContract) ||
		(!errors.Is(verifyErr, core.ErrDistributionVerification) && !errors.Is(verifyErr, core.ErrDistributionBinding)) ||
		!errors.Is(proofErr, core.ErrDistributionVerification) {
		return errors.Join(core.ErrDistributionVerification, verifyErr, proofErr)
	}
	return nil
}

func publicationCompletionExpectation(f publicationExchangeFixture, d distribution.PublicationCompletionDocument) distribution.PublicationCompletionExpectation {
	return distribution.PublicationCompletionExpectation{
		Request: f.verifiedRequest, Grant: f.grantPayload, GrantAttestation: f.grantProjection.Attestation,
		Document: d, GrantKeys: f.authorityKeys, CompletionKeys: f.callerKeys,
	}
}

func updateResponseVerification(f updateExchangeFixture, d distribution.UpdateResponseDocument) distribution.UpdateResponseVerification {
	return distribution.UpdateResponseVerification{
		Request: f.request, Document: d, ResponseKeys: f.authorityKeys, LatestKeys: f.releaseKeys,
		ManifestKeys: f.releaseKeys, ExpectedOffering: f.request.Build.Offering(),
		ObservedAt: temporal.InstantFromNanoseconds(3_000),
	}
}

func upgradeGrantExpectation(f upgradeExchangeFixture, d distribution.UpgradeGrantDocument) distribution.UpgradeGrantExpectation {
	return distribution.UpgradeGrantExpectation{
		Request: f.request, Document: d, TrustedKeys: f.authorityKeys,
		ObservedAt: temporal.InstantFromNanoseconds(3_000),
	}
}

func distributionHostileSeeds() [][]byte {
	return [][]byte{
		nil, []byte("null"), []byte("{}"), []byte("[]"), []byte(`{"unknown":true}`),
		[]byte(`{"payload":null}`), bytes.Repeat([]byte{' '}, distributionFuzzMaximumBytes+1),
	}
}

func fuzzPublicationGrantDocument(f *testing.F, fixture publicationExchangeFixture, mutation distribution.PublicationGrantProjection) {
	canonical, err := fixture.grantProjection.MarshalJSON()
	if err != nil {
		f.Fatalf("PublicationGrantProjection.MarshalJSON(seed) error = %v, want nil", err)
	}
	mutated, err := mutation.MarshalJSON()
	if err != nil {
		f.Fatalf("PublicationGrantProjection.MarshalJSON(mutation) error = %v, want nil", err)
	}
	f.Add(canonical)
	f.Add(mutated)
	for _, data := range distributionHostileSeeds() {
		f.Add(data)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		candidate := fixture.grantDocument
		decodeErr := candidate.UnmarshalJSON(data)
		if decodeErr != nil {
			if !errors.Is(decodeErr, core.ErrJSONContract) || !errors.Is(decodeErr, core.ErrDistributionContract) || !samePublicationGrantDocument(candidate, fixture.grantDocument) {
				t.Fatalf("PublicationGrantDocument refusal = (%v, preserved %t), want typed and exact", decodeErr, samePublicationGrantDocument(candidate, fixture.grantDocument))
			}
			return
		}

		projection, projectionErr := publicationGrantProjectionFromDocument(candidate)
		if projectionErr != nil {
			t.Fatalf("accepted grant projection error=%v, want nil", projectionErr)
		}
		encoded, projectionErr := projection.MarshalJSON()
		if projectionErr != nil || !distributionJSONFactsEqual(data, encoded) {
			t.Fatalf("accepted grant projection=(%d,%v), want exact input facts", len(encoded), projectionErr)
		}
		var roundTrip distribution.PublicationGrantDocument
		if err := roundTrip.UnmarshalJSON(encoded); err != nil || !samePublicationGrantDocument(roundTrip, candidate) {
			t.Fatalf("grant round trip=%v, want exact capability set", err)
		}
		proof, verifyErr := distribution.VerifyPublicationGrant(distribution.PublicationGrantExpectation{
			Document: candidate, Request: fixture.request, TrustedKeys: fixture.authorityKeys,
			ObservedAt: temporal.InstantFromNanoseconds(3_000),
		})
		if err := distributionProofOracle(proof.Validate(), verifyErr, samePublicationGrantDocument(candidate, fixture.grantDocument)); err != nil {
			t.Fatalf("PublicationGrantDocument authentication oracle error = %v, want nil", err)
		}
	})
}

func fuzzUpgradeGrantDocument(f *testing.F, fixture upgradeExchangeFixture) {
	projection, err := distribution.IssueUpgradeGrant(distribution.UpgradeGrantIssuance{
		Signer: signingKey(101), Capability: mustUpgradeCapabilityProjection(f), Payload: fixture.grantDoc.Payload,
	})
	if err != nil {
		f.Fatalf("IssueUpgradeGrant(seed projection) error = %v, want nil", err)
	}
	canonical, err := projection.MarshalJSON()
	if err != nil {
		f.Fatalf("UpgradeGrantProjection.MarshalJSON(seed) error = %v, want nil", err)
	}
	mutation := projection
	mutation.Payload.Authorization = authorityNonce(f, 63)
	mutated, err := mutation.MarshalJSON()
	if err != nil {
		f.Fatalf("UpgradeGrantProjection.MarshalJSON(mutation) error = %v, want nil", err)
	}
	f.Add(canonical)
	f.Add(mutated)
	for _, data := range distributionHostileSeeds() {
		f.Add(data)
	}
	f.Fuzz(func(t *testing.T, data []byte) {
		candidate := fixture.grantDoc
		decodeErr := candidate.UnmarshalJSON(data)
		if decodeErr != nil {
			if !errors.Is(decodeErr, core.ErrJSONContract) || !errors.Is(decodeErr, core.ErrDistributionContract) || !sameUpgradeGrantDocument(candidate, fixture.grantDoc) {
				t.Fatalf("UpgradeGrantDocument refusal = (%v, preserved %t), want typed and exact", decodeErr, sameUpgradeGrantDocument(candidate, fixture.grantDoc))
			}
			return
		}

		projection, projectionErr := upgradeGrantProjectionFromDocument(candidate)
		if projectionErr != nil {
			t.Fatalf("accepted grant projection error=%v, want nil", projectionErr)
		}
		encoded, projectionErr := projection.MarshalJSON()
		if projectionErr != nil || !distributionJSONFactsEqual(data, encoded) {
			t.Fatalf("accepted grant projection=(%d,%v), want exact input facts", len(encoded), projectionErr)
		}
		var roundTrip distribution.UpgradeGrantDocument
		if err := roundTrip.UnmarshalJSON(encoded); err != nil || !sameUpgradeGrantDocument(roundTrip, candidate) {
			t.Fatalf("grant round trip=%v, want exact capability", err)
		}
		proof, verifyErr := distribution.VerifyUpgradeGrant(upgradeGrantExpectation(fixture, candidate))
		if err := distributionProofOracle(proof.Validate(), verifyErr, sameUpgradeGrantDocument(candidate, fixture.grantDoc)); err != nil {
			t.Fatalf("UpgradeGrantDocument authentication oracle error = %v, want nil", err)
		}
	})
}

func mustUpgradeCapabilityProjection(t testing.TB) objectstore.DownloadCapabilityProjection {
	t.Helper()
	projection, _ := downloadCapabilityProjection(t, 7)
	return projection
}

func samePublicationGrantDocument(left, right distribution.PublicationGrantDocument) bool {
	if left.Payload != right.Payload || left.Attestation != right.Attestation {
		return false
	}
	for index := range left.Capabilities {
		leftCommitment, leftErr := left.Capabilities[index].Commitment()
		rightCommitment, rightErr := right.Capabilities[index].Commitment()
		if leftErr != nil || rightErr != nil || leftCommitment != rightCommitment {
			return false
		}
	}
	return true
}

func sameUpgradeGrantDocument(left, right distribution.UpgradeGrantDocument) bool {
	leftCommitment, leftErr := left.Capability.Commitment()
	rightCommitment, rightErr := right.Capability.Commitment()
	return leftErr == nil && rightErr == nil && leftCommitment == rightCommitment &&
		left.Payload == right.Payload && left.Attestation == right.Attestation
}

// Compare JSON facts through Go jsontext without converting integers to float64.
// This catches decoders that ignore or alter input while still round-tripping.
func distributionJSONFactsEqual(left, right []byte) bool {
	if len(left) > distributionFuzzMaximumBytes || len(right) > distributionFuzzMaximumBytes {
		return false
	}
	a, b := jsontext.Value(bytes.Clone(left)), jsontext.Value(bytes.Clone(right))
	options := []jsontext.Options{jsontext.ReorderRawObjects(true), jsontext.CanonicalizeRawInts(false), jsontext.CanonicalizeRawFloats(false)}
	if a.Format(options...) != nil || b.Format(options...) != nil {
		return false
	}
	return bytes.Equal(a, b)
}
