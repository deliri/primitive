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
	Authorization string
}

func (r GoogleCloudVerificationRequest) Validate() error {
	_, err := googleCloudIdentityToken(r.Authorization)
	return err
}

// Format prevents ordinary diagnostics from disclosing an inbound credential.
func (GoogleCloudVerificationRequest) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

func (GoogleCloudVerificationRequest) googleIdentityProtocolFact() {}
