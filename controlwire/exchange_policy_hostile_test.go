package controlwire

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

// TestControlExchangePolicyIsAcceptedByExchange is the ratchet that catches a
// policy no request could ever be sent under.
//
// Exchange refuses a zero retry policy and a zero redirect mode instead of
// assuming a default, so an incomplete policy fails before a single byte leaves
// the process. Nothing about that failure names the missing part, which is why
// it has to be pinned here rather than discovered at a call site.
func TestControlExchangePolicyIsAcceptedByExchange(t *testing.T) {
	t.Parallel()

	policy, err := ControlExchangePolicy()
	if err != nil {
		t.Fatalf("ControlExchangePolicy() error = %v, want nil", err)
	}
	if err := policy.Validate(); err != nil {
		t.Fatalf("control exchange policy Validate() error = %v, want nil", err)
	}
	if err := policy.Operation.Retry.Validate(); err != nil {
		t.Fatalf("retry policy Validate() error = %v, want nil", err)
	}
	if err := policy.Operation.Redirect.Validate(); err != nil {
		t.Fatalf("redirect policy Validate() error = %v, want nil", err)
	}
}

// TestControlExchangePolicyProjectsTheCompilerOwnedDocumentCeiling proves
// neither end can silently configure a different aggregate JSON bound.
func TestControlExchangePolicyProjectsTheCompilerOwnedDocumentCeiling(t *testing.T) {
	t.Parallel()

	policy, err := ControlExchangePolicy()
	if err != nil {
		t.Fatalf("ControlExchangePolicy() error = %v, want nil", err)
	}
	request, err := policy.RequestBodyLimit.Uint64()
	if err != nil {
		t.Fatalf("RequestBodyLimit.Uint64() error = %v, want nil", err)
	}
	response, err := policy.ResponseBodyLimit.Uint64()
	if err != nil {
		t.Fatalf("ResponseBodyLimit.Uint64() error = %v, want nil", err)
	}
	if request != core.JSONDocumentMaximumBytes {
		t.Fatalf("RequestBodyLimit = %d, want %d", request, core.JSONDocumentMaximumBytes)
	}
	if response != core.JSONDocumentMaximumBytes {
		t.Fatalf("ResponseBodyLimit = %d, want %d", response, core.JSONDocumentMaximumBytes)
	}
}

// TestControlExchangeOperationBudgetFitsInsideItsOwnCeiling proves the attempt
// count is a fact rather than a claim.
//
// An operation ceiling below the worst case of the retries it permits would cut
// the last attempts off, and every test of the count alone would still pass.
func TestControlExchangeOperationBudgetFitsInsideItsOwnCeiling(t *testing.T) {
	t.Parallel()

	budget := ExchangeRetryMaximumAttempts*ExchangeAttemptTimeoutSeconds + ExchangeRetryMaximumWaitSeconds
	if budget > ExchangeOperationTimeoutSeconds {
		t.Fatalf("worst-case attempt budget = %ds, want it inside the %ds operation ceiling",
			budget, ExchangeOperationTimeoutSeconds)
	}
	// The retry bounds must also agree with each other, in the same direction
	// Exchange checks them.
	if ExchangeRetryBaseDelaySeconds > ExchangeRetryMaximumDelaySeconds {
		t.Fatalf("base delay %ds exceeds the maximum delay %ds",
			ExchangeRetryBaseDelaySeconds, ExchangeRetryMaximumDelaySeconds)
	}
	if ExchangeRetryMaximumJitterSeconds > ExchangeRetryMaximumDelaySeconds {
		t.Fatalf("maximum jitter %ds exceeds the maximum delay %ds",
			ExchangeRetryMaximumJitterSeconds, ExchangeRetryMaximumDelaySeconds)
	}
	if ExchangeRetryMaximumRetryAfterSecs > ExchangeRetryMaximumWaitSeconds {
		t.Fatalf("maximum retry-after %ds exceeds the maximum wait %ds",
			ExchangeRetryMaximumRetryAfterSecs, ExchangeRetryMaximumWaitSeconds)
	}
}

