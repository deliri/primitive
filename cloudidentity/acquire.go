package cloudidentity

import (
	"context"
	"errors"
	"net/http"
	"sync"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

var successStatus = sync.OnceValues(resolveSuccessStatus)

// resolveSuccessStatus admits the one status either authority may answer one
// acquisition with. Every other status is refused by Exchange.
func resolveSuccessStatus() (core.HTTPStatusCode, error) {
	status, err := core.NewHTTPStatusCode(http.StatusOK)
	if err != nil {
		return core.HTTPStatusCode{}, contractError(err)
	}
	return status, nil
}

// acquisitionCall is one bounded provider acquisition. Each entry point states
// its own target, headers, and response bound, so nothing here selects a
// provider or infers one bound from another.
type acquisitionCall struct {
	context       context.Context
	client        Client
	headers       exchange.Headers
	target        core.HTTPEndpoint
	responseLimit core.ByteCount
	policy        Policy
}

// Validate closes the complete call at the execution boundary, immediately
// before the outbound effect.
func (c acquisitionCall) Validate() error {
	return errors.Join(
		c.client.Validate(),
		c.target.Validate(),
		c.responseLimit.Validate(),
		c.policy.Validate(),
		c.headers.Validate(),
	)
}

func acquire(call acquisitionCall) (exchange.BoundedResponse, error) {
	if err := call.Validate(); err != nil {
		return exchange.BoundedResponse{}, contractError(err)
	}
	status, err := successStatus()
	if err != nil {
		return exchange.BoundedResponse{}, err
	}
	response, err := exchange.SendNoBodyBounded(exchange.NoBodyBoundedCall{
		Context: call.context,
		Client:  call.client.exchange,
		Request: exchange.NoBodyBoundedRequest{
			Target: call.target,
			Semantics: exchange.RequestSemantics{
				Method: exchange.MethodGet,
				Replay: exchange.ReplaySingleAttempt,
			},
			Headers:        call.headers,
			ExpectedStatus: status,
		},
		Policy: exchange.NoBodyBoundedPolicy{
			Operation:         call.policy.exchange(),
			ResponseBodyLimit: call.responseLimit,
		},
	})
	if err != nil {
		return response, contractError(err)
	}
	return response, nil
}

// validateAcquisition is the one ingress gate both entry points cross. It
// reports every defect at once so a caller holding two invalid values does not
// have to fix them one round trip at a time.
func validateAcquisition(client Client, request core.Validatable) error {
	return errors.Join(client.Validate(), request.Validate())
}

var _ core.Validatable = acquisitionCall{}
