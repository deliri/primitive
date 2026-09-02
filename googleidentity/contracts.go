package googleidentity

import (
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	// AudienceMaximumBytes bounds one Google Cloud identity audience.
	AudienceMaximumBytes = 1000
	// TokenMaximumBytes bounds one Google Cloud bearer.
	TokenMaximumBytes = 16 * 1024
	// DefaultTimeoutSeconds is the complete default acquisition budget.
	DefaultTimeoutSeconds = 5
)

// Audience is one exact Google Cloud identity receiver identifier.
type Audience struct {
	value string
}

// ParseAudience owns one nonempty bounded UTF-8 audience.
func ParseAudience(value string) (Audience, error) {
	if len(value) == 0 || len(value) > AudienceMaximumBytes ||
		!utf8.ValidString(value) {
		return Audience{}, core.ErrGoogleIdentityContract
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

// IdentityTokenRequest is one Google Cloud identity-token acquisition intent.
type IdentityTokenRequest struct {
	Audience Audience
	Policy   Policy
}

// Validate checks the exact audience and bounded operation policy.
func (r IdentityTokenRequest) Validate() error {
	if err := r.Audience.Validate(); err != nil {
		return err
	}
	return r.Policy.Validate()
}

// Client is an immutable no-proxy capability derived from one caller-owned
// Exchange client. Google metadata is cleartext link-local infrastructure;
// ambient proxy configuration must never mediate it.
type Client struct {
	exchange exchange.Client
}

// NewClient constructs one Google Cloud identity client.
func NewClient(client exchange.Client) (Client, error) {
	direct, err := client.WithoutProxy()
	if err != nil {
		return Client{}, contractError(err)
	}
	value := Client{exchange: direct}
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

var (
	_ core.Validatable = Audience{}
	_ core.Validatable = Policy{}
	_ core.Validatable = IdentityTokenRequest{}
	_ core.Validatable = Client{}
)
