package submission

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/attest"
	"github.com/deliri/primitive/v2026/controlwire"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/receipt"
	"github.com/deliri/primitive/v2026/temporal"
)

type uploadCallFixture struct {
	decision VerifiedDecision
	content  []byte
	request  RequestPayload
	trusted  attest.TrustedKeys
	account  receipt.AccountIdentity
	offering receipt.OfferingIdentity
}

func TestVerifiedDecisionUploadCallLayerTriad(t *testing.T) {
	t.Parallel()

	t.Run("positive exact verified grant projects one blind objectstore call", func(t *testing.T) {
		t.Parallel()

		fixture := newUploadCallFixture(t, []byte{1, 2, 3})
		call, err := fixture.decision.UploadCall(UploadCallRequest{
			Source: bytes.NewReader(fixture.content), Request: fixture.request,
			Policy: providerPolicy(t),
		})
		if err != nil {
			t.Fatalf("VerifiedDecision.UploadCall() error = %v, want nil", err)
		}
		if err := call.Validate(); err != nil {
			t.Fatalf("UploadCapabilityRequest.Validate() error = %v, want nil", err)
		}
		provider, err := call.Capability.Provider()
		if err != nil || provider != objectstore.ProviderGoogleCloudStorage {
			t.Fatalf("UploadCapability.Provider() = (%v, %v), want (%v, nil)",
				provider, err, objectstore.ProviderGoogleCloudStorage)
		}
		if call.ContentType != fixture.request.Declaration.ContentType ||
			call.Integrity != fixture.request.Declaration.Integrity() {
			t.Fatalf("projected call declaration = (%v, %v), want exact request declaration",
				call.ContentType, call.Integrity)
		}
	})

	t.Run("negative different signed request cannot reuse a verified grant", func(t *testing.T) {
		t.Parallel()

		fixture := newUploadCallFixture(t, []byte{4, 5, 6})
		other := newGrantFixture(t, grantFixtureRequest{content: []byte{7, 8, 9}})
		call, gotErr := fixture.decision.UploadCall(UploadCallRequest{
			Source: bytes.NewReader(fixture.content), Request: other.request,
			Policy: providerPolicy(t),
		})
		if !errors.Is(gotErr, core.ErrControlPlaneResponseBinding) || !uploadCallIsZero(call) {
			t.Fatalf("VerifiedDecision.UploadCall(different request) = (%v, %v), want zero and errors.Is %v",
				call, gotErr, core.ErrControlPlaneResponseBinding)
		}
	})

	t.Run("neutral reuse decision refuses an upload call without reading source", func(t *testing.T) {
		t.Parallel()

		fixture := newUploadCallFixture(t, []byte{10, 11, 12})
		reuse := newReuseEvidenceFixture(t, reuseEvidenceFixtureRequest{
			Request: fixture.request, KeyByte: 0x41, ScopeByte: 0x72,
		})
		fixture.account = reuse.account
		fixture.offering = reuse.offering
		decision := verifyDecisionProjection(t, decisionProjectionFixture{
			Fixture: fixture, Projection: mustReuseDecision(t, reuse),
		})
		source := bytes.NewReader(fixture.content)
		call, gotErr := decision.UploadCall(UploadCallRequest{
			Source: source, Request: fixture.request, Policy: providerPolicy(t),
		})
		if !errors.Is(gotErr, core.ErrControlPlaneResponseBinding) || !uploadCallIsZero(call) {
			t.Fatalf("VerifiedDecision.UploadCall(reuse) = (%v, %v), want zero and errors.Is %v",
				call, gotErr, core.ErrControlPlaneResponseBinding)
		}
		if gotRemaining := source.Len(); gotRemaining != len(fixture.content) {
			t.Fatalf("reuse source bytes remaining = %d, want untouched %d", gotRemaining, len(fixture.content))
		}
	})
}

type uploadCallMutation uint8

const (
	uploadCallMutationZeroRequest uploadCallMutation = iota
	uploadCallMutationNilSource
	uploadCallMutationZeroPayload
	uploadCallMutationZeroDeclaration
	uploadCallMutationZeroBuild
	uploadCallMutationZeroRevision
	uploadCallMutationFutureRevision
	uploadCallMutationZeroNonce
	uploadCallMutationZeroPolicy
	uploadCallMutationDifferentDeclaration
	uploadCallMutationDifferentBuild
	uploadCallMutationDifferentNonce
	uploadCallMutationZeroDecision
)

type uploadCallCase struct {
	wantErr  error
	name     string
	mutation uploadCallMutation
}

