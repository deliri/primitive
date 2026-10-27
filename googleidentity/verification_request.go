package googleidentity

import (
	"fmt"
	"io"

	"github.com/deliri/primitive/v2026/core"
)

// GoogleCloudVerificationRequest carries one inbound Authorization value.
// Its grammar is the existing Google workload verifier's grammar; unlike a
// generic outbound header, it adds no token or signed-claim extent quota.
type GoogleCloudVerificationRequest struct {
	authorization *string
}

// NewGoogleCloudVerificationRequest admits the exact inbound value into opaque
// immutable custody. No accepted-token accessor exposes it to diagnostics.
func NewGoogleCloudVerificationRequest(authorization string) (GoogleCloudVerificationRequest, error) {
	request := GoogleCloudVerificationRequest{authorization: &authorization}
	if err := request.Validate(); err != nil {
		return GoogleCloudVerificationRequest{}, err
	}
	return request, nil
}

func (r GoogleCloudVerificationRequest) Validate() error {
	_, err := r.token()
	return err
}

func (r GoogleCloudVerificationRequest) token() (string, error) {
	if r.authorization == nil {
		return "", core.ErrGoogleIdentityContract
	}
	return googleCloudIdentityToken(*r.authorization)
}

// Format prevents ordinary diagnostics from disclosing an inbound credential.
func (GoogleCloudVerificationRequest) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

func (GoogleCloudVerificationRequest) googleIdentityProtocolFact() {}
