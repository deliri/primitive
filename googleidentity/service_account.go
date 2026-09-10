package googleidentity

import (
	"bytes"
	"context"
	"errors"

	"cloud.google.com/go/auth/credentials"
	"cloud.google.com/go/auth/credentials/idtoken"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/temporal"
)

// IdentitySource owns acquisition. Product packages receive an opaque token,
// never a provider credential document, private key, or unverified claim map.
type IdentitySource interface {
	Validate() error
	Acquire(context.Context, IdentityTokenRequest) (Token, error)
}

func (c Client) Acquire(ctx context.Context, request IdentityTokenRequest) (Token, error) {
	return AcquireGoogleCloud(ctx, c, request)
}

// ServiceAccountSource owns a dedicated workload credential on a non-GCE host.
// The file must be provisioned by the operator; personal ADC is never inferred.
type ServiceAccountSource struct {
	client exchange.Client
	path   core.AbsolutePath
}

func NewServiceAccountSource(client exchange.Client, path core.AbsolutePath) (ServiceAccountSource, error) {
	source := ServiceAccountSource{client: client, path: path}
	if err := source.Validate(); err != nil {
		return ServiceAccountSource{}, err
	}
	return source, nil
}

func (s ServiceAccountSource) Validate() error {
	if err := errors.Join(s.client.Validate(), s.path.Validate()); err != nil {
		return contractError(err)
	}
	return nil
}

func (s ServiceAccountSource) Acquire(ctx context.Context, request IdentityTokenRequest) (Token, error) {
	if ctx == nil {
		return Token{}, core.ErrGoogleIdentityContract
	}
	if err := errors.Join(s.Validate(), request.Validate(), ctx.Err()); err != nil {
		return Token{}, contractError(err)
	}
	owned, cancel, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: ctx, Duration: request.Policy.OperationTimeout})
	if err != nil {
		return Token{}, contractError(err)
	}
	defer cancel()
	document, err := readServiceAccountCredential(owned, s.path)
	if err != nil {
		return Token{}, err
	}
	defer clear(document)
	return acquireServiceAccountDocument(owned, s.client, request.Audience, document)
}

// The official authentication SDK requires a complete credential JSON value.
// This adapter owns that allocation; Filestore imposes no transfer quota.
func readServiceAccountCredential(ctx context.Context, path core.AbsolutePath) (data []byte, resultErr error) {
	location, err := filestore.OpenParent(ctx, path)
	if err != nil {
		return nil, contractError(err)
	}
	defer func() {
		if err := location.Root.Close(); err != nil {
			clear(data)
			data = nil
			resultErr = contractError(errors.Join(resultErr, err))
		}
	}()
	var buffer bytes.Buffer
	if _, err := filestore.Read(ctx, filestore.ReadRequest{Destination: &buffer, Location: location}); err != nil {
		clear(buffer.Bytes())
		return nil, contractError(err)
	}
	return buffer.Bytes(), nil
}

func acquireServiceAccountDocument(ctx context.Context, client exchange.Client, audience Audience, document []byte) (Token, error) {
	boundary, err := exchange.NewOfficialSDKMethodResponseBoundary(exchange.OfficialSDKMethodResponseBoundaryRequest{Method: exchange.MethodPost, Representation: exchange.OfficialSDKResponseRepresentationJSON})
	if err != nil {
		return Token{}, contractError(err)
	}
	transport, err := client.OfficialSDKResponseTransport(boundary)
	if err != nil {
		return Token{}, contractError(err)
	}
	sdkClient, err := exchange.NewOfficialSDKHTTPClient(transport)
	if err != nil {
		return Token{}, contractError(err)
	}
	credential, err := idtoken.NewCredentialsFromJSON(credentials.ServiceAccount, document, &idtoken.Options{Audience: audience.String(), Client: sdkClient})
	if err != nil {
		return Token{}, contractError(err)
	}
	token, err := credential.Token(ctx)
	if err != nil {
		return Token{}, contractError(err)
	}
	if token == nil {
		return Token{}, core.ErrGoogleIdentityContract
	}
	return newToken(token.Value)
}

func (ServiceAccountSource) googleIdentityCapabilityWrapper() {}

var _ IdentitySource = Client{}
var _ IdentitySource = ServiceAccountSource{}
