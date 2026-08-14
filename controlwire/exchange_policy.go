package controlwire

import (
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

// This file publishes the bounds a control exchange runs under. It does not
// implement retrying. Exchange owns the retry classification, the exponential
// backoff, the jitter draw, and the server-hint handling, and it owns the rules
// those bounds must satisfy; what is stated below is only the numbers the
// control wire chooses, in the one place all three products read them from.

const (
	// ExchangeOperationTimeoutSeconds bounds one complete control exchange
	// including every retry, and ExchangeAttemptTimeoutSeconds bounds a single
	// attempt.
	//
	// These are conservative ceilings, not tuning. A control exchange carries a
	// few kilobytes and the authority answers from one document read, so a call
	// that has not completed well inside this is failing rather than working.
	// The operation ceiling leaves room for every attempt the retry policy
	// permits plus the waiting between them; a ceiling below its own retry
	// budget would cut the last attempts off and make the attempt count a claim
	// rather than a fact.
	ExchangeOperationTimeoutSeconds = 60
	ExchangeAttemptTimeoutSeconds   = 10

	// The retry bounds. Every control request carries an idempotency key
	// derived from its own nonce, so a retry is provably the same request
	// rather than a second one, which is what makes retrying safe at all.
	// Three attempts crosses a dropped connection or one restarting instance
	// without turning a refusal into a stampede.
	ExchangeRetryMaximumAttempts       = 3
	ExchangeRetryBaseDelaySeconds      = 1
	ExchangeRetryMaximumDelaySeconds   = 5
	ExchangeRetryMaximumJitterSeconds  = 1
	ExchangeRetryMaximumRetryAfterSecs = 10
	ExchangeRetryMaximumWaitSeconds    = 15

	// exchangeRedirectMaximumHops is zero because control exchanges reject
	// redirects outright, and Exchange requires those two facts to agree.
	exchangeRedirectMaximumHops = 0
)

// ControlExchangePolicy returns the complete bounds every control exchange runs
// under.
//
// It lives here rather than in each product because all three products speak
// one control wire. A per-product copy would drift, and the drift would show up
// as one product retrying an exchange another product had already given up on
// against the same authority.
//
// All four parts of the operation policy are stated, including the two that
// look like they could be left alone. Exchange refuses a zero retry policy and
// a zero redirect mode rather than assuming a default, and that is the right
// refusal: an unstated attempt count is not "once", and an unstated redirect
// rule is not "reject".
func ControlExchangePolicy() (exchange.JSONPolicy, error) {
	request, err := core.NewByteCount(core.JSONDocumentMaximumBytes)
	if err != nil {
		return exchange.JSONPolicy{}, contractError(err)
	}
	response, err := core.NewByteCount(core.JSONDocumentMaximumBytes)
	if err != nil {
		return exchange.JSONPolicy{}, contractError(err)
	}
	operation, err := ControlExchangeOperationPolicy()
	if err != nil {
		return exchange.JSONPolicy{}, err
	}
	policy := exchange.JSONPolicy{
		Operation:         operation,
		RequestBodyLimit:  request,
		ResponseBodyLimit: response,
	}
	if err := policy.Validate(); err != nil {
		return exchange.JSONPolicy{}, contractError(err)
	}
	return policy, nil
}

// ControlExchangeOperationPolicy returns the timeout, retry, and redirect
// bounds without the per-call document ceilings, for a control exchange that
// does not carry a JSON body.
func ControlExchangeOperationPolicy() (exchange.OperationPolicy, error) {
	durations, err := controlExchangeDurations()
	if err != nil {
		return exchange.OperationPolicy{}, err
	}
	policy := exchange.OperationPolicy{
		OperationTimeout: durations[0],
		AttemptTimeout:   durations[1],
		Retry: exchange.RetryPolicy{
			BaseDelay:         durations[2],
			MaximumDelay:      durations[3],
			MaximumJitter:     durations[4],
			MaximumRetryAfter: durations[5],
			MaximumWait:       durations[6],
			MaximumAttempts:   ExchangeRetryMaximumAttempts,
		},
		// A control-plane address is compiled into the product. Following a
		// redirect would let whoever answers at that address move an
		// installation onto a host the build never agreed to talk to, which is
		// the one thing a pinned address exists to prevent.
		Redirect: exchange.RedirectPolicy{
			Mode:        exchange.RedirectReject,
			MaximumHops: exchangeRedirectMaximumHops,
		},
	}
	if err := policy.Validate(); err != nil {
		return exchange.OperationPolicy{}, contractError(err)
	}
	return policy, nil
}

// controlExchangeDurations converts every published second count in one pass,
// so a value that cannot become a duration is refused before any part of the
// policy is assembled from the rest.
func controlExchangeDurations() ([7]temporal.Duration, error) {
	seconds := [...]uint64{
		ExchangeOperationTimeoutSeconds, ExchangeAttemptTimeoutSeconds,
		ExchangeRetryBaseDelaySeconds, ExchangeRetryMaximumDelaySeconds,
		ExchangeRetryMaximumJitterSeconds, ExchangeRetryMaximumRetryAfterSecs,
		ExchangeRetryMaximumWaitSeconds,
	}
	var durations [len(seconds)]temporal.Duration
	for index, value := range seconds {
		duration, err := temporal.DurationFromSeconds(value)
		if err != nil {
			return durations, contractError(err)
		}
		durations[index] = duration
	}
	return durations, nil
}

// Semantics projects the replay rule this route runs under, bound to the exact
// request the nonce identifies.
//
// The idempotency key is derived from the nonce rather than drawn separately,
// so a retry of one signed request is provably the same request instead of a
// second one that happens to look alike. Every control route submits a signed
// document, so none of them is safe to retry blindly and all of them carry a
// key.
func (c RouteContract) Semantics(nonce RequestNonce) (exchange.RequestSemantics, error) {
	method, err := c.Method()
	if err != nil {
		return exchange.RequestSemantics{}, err
	}
	key, err := nonce.IdempotencyKey()
	if err != nil {
		return exchange.RequestSemantics{}, err
	}
	semantics := exchange.RequestSemantics{
		IdempotencyKey: key,
		Method:         method,
		Replay:         exchange.ReplayIdempotencyKey,
	}
	if err := semantics.Validate(); err != nil {
		return exchange.RequestSemantics{}, contractError(err)
	}
	return semantics, nil
}
