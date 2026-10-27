package secretstore

import (
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
)

// GoogleConnectionRequest supplies a caller-configured native connection. Its
// authentication, endpoint and transport security remain connection-owned.
type GoogleConnectionRequest struct {
	Connection *grpc.ClientConn
}

func (r GoogleConnectionRequest) Validate() error {
	if r.Connection == nil {
		return core.ErrSecretStoreContract
	}
	return nil
}

// NewGoogleReaderWithConnection uses one existing native gRPC connection with
// the official SDK. On success, ownership transfers to the reader: Close closes
// the connection. A refused construction leaves the connection with its caller.
// This door supports caller-configured staging/emulator transports without
// changing the SDK's authentication, wire protocol or access-result validation.
func NewGoogleReaderWithConnection(ctx context.Context, request GoogleConnectionRequest) (*GoogleReader, error) {
	if err := contextstate.Validate(ctx); err != nil {
		return nil, errors.Join(core.ErrSecretStoreContract, err)
	}
	if err := request.Validate(); err != nil {
		return nil, err
	}
	return newGoogleReader(ctx, option.WithGRPCConn(request.Connection))
}
