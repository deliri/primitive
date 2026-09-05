package googleidentity

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
	googleCloudIdentityEmailVerifiedClaim      = "email_verified"
	googleCloudIdentityBearerPrefix            = "Bearer "
	googleCloudIdentityIssuer                  = "https://accounts.google.com"
)

// GoogleCloudVerifierConfiguration fixes the one audience accepted by a
// verifier. Product code separately decides which verified principal is
// authorized for a capability.
type GoogleCloudVerifierConfiguration struct {
	Audience Audience
	Client   exchange.Client
}

func (c GoogleCloudVerifierConfiguration) Validate() error {
	return errors.Join(c.Audience.Validate(), c.Client.Validate())
}

// GoogleCloudVerifiedIdentity contains only signature-verified provider facts.
// It grants no product permission by itself.
type GoogleCloudVerifiedIdentity struct {
	Audience      string
	Issuer        string
	Subject       string
	Email         string
	EmailVerified bool
	IssuedAt      temporal.Instant
	Expires       temporal.Instant
}

func (i GoogleCloudVerifiedIdentity) Validate() error {
	if !validGoogleCloudIdentityText(i.Audience) || !validGoogleCloudIdentityText(i.Issuer) ||
		!validGoogleCloudIdentityText(i.Subject) || !validGoogleCloudIdentityText(i.Email) ||
		!validGoogleCloudIdentityIssuer(i.Issuer) || !i.EmailVerified {
		return core.ErrGoogleIdentityContract
	}
	if err := errors.Join(i.IssuedAt.Validate(), i.Expires.Validate()); err != nil {
		return contractError(err)
	}
	comparison, err := i.IssuedAt.Compare(i.Expires)
	if err != nil || comparison != core.ComparisonLess {
		return contractError(errors.Join(core.ErrGoogleIdentityContract, err))
	}
	return nil
}

// PrincipalIdentity derives the stable OpenID Connect issuer/subject pair.
// Google documents sub as the unique, never-reused account identifier; aud is
// a receiving-service binding, while email is an address rather than the
// principal key. This workload contract admits Google's current service-account
// issuer only.
func (i GoogleCloudVerifiedIdentity) PrincipalIdentity() (core.SHA256Digest, error) {
	if err := i.Validate(); err != nil {
		return core.SHA256Digest{}, err
	}
	projection := struct {
		Issuer  string `json:"issuer"`
		Subject string `json:"subject"`
	}{Issuer: i.Issuer, Subject: i.Subject}
	canonical, err := core.MarshalCanonicalJSONDocument(projection)
	if err != nil {
		return core.SHA256Digest{}, contractError(err)
	}
	return core.SHA256Of(canonical), nil
}

func validGoogleCloudIdentityIssuer(value string) bool {
	return value == googleCloudIdentityIssuer
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
	transport, err := configuration.Client.OfficialSDKResponseTransport(boundary)
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
		return core.ErrGoogleIdentityContract
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
		return "", core.ErrGoogleIdentityContract
	}
	value := authorization[len(googleCloudIdentityBearerPrefix):]
	if err := validateBearerTokenValue(value); err != nil {
		return "", err
	}
	if err := validateGoogleCloudJWTHeader(value); err != nil {
		return "", err
	}
	return value, nil
}

func googleCloudVerifiedIdentity(payload *idtoken.Payload) (GoogleCloudVerifiedIdentity, error) {
	if payload == nil {
		return GoogleCloudVerifiedIdentity{}, core.ErrGoogleIdentityContract
	}
	email, ok := payload.Claims[googleCloudIdentityEmailClaim].(string)
	if !ok {
		return GoogleCloudVerifiedIdentity{}, core.ErrGoogleIdentityContract
	}
	emailVerified, ok := payload.Claims[googleCloudIdentityEmailVerifiedClaim].(bool)
	if !ok || !emailVerified {
		return GoogleCloudVerifiedIdentity{}, core.ErrGoogleIdentityContract
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
		EmailVerified: emailVerified, IssuedAt: issuedAt, Expires: expires,
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
