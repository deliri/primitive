package awsidentity

import (
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

type acquisitionCall struct {
	context       context.Context
	client        Client
	target        core.HTTPEndpoint
	responseLimit core.ByteCount
	policy        Policy
}

func (c acquisitionCall) Validate() error {
	return errors.Join(
		c.client.Validate(),
		c.target.Validate(),
		c.responseLimit.Validate(),
		c.policy.Validate(),
	)
}

func acquire(call acquisitionCall) (exchange.BoundedResponse, error) {
	if err := call.Validate(); err != nil {
		return exchange.BoundedResponse{}, contractError(err)
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
			ExpectedStatus: core.HTTPStatusOK(),
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

func validateAcquisition(client Client, request Request) error {
	return errors.Join(client.Validate(), request.Validate())
}

var _ core.Validatable = acquisitionCall{}