func TestVerifiedDecisionUploadCallHostileBoundaryMatrix(t *testing.T) {
	t.Parallel()

	cases := []uploadCallCase{
		{name: "zero call request", mutation: uploadCallMutationZeroRequest, wantErr: core.ErrControlPlaneContract},
		{name: "nil source", mutation: uploadCallMutationNilSource, wantErr: core.ErrControlPlaneContract},
		{name: "zero request payload", mutation: uploadCallMutationZeroPayload, wantErr: core.ErrControlPlaneContract},
		{name: "zero declaration", mutation: uploadCallMutationZeroDeclaration, wantErr: core.ErrControlPlaneContract},
		{name: "zero build", mutation: uploadCallMutationZeroBuild, wantErr: core.ErrControlPlaneContract},
		{name: "zero revision", mutation: uploadCallMutationZeroRevision, wantErr: core.ErrControlPlaneContract},
		{name: "future revision", mutation: uploadCallMutationFutureRevision, wantErr: core.ErrControlPlaneContract},
		{name: "zero nonce", mutation: uploadCallMutationZeroNonce, wantErr: core.ErrControlPlaneContract},
		{name: "zero transfer policy", mutation: uploadCallMutationZeroPolicy, wantErr: core.ErrControlPlaneContract},
		{name: "different declaration", mutation: uploadCallMutationDifferentDeclaration, wantErr: core.ErrControlPlaneResponseBinding},
		{name: "different build", mutation: uploadCallMutationDifferentBuild, wantErr: core.ErrControlPlaneResponseBinding},
		{name: "different nonce", mutation: uploadCallMutationDifferentNonce, wantErr: core.ErrControlPlaneResponseBinding},
		{name: "zero verified decision", mutation: uploadCallMutationZeroDecision, wantErr: core.ErrControlPlaneContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fixture := newUploadCallFixture(t, []byte{21, 22, 23})
			source := bytes.NewReader(fixture.content)
			input := UploadCallRequest{
				Source: source, Request: fixture.request,
				Policy: providerPolicy(t),
			}
			decision := fixture.decision
			switch tc.mutation {
			case uploadCallMutationZeroRequest:
				input = UploadCallRequest{}
			case uploadCallMutationNilSource:
				input.Source = nil
			case uploadCallMutationZeroPayload:
				input.Request = RequestPayload{}
			case uploadCallMutationZeroDeclaration:
				input.Request.Declaration = Declaration{}
			case uploadCallMutationZeroBuild:
				input.Request.Build = core.BuildIdentity{}
			case uploadCallMutationZeroRevision:
				input.Request.Revision = 0
			case uploadCallMutationFutureRevision:
				input.Request.Revision = controlwire.Revision(255)
			case uploadCallMutationZeroNonce:
				input.Request.Nonce = controlwire.RequestNonce{}
			case uploadCallMutationZeroPolicy:
				input.Policy = objectstore.Policy{}
			case uploadCallMutationDifferentDeclaration:
				input.Request.Declaration = testDeclaration(t, []byte{24})
			case uploadCallMutationDifferentBuild:
				input.Request.Build = newGrantFixture(t, grantFixtureRequest{offering: core.OfferingBug}).request.Build
			case uploadCallMutationDifferentNonce:
				input.Request.Nonce = newGrantFixture(t, grantFixtureRequest{requestNonceByte: 0x73}).request.Nonce
			case uploadCallMutationZeroDecision:
				decision = VerifiedDecision{}
			default:
				t.Fatalf("upload call mutation = %d, want a published test mutation", tc.mutation)
			}
			call, gotErr := decision.UploadCall(input)
			if !errors.Is(gotErr, tc.wantErr) || !uploadCallIsZero(call) {
				t.Fatalf("VerifiedDecision.UploadCall(hostile) = (%v, %v), want zero and errors.Is %v",
					call, gotErr, tc.wantErr)
			}
			if gotRemaining := source.Len(); gotRemaining != len(fixture.content) {
				t.Fatalf("VerifiedDecision.UploadCall(hostile) source bytes remaining = %d, want untouched %d",
					gotRemaining, len(fixture.content))
			}
		})
	}
}

func newUploadCallFixture(t *testing.T, content []byte) uploadCallFixture {
	t.Helper()

	grant := newGrantFixture(t, grantFixtureRequest{content: content})
	reuse := newReuseEvidenceFixture(t, reuseEvidenceFixtureRequest{
		Request: grant.request, KeyByte: 0x41, ScopeByte: 0x62,
	})
	fixture := uploadCallFixture{
		request: grant.request, content: content, trusted: grant.trusted,
		account: reuse.account, offering: reuse.offering,
	}
	fixture.decision = verifyDecisionProjection(t, decisionProjectionFixture{
		Fixture: fixture, Projection: mustUploadDecision(t, grant.projection),
	})
	return fixture
}

type decisionProjectionFixture struct {
	Projection DecisionProjection
	Fixture    uploadCallFixture
}

func verifyDecisionProjection(t *testing.T, input decisionProjectionFixture) VerifiedDecision {
	t.Helper()

	decision, err := VerifyDecision(DecisionExpectation{
		Decision: decodeDecisionProjection(t, input.Projection), Request: input.Fixture.request,
		Account: input.Fixture.account, Offering: input.Fixture.offering,
		ObservedAt:  temporal.InstantFromNanoseconds(testGrantIssuedAt),
		TrustedKeys: input.Fixture.trusted,
	})
	if err != nil {
		t.Fatalf("VerifyDecision() error = %v, want nil", err)
	}
	return decision
}

func uploadCallIsZero(call objectstore.UploadCapabilityRequest) bool {
	return call.Source == nil && call.Observer == nil && call.Capability.IsZero() &&
		call.ContentType.IsZero() && call.Integrity == (objectstore.Integrity{}) &&
		call.Policy == (objectstore.Policy{})
}

func providerPolicy(t *testing.T) objectstore.Policy {
	t.Helper()

	limit, err := core.NewByteCount(4096)
	if err != nil {
		t.Fatalf("core.NewByteCount() error = %v, want nil", err)
	}
	operation, err := temporal.DurationFromSeconds(10)
	if err != nil {
		t.Fatalf("temporal.DurationFromSeconds(operation) error = %v, want nil", err)
	}
	attempt, err := temporal.DurationFromSeconds(5)
	if err != nil {
		t.Fatalf("temporal.DurationFromSeconds(attempt) error = %v, want nil", err)
	}
	return objectstore.Policy{
		OperationTimeout: operation, AttemptTimeout: attempt, ErrorBodyLimit: limit,
	}
}
