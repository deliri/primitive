package searchconsole

import (
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/googleidentity"
	"github.com/deliri/primitive/v2026/temporal"
)

// ServiceAccountSource signs for one Search Console reader. The attached
// runtime authorizes the IAM call and the reader receives the read-only bearer.
// This source retains no credential between calls.
type ServiceAccountSource struct {
	identity googleidentity.Client
	signer   googleidentity.ServiceAccountEmail
	policy   googleidentity.Policy
}

func NewServiceAccountSource(network exchange.Client, signer googleidentity.ServiceAccountEmail, policy googleidentity.Policy) (ServiceAccountSource, error) {
	if err := errors.Join(signer.Validate(), policy.Validate()); err != nil {
		return ServiceAccountSource{}, errors.Join(core.ErrSearchConsoleContract, err)
	}
	identity, err := googleidentity.NewClient(network)
	if err != nil {
		return ServiceAccountSource{}, errors.Join(core.ErrSearchConsoleContract, err)
	}
	return ServiceAccountSource{identity: identity, signer: signer, policy: policy}, nil
}

func (s ServiceAccountSource) BorrowReadAccess(ctx context.Context) (core.OAuthClientSecret, error) {
	now, err := temporal.Observe()
	if err != nil {
		return core.OAuthClientSecret{}, errors.Join(core.ErrSearchConsoleContract, err)
	}
	observedAt, err := now.Instant()
	if err != nil {
		return core.OAuthClientSecret{}, errors.Join(core.ErrSearchConsoleContract, err)
	}
	token, err := googleidentity.AcquireSearchConsoleAccess(ctx, s.identity, googleidentity.SearchConsoleAccessRequest{
		Signer: s.signer, ObservedAt: observedAt, Policy: s.policy,
	})
	if err != nil {
		return core.OAuthClientSecret{}, err
	}
	defer func() { _ = token.Close() }()
	return token.CopyBearer()
}

// CloseIdleConnections releases the Google identity client's parked connections.
func (s ServiceAccountSource) CloseIdleConnections() error {
	return s.identity.CloseIdleConnections()
}
