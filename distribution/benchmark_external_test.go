package distribution_test

import (
	"bytes"
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	"testing"

	"github.com/deliri/primitive/v2026/distribution"
)

func BenchmarkParseSigningDomain(b *testing.B) {
	const value = distribution.SigningDomainUpgradeGrantV1Token
	var wantErr error
	b.ReportAllocs()
	var last distribution.SigningDomain
	for b.Loop() {
		got, err := distribution.ParseSigningDomain(value)
		if !errors.Is(err, wantErr) {
			b.Fatalf("distribution.ParseSigningDomain() error = %v, want %v", err, wantErr)
		}
		last = got
	}
	if last.String() != value {
		b.Fatalf("distribution.ParseSigningDomain() = %q, want %q", last, value)
	}
}

// The fixed typed fixtures contain every required release object. No transfer,
// signing-key generation, or fixture construction is in the timed operations.
func BenchmarkCommitPublicationRequest(b *testing.B) {
	f := newPublicationExchangeFixture(b)
	want, err := distribution.CommitRequest(f.request)
	if err != nil {
		b.Fatalf("CommitRequest(setup) error = %v, want nil", err)
	}
	var got distribution.RequestCommitment
	b.ReportAllocs()
	for b.Loop() {
		got, err = distribution.CommitRequest(f.request)
		if err != nil {
			b.Fatalf("CommitRequest() error = %v, want nil", err)
		}
	}
	if got != want {
		b.Fatalf("commitment = %v, want %v", got, want)
	}
}

func BenchmarkVerifyPublicationRequest(b *testing.B) {
	f := newPublicationExchangeFixture(b)
	want, err := distribution.VerifyPublicationRequest(distribution.PublicationRequestVerification{Document: f.requestDocument, RequestKeys: f.callerKeys, ManifestKeys: f.releaseKeys, ExpectedOffering: f.request.Build.Offering()})
	if err != nil {
		b.Fatalf("VerifyPublicationRequest(setup) error = %v, want nil", err)
	}
	var got distribution.VerifiedPublicationRequest
	b.ReportAllocs()
	for b.Loop() {
		got, err = distribution.VerifyPublicationRequest(distribution.PublicationRequestVerification{Document: f.requestDocument, RequestKeys: f.callerKeys, ManifestKeys: f.releaseKeys, ExpectedOffering: f.request.Build.Offering()})
		if err != nil {
			b.Fatalf("VerifyPublicationRequest() error = %v, want nil", err)
		}
	}
	if got != want {
		b.Fatalf("VerifyPublicationRequest() = %v, want %v", got, want)
	}
}

func BenchmarkVerifyPublicationGrant(b *testing.B) {
	f := newPublicationExchangeFixture(b)
	want, err := distribution.VerifyPublicationGrant(distribution.PublicationGrantExpectation{Document: f.grantDocument, Request: f.request, TrustedKeys: f.authorityKeys, ObservedAt: temporal.InstantFromNanoseconds(3_000)})
	if err != nil {
		b.Fatalf("VerifyPublicationGrant(setup) error = %v, want nil", err)
	}
	var got distribution.VerifiedPublicationGrant
	b.ReportAllocs()
	for b.Loop() {
		got, err = distribution.VerifyPublicationGrant(distribution.PublicationGrantExpectation{Document: f.grantDocument, Request: f.request, TrustedKeys: f.authorityKeys, ObservedAt: temporal.InstantFromNanoseconds(3_000)})
		if err != nil {
			b.Fatalf("VerifyPublicationGrant() error = %v, want nil", err)
		}
	}
	gotRequest, gotErr := got.Request()
	wantRequest, wantErr := want.Request()
	if gotErr != nil || wantErr != nil || got.Validate() != nil || gotRequest != wantRequest {
		b.Fatalf("VerifyPublicationGrant() = %v, want %v", got, want)
	}
}

func BenchmarkVerifyUpdateRequest(b *testing.B) {
	f := newUpdateExchangeFixture(b)
	want, err := distribution.VerifyUpdateRequest(distribution.UpdateRequestVerification{Document: f.requestDoc, TrustedKeys: f.callerKeys})
	if err != nil {
		b.Fatalf("VerifyUpdateRequest(setup) error = %v, want nil", err)
	}
	var got distribution.VerifiedUpdateRequest
	b.ReportAllocs()
	for b.Loop() {
		got, err = distribution.VerifyUpdateRequest(distribution.UpdateRequestVerification{Document: f.requestDoc, TrustedKeys: f.callerKeys})
		if err != nil {
			b.Fatalf("VerifyUpdateRequest() error = %v, want nil", err)
		}
	}
	if got != want {
		b.Fatalf("VerifyUpdateRequest() = %v, want %v", got, want)
	}
}

