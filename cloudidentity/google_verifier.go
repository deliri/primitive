package cloudidentity

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
	"google.golang.org/api/idtoken"
	"google.golang.org/api/option"
)

const (
	GoogleCloudIdentityCertificateMaximumBytes = 256 << 10
	GoogleCloudIdentityTextMaximumBytes        = 1024
	googleCloudIdentityEmailClaim              = "email"
	googleCloudIdentityBearerPrefix            = "Bearer "
)

// GoogleCloudVerifierConfiguration fixes the one audience accepted by a
// verifier. Product code separately decides which verified principal is
// authorized for a capability.
type GoogleCloudVerifierConfiguration struct {
	Audience Audience
}

func (c GoogleCloudVerifierConfiguration) Validate() error { return c.Audience.Validate() }

// GoogleCloudVerifiedIdentity contains only signature-verified provider facts.
// It grants no product permission by itself.
type GoogleCloudVerifiedIdentity struct {
	Audience string
	Issuer   string
	Subject  string
	Email    string
	IssuedAt temporal.Instant
	Expires  temporal.Instant
}

func (i GoogleCloudVerifiedIdentity) Validate() error {
	if !validGoogleCloudIdentityText(i.Audience) || !validGoogleCloudIdentityText(i.Issuer) ||
		!validGoogleCloudIdentityText(i.Subject) || !validGoogleCloudIdentityText(i.Email) {
		return core.ErrCloudIdentityContract
	}
	if err := errors.Join(i.IssuedAt.Validate(), i.Expires.Validate()); err != nil {
		return contractError(err)
	}
	comparison, err := i.IssuedAt.Compare(i.Expires)
	if err != nil || comparison != core.ComparisonLess {
		return contractError(errors.Join(core.ErrCloudIdentityContract, err))
	}
	return nil
}

func validGoogleCloudIdentityText(value string) bool {
	return value != "" && len(value) <= GoogleCloudIdentityTextMaximumBytes &&
		utf8.ValidString(value) && strings.TrimSpace(value) == value
}

// GoogleCloudVerifier owns Google's official signature verifier behind a
// Primitive Exchange response boundary.
type GoogleCloudVerifier struct {
	validator *idtoken.Validator
	audience  Audience
}

func NewGoogleCloudVerifier(ctx context.Context, configuration GoogleCloudVerifierConfiguration) (GoogleCloudVerifier, error) {
	if err := errors.Join(contextstate.Validate(ctx), configuration.Validate()); err != nil {
		return GoogleCloudVerifier{}, contractError(err)
	}
	maximum, err := core.NewByteCount(GoogleCloudIdentityCertificateMaximumBytes)
	if err != nil {
		return GoogleCloudVerifier{}, contractError(err)
	}
	boundary, err := exchange.NewOfficialSDKResponseCeiling(exchange.OfficialSDKResponseCeilingRequest{
		Method: exchange.MethodGet, Representation: exchange.OfficialSDKResponseRepresentationJSON, MaximumBytes: maximum,
	})
	if err != nil {
		return GoogleCloudVerifier{}, contractError(err)
	}
	transport, err := exchange.NewStandardOfficialSDKResponseTransport(boundary)
	if err != nil {
		return GoogleCloudVerifier{}, contractError(err)
	}
	httpClient, err := exchange.NewOfficialSDKHTTPClient(transport)
	if err != nil {
		return GoogleCloudVerifier{}, contractError(err)
	}
	validator, err := idtoken.NewValidator(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return GoogleCloudVerifier{}, contractError(err)
	}
	value := GoogleCloudVerifier{validator: validator, audience: configuration.Audience}
	return value, value.Validate()
}

func (v GoogleCloudVerifier) Validate() error {
	if v.validator == nil {
		return core.ErrCloudIdentityContract
	}
	return v.audience.Validate()
}

// Verify authenticates one RFC 6750 Authorization value and returns only
// signature-verified Google identity facts.
func (v GoogleCloudVerifier) Verify(ctx context.Context, authorization string) (GoogleCloudVerifiedIdentity, error) {
	if err := errors.Join(contextstate.Validate(ctx), v.Validate()); err != nil {
		return GoogleCloudVerifiedIdentity{}, contractError(err)
	}
	token, err := googleCloudIdentityToken(authorization)
	if err != nil {
		return GoogleCloudVerifiedIdentity{}, err
	}
	payload, err := v.validator.Validate(ctx, token, v.audience.String())
	if err != nil {
		return GoogleCloudVerifiedIdentity{}, contractError(err)
	}
	identity, err := googleCloudVerifiedIdentity(payload)
	if err != nil {
		return GoogleCloudVerifiedIdentity{}, err
	}
	return identity, nil
}

func googleCloudIdentityToken(authorization string) (string, error) {
	if len(authorization) <= len(googleCloudIdentityBearerPrefix) || len(authorization) > TokenMaximumBytes+len(googleCloudIdentityBearerPrefix) ||
		!strings.HasPrefix(authorization, googleCloudIdentityBearerPrefix) {
		return "", core.ErrCloudIdentityContract
	}
	value := authorization[len(googleCloudIdentityBearerPrefix):]
	if err := validateBearerTokenValue(value); err != nil {
		return "", err
	}
	return value, nil
}

func googleCloudVerifiedIdentity(payload *idtoken.Payload) (GoogleCloudVerifiedIdentity, error) {
	if payload == nil {
		return GoogleCloudVerifiedIdentity{}, core.ErrCloudIdentityContract
	}
	email, ok := payload.Claims[googleCloudIdentityEmailClaim].(string)
	if !ok {
		return GoogleCloudVerifiedIdentity{}, core.ErrCloudIdentityContract
	}
	issuedAt, err := temporal.NewInstant(time.Unix(payload.IssuedAt, 0).UTC())
	if err != nil {
		return GoogleCloudVerifiedIdentity{}, contractError(err)
	}
	expires, err := temporal.NewInstant(time.Unix(payload.Expires, 0).UTC())
	if err != nil {
		return GoogleCloudVerifiedIdentity{}, contractError(err)
	}
	identity := GoogleCloudVerifiedIdentity{
		Audience: payload.Audience, Issuer: payload.Issuer, Subject: payload.Subject, Email: email,
		IssuedAt: issuedAt, Expires: expires,
	}
	if err := identity.Validate(); err != nil {
		return GoogleCloudVerifiedIdentity{}, err
	}
	return identity, nil
}

var (
	_ core.Validatable = GoogleCloudVerifierConfiguration{}
	_ core.Validatable = GoogleCloudVerifiedIdentity{}
	_ core.Validatable = GoogleCloudVerifier{}
)
