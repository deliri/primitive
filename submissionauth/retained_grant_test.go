package submissionauth

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/submission"
)

func TestRetainedGrantCredentialedCompletionLayerTriad(t *testing.T) {
	t.Parallel()
	fixture := newAuthCompletionFixture(t, authCompletionFixtureRequest{})
	encoded, err := fixture.grant.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	var retained submission.GrantRecord
	if err := retained.UnmarshalJSON(encoded); err != nil || retained != fixture.grant {
		t.Fatalf("retained grant = (%v, %v), want exact issued agreement", retained, err)
	}
	base := CompletionVerification{Provider: objectstore.ProviderGoogleCloudStorage, Document: fixture.credentialed, Request: fixture.verifiedRequest, Grant: retained, GrantKeys: fixture.request.trusted, Server: submissionAuthServer(t, fixture.request.trusted), Nonce: fixture.completionNonce}
	for _, tc := range []struct {
		wantErr  error
		name     string
		grant    submission.GrantRecord
		provider objectstore.Provider
	}{
		{name: "retained agreement authenticates nominated device", grant: retained, provider: objectstore.ProviderGoogleCloudStorage},
		{name: "missing grant never creates credentialed proof", provider: objectstore.ProviderGoogleCloudStorage, wantErr: core.ErrControlPlaneContract},
		{name: "foreign provider cannot become credentialed success", grant: retained, provider: objectstore.ProviderAmazonS3, wantErr: core.ErrControlPlaneResponseBinding},
		{name: "absent provider policy is not client inferred", grant: retained, wantErr: core.ErrControlPlaneContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			input := base
			input.Grant, input.Provider = tc.grant, tc.provider
			got, err := VerifyCompletion(input)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("VerifyCompletion = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (VerifiedCompletion{}) {
					t.Fatalf("refused proof = %v, want zero", got)
				}
				return
			}
			payload, err := got.Payload()
			if err != nil || payload != fixture.completionDocument.Payload {
				t.Fatalf("credentialed payload = (%v, %v), want exact signed provider facts", payload, err)
			}
		})
	}
}