func BenchmarkVerifyUpdateResponse(b *testing.B) {
	f := newUpdateExchangeFixture(b)
	want, err := distribution.VerifyUpdateResponse(updateResponseVerification(f, f.responseDoc))
	if err != nil {
		b.Fatalf("VerifyUpdateResponse(setup) error = %v, want nil", err)
	}
	var got distribution.VerifiedUpdateResponse
	b.ReportAllocs()
	for b.Loop() {
		got, err = distribution.VerifyUpdateResponse(updateResponseVerification(f, f.responseDoc))
		if err != nil {
			b.Fatalf("VerifyUpdateResponse() error = %v, want nil", err)
		}
	}
	if got != want {
		b.Fatalf("VerifyUpdateResponse() = %v, want %v", got, want)
	}
}

func BenchmarkVerifyUpgradeRequest(b *testing.B) {
	f := newUpgradeExchangeFixture(b)
	want, err := distribution.VerifyUpgradeRequest(distribution.UpgradeRequestVerification{Document: f.requestDoc, TrustedKeys: f.callerKeys})
	if err != nil {
		b.Fatalf("VerifyUpgradeRequest(setup) error = %v, want nil", err)
	}
	var got distribution.VerifiedUpgradeRequest
	b.ReportAllocs()
	for b.Loop() {
		got, err = distribution.VerifyUpgradeRequest(distribution.UpgradeRequestVerification{Document: f.requestDoc, TrustedKeys: f.callerKeys})
		if err != nil {
			b.Fatalf("VerifyUpgradeRequest() error = %v, want nil", err)
		}
	}
	if got != want {
		b.Fatalf("VerifyUpgradeRequest() = %v, want %v", got, want)
	}
}

func BenchmarkVerifyUpgradeGrant(b *testing.B) {
	f := newUpgradeExchangeFixture(b)
	want, err := distribution.VerifyUpgradeGrant(upgradeGrantExpectation(f, f.grantDoc))
	if err != nil {
		b.Fatalf("VerifyUpgradeGrant(setup) error = %v, want nil", err)
	}
	var got distribution.VerifiedUpgradeGrant
	b.ReportAllocs()
	for b.Loop() {
		got, err = distribution.VerifyUpgradeGrant(upgradeGrantExpectation(f, f.grantDoc))
		if err != nil {
			b.Fatalf("VerifyUpgradeGrant() error = %v, want nil", err)
		}
	}
	gotRequest, gotErr := got.Request()
	wantRequest, wantErr := want.Request()
	if gotErr != nil || wantErr != nil || got.Validate() != nil || gotRequest != wantRequest {
		b.Fatalf("VerifyUpgradeGrant() = %v, want %v", got, want)
	}
}

func BenchmarkVerifyPublicationCompletion(b *testing.B) {
	f := newPublicationExchangeFixture(b)
	doc := completedPublicationDocument(b, f, 0)
	request := publicationCompletionExpectation(f, doc)
	want, err := distribution.VerifyPublicationCompletion(request)
	if err != nil {
		b.Fatalf("VerifyPublicationCompletion(setup) error = %v, want nil", err)
	}
	var got distribution.VerifiedPublicationCompletion
	b.ReportAllocs()
	for b.Loop() {
		got, err = distribution.VerifyPublicationCompletion(request)
		if err != nil {
			b.Fatalf("VerifyPublicationCompletion() error = %v, want nil", err)
		}
	}
	if got != want {
		b.Fatalf("VerifyPublicationCompletion() = %v, want %v", got, want)
	}
}

func BenchmarkUpdateResponseJSON(b *testing.B) {
	b.ReportAllocs()
	f := newUpdateExchangeFixture(b)
	wire, err := f.responseDoc.MarshalJSON()
	if err != nil || len(wire) == 0 || len(wire) > distribution.ResponseDocumentJSONMaximumBytes {
		b.Fatalf("MarshalJSON(setup) = (%d,%v), want bounded document", len(wire), err)
	}
	cases := []struct {
		name   string
		encode bool
	}{{name: "encode", encode: true}, {name: "decode"}}
	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			var encoded []byte
			var decoded distribution.UpdateResponseDocument
			b.ReportAllocs()
			b.SetBytes(int64(len(wire)))
			for b.Loop() {
				if tc.encode {
					encoded, err = f.responseDoc.MarshalJSON()
				} else {
					err = decoded.UnmarshalJSON(wire)
				}
				if err != nil {
					b.Fatalf("JSON() error = %v, want nil", err)
				}
			}
			if tc.encode {
				if !bytes.Equal(encoded, wire) {
					b.Fatalf("JSON() = %d bytes, want exact %d bytes", len(encoded), len(wire))
				}
			} else if decoded != f.responseDoc {
				b.Fatalf("JSON() = %v, want %v", decoded, f.responseDoc)
			}
		})
	}
}

func BenchmarkSigningDomainJSONRefusal(b *testing.B) {
	wire := bytes.Repeat([]byte{'x'}, distribution.ResponseDocumentJSONMaximumBytes)
	wire[0], wire[len(wire)-1] = '"', '"'
	b.ReportAllocs()
	for b.Loop() {
		got := distribution.SigningDomainUpgradeGrantV1
		err := got.UnmarshalJSON(wire)
		if !errors.Is(err, core.ErrDistributionContract) || got != distribution.SigningDomainUpgradeGrantV1 {
			b.Fatalf("UnmarshalJSON() = (%v,%v), want preserved typed refusal", got, err)
		}
	}
}
