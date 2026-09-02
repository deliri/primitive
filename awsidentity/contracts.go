package awsidentity

import (
	"fmt"
	"io"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	// AudienceMaximumBytes is AWS's published GetWebIdentityToken audience
	// ceiling, expressed as bytes for deterministic request bounds.
	AudienceMaximumBytes = 1000
	// TokenMaximumBytes bounds one AWS identity token before disclosure.
	TokenMaximumBytes = 16 * 1024
	// DefaultTimeoutSeconds is the complete default acquisition budget.
	DefaultTimeoutSeconds = 5
)

// Audience is one exact AWS GetWebIdentityToken receiver identifier.
type Audience struct{ value string }

// ParseAudience owns one nonempty bounded UTF-8 AWS audience.
func ParseAudience(value string) (Audience, error) {
	if value == "" || len(value) > AudienceMaximumBytes || !utf8.ValidString(value) {
		return Audience{}, core.ErrAWSIdentityContract
	}
	return Audience{value: value}, nil
}

// Validate rejects an unset, oversized, or invalid UTF-8 audience.
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

// Policy owns the complete and per-attempt bounds for one AWS call.
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
	return Policy{OperationTimeout: timeout, AttemptTimeout: timeout}, nil
}

// Validate closes the timeout pair and fixed single-attempt behavior.
func (p Policy) Validate() error { return contractErrorIfInvalid(p.exchange()) }

func (p Policy) exchange() exchange.OperationPolicy {
	return exchange.OperationPolicy{
		OperationTimeout: p.OperationTimeout,
		AttemptTimeout:   p.AttemptTimeout,
		Retry:            exchange.RetryPolicy{MaximumAttempts: 1},
		Redirect:         exchange.RedirectPolicy{Mode: exchange.RedirectReject},
	}
}

// Client is an immutable capability over one caller-owned Exchange client.
type Client struct{ exchange exchange.Client }

// NewClient constructs one AWS identity client.
func NewClient(client exchange.Client) (Client, error) {
	value := Client{exchange: client}
	if err := value.Validate(); err != nil {
		return Client{}, err
	}
	return value, nil
}

// Validate rejects an unset Exchange capability.
func (c Client) Validate() error { return contractErrorIfInvalid(c.exchange) }

// RequestInput supplies a caller-created SigV4 query-signed regional STS GET
// capability and the exact provider request facts it must carry.
type RequestInput struct {
	SignedURL string
	Audience  Audience
	Policy    Policy
}

// Validate admits one exact provider-signed capability without retaining a
// parsed endpoint outside the owned Request.
func (i RequestInput) Validate() error {
	_, err := i.endpoint()
	return err
}

func (i RequestInput) endpoint() (core.HTTPEndpoint, error) {
	if err := i.Audience.Validate(); err != nil {
		return core.HTTPEndpoint{}, err
	}
	if err := i.Policy.Validate(); err != nil {
		return core.HTTPEndpoint{}, err
	}
	endpoint, err := core.ParseHTTPEndpoint(i.SignedURL)
	if err != nil {
		return core.HTTPEndpoint{}, requestFailure(contractError(err))
	}
	if err := validateAmazonWebServicesEndpoint(endpoint.HTTPURL(), i.Audience); err != nil {
		return core.HTTPEndpoint{}, requestFailure(err)
	}
	return endpoint, nil
}

// Request is one validated and redacted AWS acquisition capability. It has no
// URL accessor so diagnostics cannot disclose the signed query.
type Request struct {
	endpoint *core.HTTPEndpoint
	audience Audience
	policy   Policy
}

// NewRequest parses and owns one exact signed AWS request.
func NewRequest(input RequestInput) (Request, error) {
	endpoint, err := input.endpoint()
	if err != nil {
		return Request{}, err
	}
	value := Request{endpoint: &endpoint, audience: input.Audience, policy: input.Policy}
	if err := value.Validate(); err != nil {
		return Request{}, requestFailure(err)
	}
	return value, nil
}

// Validate checks the complete AWS signed-query contract.
func (r Request) Validate() error {
	if r.endpoint == nil {
		return core.ErrAWSIdentityContract
	}
	if err := r.audience.Validate(); err != nil {
		return err
	}
	if err := r.policy.Validate(); err != nil {
		return err
	}
	if err := r.endpoint.Validate(); err != nil {
		return contractError(err)
	}
	return validateAmazonWebServicesEndpoint(r.endpoint.HTTPURL(), r.audience)
}

// Format redacts the signed capability for every formatting verb.
func (Request) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

func contractErrorIfInvalid(value core.Validatable) error {
	if err := value.Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

var (
	_ core.Validatable = Audience{}
	_ core.Validatable = Policy{}
	_ core.Validatable = Client{}
	_ core.Validatable = RequestInput{}
	_ core.Validatable = Request{}
	_ fmt.Formatter    = Request{}
)