// TestControlExchangeRejectsRedirects pins the rule a pinned address depends
// on. Following a redirect would let whoever answers move an installation onto
// a host the build never agreed to talk to.
func TestControlExchangeRejectsRedirects(t *testing.T) {
	t.Parallel()

	policy, err := ControlExchangeOperationPolicy()
	if err != nil {
		t.Fatalf("ControlExchangeOperationPolicy() error = %v, want nil", err)
	}
	if policy.Redirect.Mode != exchange.RedirectReject {
		t.Fatalf("redirect mode = %v, want %v", policy.Redirect.Mode, exchange.RedirectReject)
	}
	if policy.Redirect.MaximumHops != 0 {
		t.Fatalf("redirect hops = %d, want 0 alongside a rejecting mode", policy.Redirect.MaximumHops)
	}
	if policy.Retry.MaximumAttempts < 2 {
		t.Fatalf("retry attempts = %d, want a policy that survives one dropped connection",
			policy.Retry.MaximumAttempts)
	}
}

// TestRouteSemanticsBindReplayToTheRequestNonce proves the idempotency key is
// derived from the nonce rather than drawn beside it.
//
// A key that did not come from the nonce would make a retry a second request
// that merely looked alike, and the authority would be entitled to act on it
// twice.
func TestRouteSemanticsBindReplayToTheRequestNonce(t *testing.T) {
	t.Parallel()

	contract, err := NewRouteContract(core.OfferingPeachfuzz, RouteFamilyRegistrations)
	if err != nil {
		t.Fatalf("NewRouteContract() error = %v, want nil", err)
	}
	nonce, err := GenerateRequestNonce()
	if err != nil {
		t.Fatalf("GenerateRequestNonce() error = %v, want nil", err)
	}
	semantics, err := contract.Semantics(nonce)
	if err != nil {
		t.Fatalf("Semantics() error = %v, want nil", err)
	}
	if err := semantics.Validate(); err != nil {
		t.Fatalf("Semantics().Validate() error = %v, want nil", err)
	}
	if semantics.Method != exchange.MethodPost {
		t.Fatalf("Semantics() method = %v, want %v", semantics.Method, exchange.MethodPost)
	}
	if semantics.Replay != exchange.ReplayIdempotencyKey {
		t.Fatalf("Semantics() replay = %v, want %v", semantics.Replay, exchange.ReplayIdempotencyKey)
	}
	derived, err := nonce.IdempotencyKey()
	if err != nil {
		t.Fatalf("IdempotencyKey() error = %v, want nil", err)
	}
	if semantics.IdempotencyKey != derived {
		t.Fatalf("Semantics() key = %v, want the key derived from the nonce %v", semantics.IdempotencyKey, derived)
	}

	// Two requests must never share a replay identity, or the authority would
	// treat a genuinely new request as a retry of the previous one.
	other, err := GenerateRequestNonce()
	if err != nil {
		t.Fatalf("GenerateRequestNonce() error = %v, want nil", err)
	}
	otherSemantics, err := contract.Semantics(other)
	if err != nil {
		t.Fatalf("Semantics() error = %v, want nil", err)
	}
	if otherSemantics.IdempotencyKey == semantics.IdempotencyKey {
		t.Fatalf("two nonces produced one idempotency key, want distinct replay identities")
	}
}

// TestRouteSemanticsRefuseAnUnusableContractOrNonce closes both ingresses. A
// semantics value assembled from an unset route or an unset nonce would send a
// request nothing could identify.
func TestRouteSemanticsRefuseAnUnusableContractOrNonce(t *testing.T) {
	t.Parallel()

	nonce, err := GenerateRequestNonce()
	if err != nil {
		t.Fatalf("GenerateRequestNonce() error = %v, want nil", err)
	}
	if _, err := (RouteContract{}).Semantics(nonce); !errors.Is(err, core.ErrControlWireContract) {
		t.Fatalf("zero RouteContract Semantics() error = %v, want errors.Is %v", err, core.ErrControlWireContract)
	}

	contract, err := NewRouteContract(core.OfferingPeachfuzz, RouteFamilyCheckIns)
	if err != nil {
		t.Fatalf("NewRouteContract() error = %v, want nil", err)
	}
	if _, err := contract.Semantics(RequestNonce{}); !errors.Is(err, core.ErrControlWireContract) {
		t.Fatalf("zero RequestNonce Semantics() error = %v, want errors.Is %v", err, core.ErrControlWireContract)
	}
}
