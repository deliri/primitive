package cloudidentity

import (
	"fmt"
	"io"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	// AudienceMaximumBytes is the common audience bound. AWS publishes a
	// maximum of 1,000 characters for each GetWebIdentityToken audience;
	// Cloudidentity uses the stricter byte bound for deterministic HTTP extent.
	AudienceMaximumBytes = 1000
	// TokenMaximumBytes bounds one acquired bearer independently of provider.
	// Each provider states its own response bound from this one, because a
	// bare token and an enveloped token are different extents on the wire.
	TokenMaximumBytes = 16 * 1024
	// DefaultTimeoutSeconds is the complete default acquisition budget.
	DefaultTimeoutSeconds = 5
)

// Provider identifies the cloud authority that issued a token.
type Provider uint8

const (
	// ProviderUnknown is the invalid zero provider.
	ProviderUnknown Provider = iota
	// ProviderGoogleCloud identifies Google Cloud metadata identity.
	ProviderGoogleCloud
	// ProviderAmazonWebServices identifies AWS STS outbound identity.
	ProviderAmazonWebServices
	providerLimit
)

// Validate closes the provider domain.
func (p Provider) Validate() error {
	if p <= ProviderUnknown || p >= providerLimit || providerLabels()[p] == "" {
		return core.ErrCloudIdentityContract
	}
	return nil
}

// IsValid reports whether p belongs to the provider domain.
func (p Provider) IsValid() bool { return p.Validate() == nil }

// String returns a diagnostic provider name.
func (p Provider) String() string {
	if p >= providerLimit {
		return ""
	}
	return providerLabels()[p]
}

func providerLabels() [providerLimit]string {
	return [...]string{"", "google_cloud", "amazon_web_services"}
}

// OffWireEnum declares Provider as an execution enum.
func (Provider) OffWireEnum() {}

// Audience is one exact receiver identifier shared by both provider calls.
type Audience struct {
	value string
}

// ParseAudience owns one nonempty bounded UTF-8 audience.
func ParseAudience(value string) (Audience, error) {
	if len(value) == 0 || len(value) > AudienceMaximumBytes ||
		!utf8.ValidString(value) {
		return Audience{}, core.ErrCloudIdentityContract
	}
	return Audience{value: value}, nil
}

// Validate rejects unset, oversized, or invalid UTF-8 audiences.
func (a Audience) Validate() error {
	_, err := ParseAudience(a.value)
	return err
}

// String returns the exact audience.
func (a Audience) String() string {
	if err := a.Validate(); err != nil {
		return ""
	}
	return a.value
}

// Policy owns the complete and per-attempt bounds for one provider call.
// Acquisition is always single-attempt and always rejects redirects.
type Policy struct {
	OperationTimeout temporal.Duration
	AttemptTimeout   temporal.Duration
}

// DefaultPolicy returns one five-second, single-attempt policy.
func DefaultPolicy() (Policy, error) {
	timeout, err := temporal.DurationFromSeconds(DefaultTimeoutSeconds)
	if err != nil {
		return Policy{}, contractError(err)
	}
	return Policy{
		OperationTimeout: timeout,
		AttemptTimeout:   timeout,
	}, nil
}

// Validate closes the timeout pair and fixed single-attempt behavior.
func (p Policy) Validate() error {
	if err := p.exchange().Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

func (p Policy) exchange() exchange.OperationPolicy {
	return exchange.OperationPolicy{
		OperationTimeout: p.OperationTimeout,
		AttemptTimeout:   p.AttemptTimeout,
		Retry: exchange.RetryPolicy{
			MaximumAttempts: 1,
		},
		Redirect: exchange.RedirectPolicy{
			Mode: exchange.RedirectReject,
		},
	}
}

// Request is the provider-independent acquisition intent.
type Request struct {
	Audience Audience
	Policy   Policy
}

// Validate checks the exact audience and bounded operation policy.
func (r Request) Validate() error {
	if err := r.Audience.Validate(); err != nil {
		return err
	}
	return r.Policy.Validate()
}

// Client is an immutable capability over one caller-owned Exchange client.
type Client struct {
	exchange exchange.Client
}

// NewClient constructs one Cloudidentity client.
func NewClient(client exchange.Client) (Client, error) {
	value := Client{exchange: client}
	if err := value.Validate(); err != nil {
		return Client{}, err
	}
	return value, nil
}

// Validate rejects an unset Exchange capability.
func (c Client) Validate() error {
	if err := c.exchange.Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

// AmazonWebServicesRequestInput supplies one exact common request and a
// caller-created SigV4 query-signed regional STS GET capability.
type AmazonWebServicesRequestInput struct {
	SignedURL string
	Request   Request
}

// AmazonWebServicesRequest is one validated, redacted AWS acquisition
// capability. It has no URL accessor.
type AmazonWebServicesRequest struct {
	request  Request
	endpoint core.HTTPEndpoint
	set      bool
}

// NewAmazonWebServicesRequest parses and owns one exact signed AWS request.
func NewAmazonWebServicesRequest(
	input AmazonWebServicesRequestInput,
) (AmazonWebServicesRequest, error) {
	if err := input.Request.Validate(); err != nil {
		return AmazonWebServicesRequest{}, err
	}
	endpoint, err := core.ParseHTTPEndpoint(input.SignedURL)
	if err != nil {
		return AmazonWebServicesRequest{}, amazonFailure(contractError(err))
	}
	value := AmazonWebServicesRequest{
		request: input.Request, endpoint: endpoint, set: true,
	}
	if err := value.Validate(); err != nil {
		return AmazonWebServicesRequest{}, amazonFailure(err)
	}
	return value, nil
}

// Validate checks the common request and exact AWS signed-query contract.
func (r AmazonWebServicesRequest) Validate() error {
	if !r.set {
		return core.ErrCloudIdentityContract
	}
	if err := r.request.Validate(); err != nil {
		return err
	}
	if err := r.endpoint.Validate(); err != nil {
		return contractError(err)
	}
	return validateAmazonWebServicesEndpoint(
		r.endpoint.HTTPURL(),
		r.request.Audience,
	)
}

// Format redacts the signed capability for every formatting verb.
func (AmazonWebServicesRequest) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

var (
	_ core.Validatable = ProviderUnknown
	_ core.OffWireEnum = ProviderUnknown
	_ core.Validatable = Audience{}
	_ core.Validatable = Policy{}
	_ core.Validatable = Request{}
	_ core.Validatable = Client{}
	_ core.Validatable = AmazonWebServicesRequest{}
)
