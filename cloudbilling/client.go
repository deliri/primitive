package cloudbilling

import (
	"context"
	"errors"
	"net/http"
	"unicode/utf8"

	"cloud.google.com/go/auth"
	"cloud.google.com/go/auth/credentials"
	"cloud.google.com/go/auth/httptransport"
	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	readOnlyScope      = "https://www.googleapis.com/auth/cloud-billing.readonly"
	quotaProjectHeader = "X-Goog-User-Project"
)

// QuotaProject is a Google project ID or decimal project number. The provider
// owns project existence; this gate admits only the single header value.
type QuotaProject struct{ value string }

func ParseQuotaProject(value string) (QuotaProject, error) {
	if value == "" || !utf8.ValidString(value) {
		return QuotaProject{}, core.ErrCloudBillingContract
	}
	for _, c := range value {
		if c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' {
			continue
		}
		return QuotaProject{}, core.ErrCloudBillingContract
	}
	return QuotaProject{value: value}, nil
}
func (p QuotaProject) Validate() error { _, err := ParseQuotaProject(p.value); return err }
func (p QuotaProject) String() string  { return p.value }

type ADCClientRequest struct {
	QuotaProject      QuotaProject
	CredentialTimeout temporal.Duration
	Credentials       CredentialSource
}

// CredentialSourceKind selects Google's ordinary ADC discovery or exactly the
// caller-bound authorized-user file needed by offline native execution owners.
type CredentialSourceKind uint8

const (
	CredentialSourceUnknown CredentialSourceKind = iota
	CredentialApplicationDefault
	CredentialAuthorizedUserFile
)

// OffWireEnum marks the caller's credential acquisition policy as off wire.
func (CredentialSourceKind) OffWireEnum() {}

type CredentialSource struct {
	Kind CredentialSourceKind
	File core.AbsolutePath
}

func (s CredentialSource) Validate() error {
	switch s.Kind {
	case CredentialApplicationDefault:
		if s.File.String() != "" {
			return core.ErrCloudBillingContract
		}
	case CredentialAuthorizedUserFile:
		if err := s.File.Validate(); err != nil {
			return errors.Join(core.ErrCloudBillingContract, err)
		}
	default:
		return core.ErrCloudBillingContract
	}
	return nil
}

func (r ADCClientRequest) Validate() error {
	if r.CredentialTimeout.IsZero() {
		return core.ErrCloudBillingContract
	}
	if err := errors.Join(r.QuotaProject.Validate(), r.CredentialTimeout.Validate(), r.Credentials.Validate()); err != nil {
		return errors.Join(core.ErrCloudBillingContract, err)
	}
	return nil
}

// ADCClient owns the cloned native transport used by Google's ordinary ADC
// discovery and authenticated requests. It invents no credential search path.
type ADCClient struct {
	client    exchange.Client
	transport *http.Transport
}

func NewADCClient(ctx context.Context, request ADCClientRequest) (*ADCClient, error) {
	base, ok := http.DefaultTransport.(*http.Transport)
	if !ok || base == nil {
		return nil, core.ErrCloudBillingContract
	}
	return newADCClientWithTransport(ctx, request, base)
}

// The supplied native transport is a mechanical dependency, cloned and owned
// here. Ordinary callers use NewADCClient's standard transport. Local provider
// fixtures can exercise this exact setup with their own verified TLS transport.
func newADCClientWithTransport(ctx context.Context, request ADCClientRequest, base *http.Transport) (*ADCClient, error) {
	if err := errors.Join(contextstate.Validate(ctx), request.Validate()); err != nil {
		return nil, errors.Join(core.ErrCloudBillingContract, err)
	}
	if base == nil {
		return nil, core.ErrCloudBillingContract
	}
	transport := base.Clone()
	native, err := request.CredentialTimeout.Stdlib()
	if err != nil {
		transport.CloseIdleConnections()
		return nil, errors.Join(core.ErrCloudBillingContract, err)
	}
	identityHTTP := &http.Client{Transport: transport, Timeout: native}
	detect := &credentials.DetectOptions{Scopes: []string{readOnlyScope}, Client: identityHTTP}
	var creds *auth.Credentials
	switch request.Credentials.Kind {
	case CredentialApplicationDefault:
		creds, err = credentials.DetectDefault(detect)
	case CredentialAuthorizedUserFile:
		creds, err = credentials.NewCredentialsFromFile(credentials.AuthorizedUser, request.Credentials.File.String(), detect)
	default:
		transport.CloseIdleConnections()
		return nil, core.ErrCloudBillingContract
	}
	if err != nil {
		transport.CloseIdleConnections()
		return nil, errors.Join(core.ErrCloudBillingAuthentication, err)
	}
	headers := make(http.Header)
	headers.Set(quotaProjectHeader, request.QuotaProject.value)
	authenticated, err := httptransport.NewClient(&httptransport.Options{Credentials: creds, BaseRoundTripper: transport, Headers: headers, DisableTelemetry: true})
	if err != nil {
		transport.CloseIdleConnections()
		return nil, errors.Join(core.ErrCloudBillingAuthentication, err)
	}
	client, err := exchange.NewClient(authenticated)
	if err != nil {
		transport.CloseIdleConnections()
		return nil, errors.Join(core.ErrCloudBillingContract, err)
	}
	return &ADCClient{client: client, transport: transport}, nil
}

func (c *ADCClient) Server(options ServerOptions) (Server, error) {
	if c == nil || c.transport == nil {
		return Server{}, core.ErrCloudBillingContract
	}
	return NewServer(c.client, options)
}

// Close releases every idle connection in the owned transport and revokes this
// client. Callers join their operations before closing it.
func (c *ADCClient) Close() error {
	if c == nil || c.transport == nil {
		return core.ErrCloudBillingContract
	}
	c.transport.CloseIdleConnections()
	c.transport = nil
	c.client = exchange.Client{}
	return nil
}
