package github

import (
	"context"
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	json "encoding/json/v2"
	"errors"
	"strconv"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/keygen"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	jwtAlgorithm                 = "RS256"
	jwtType                      = "JWT"
	installationAccessPathPrefix = "/app/installations/"
	installationTokenPathSuffix  = "/access_tokens"
)

type jwtHeader struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ"`
}

type jwtUnixSeconds int64

type jwtClaims struct {
	ExpiresAtUnixSeconds jwtUnixSeconds `json:"exp"`
	IssuedAtUnixSeconds  jwtUnixSeconds `json:"iat"`
	Issuer               uint64         `json:"iss"`
}

func (c Client) authorization(ctx context.Context) (string, error) {
	if c.state.credential.state == nil {
		return "", nil
	}
	c.state.mutex.Lock()
	defer c.state.mutex.Unlock()
	observed, err := c.state.observe()
	if err != nil {
		return "", authenticationError(err)
	}
	instant, err := observed.Instant()
	if err != nil {
		return "", authenticationError(err)
	}
	usable, err := c.state.token.usable(instant)
	if err != nil {
		return "", err
	}
	if usable {
		return c.state.token.value, nil
	}
	token, err := c.requestInstallationToken(ctx, instant)
	if err != nil {
		return "", err
	}
	c.state.token = token
	return token.value, nil
}

func (c Client) requestInstallationToken(ctx context.Context, now temporal.Instant) (installationToken, error) {
	jwt, err := signAppJWT(c.state.credential, now)
	if err != nil {
		return installationToken{}, err
	}
	defer clearString(&jwt)
	headers, _, err := c.baseHeaders(jwt)
	if err != nil {
		return installationToken{}, err
	}
	credential := c.state.credential.state
	installation, err := credential.installation.Uint64()
	if err != nil {
		return installationToken{}, authenticationError(err)
	}
	target, err := c.target(installationAccessPathPrefix+strconv.FormatUint(installation, 10)+installationTokenPathSuffix, nil)
	if err != nil {
		return installationToken{}, err
	}
	status, statusErr := expectedStatus(201)
	policy, policyErr := boundedPolicy(core.GitHubInstallationTokenResponseCustodyMaximumBytes)
	media, mediaErr := exchange.StandardMediaTypeJSON.HTTPMediaType()
	if err := errors.Join(statusErr, policyErr, mediaErr); err != nil {
		return installationToken{}, err
	}
	response, err := exchange.SendNoBodyBounded(exchange.NoBodyBoundedCall{
		Context: ctx,
		Client:  c.state.client,
		Request: exchange.NoBodyBoundedRequest{
			Target: target, Semantics: exchange.RequestSemantics{Method: exchange.MethodPost, Replay: exchange.ReplaySingleAttempt},
			ExpectedResponseContentType: media, Headers: headers, ExpectedStatus: status,
		},
		Policy: policy,
	})
	if err != nil {
		return installationToken{}, authenticationError(err)
	}
	token, err := decodeInstallationToken(response.Body, now)
	if err != nil {
		return installationToken{}, authenticationError(err)
	}
	return token, nil
}

func (c Client) baseHeaders(bearer string) (exchange.Headers, exchange.HeaderSelection, error) {
	userName, userNameErr := core.ParseHTTPHeaderName(headerUserAgent)
	versionName, versionNameErr := core.ParseHTTPHeaderName(headerAPIVersion)
	userValue, userValueErr := exchange.NewHeaderValue(c.state.userAgent.String())
	versionValue, versionValueErr := exchange.NewHeaderValue(core.GitHubAPIVersion)
	if err := errors.Join(userNameErr, versionNameErr, userValueErr, versionValueErr); err != nil {
		return exchange.Headers{}, exchange.HeaderSelection{}, contractError(err)
	}
	headers := exchange.Headers{Values: []exchange.Header{
		{Name: userName, Values: []exchange.HeaderValue{userValue}},
		{Name: versionName, Values: []exchange.HeaderValue{versionValue}},
	}}
	if bearer != "" {
		token := []byte(bearer)
		authorization, err := exchange.NewBearerAuthorizationHeader(exchange.BearerAuthorization{Token: token})
		clear(token)
		if err != nil {
			return exchange.Headers{}, exchange.HeaderSelection{}, authenticationError(err)
		}
		headers.Values = append(headers.Values, authorization)
	}
	return headers, exchange.HeaderSelection{}, headers.Validate()
}

func signAppJWT(credential AppCredential, now temporal.Instant) (string, error) {
	if err := credential.Validate(); err != nil {
		return "", err
	}
	app, err := credential.state.app.Uint64()
	if err != nil {
		return "", authenticationError(err)
	}
	issued, expires, instantErr := jwtClaimInstants(now)
	header, headerErr := json.Marshal(jwtHeader{Algorithm: jwtAlgorithm, Type: jwtType})
	claims, claimsErr := json.Marshal(jwtClaims{
		ExpiresAtUnixSeconds: expires,
		IssuedAtUnixSeconds:  issued,
		Issuer:               app,
	})
	if err := errors.Join(instantErr, headerErr, claimsErr); err != nil {
		return "", authenticationError(err)
	}
	unsigned := base64.RawURLEncoding.EncodeToString(header) + "." + base64.RawURLEncoding.EncodeToString(claims)
	digest := sha256.Sum256([]byte(unsigned))
	key, err := parsePrivateKey(credential.state.privateKey)
	if err != nil {
		return "", authenticationError(err)
	}
	signature, err := rsa.SignPKCS1v15(keygen.NewEntropyReader(), key, crypto.SHA256, digest[:])
	if err != nil {
		return "", authenticationError(err)
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature), nil
}

func jwtClaimInstants(now temporal.Instant) (jwtUnixSeconds, jwtUnixSeconds, error) {
	lifetime, lifetimeErr := temporal.DurationFromSeconds(core.GitHubAppJWTMaximumLifetimeSeconds - core.GitHubAppJWTClockSkewSeconds)
	skew, skewErr := temporal.DurationFromSeconds(core.GitHubAppJWTClockSkewSeconds)
	if err := errors.Join(lifetimeErr, skewErr); err != nil {
		return 0, 0, authenticationError(err)
	}
	expiresAt, expiresErr := now.Add(lifetime)
	issuedAt, issuedErr := now.Subtract(skew)
	if err := errors.Join(expiresErr, issuedErr); err != nil {
		return 0, 0, authenticationError(err)
	}
	issued, issuedErr := jwtClaimUnixSeconds(issuedAt)
	expires, expiresErr := jwtClaimUnixSeconds(expiresAt)
	if err := errors.Join(issuedErr, expiresErr); err != nil {
		return 0, 0, authenticationError(err)
	}
	return issued, expires, nil
}

func jwtClaimUnixSeconds(value temporal.Instant) (jwtUnixSeconds, error) {
	nanoseconds, err := value.Nanoseconds()
	if err != nil {
		return 0, err
	}
	return jwtUnixSeconds(nanoseconds / int64(temporal.NanosecondsPerSecond)), nil
}

// decodeInstallationToken trusts the response's exact expires_at observation;
// GitHub documents that installation tokens normally expire after one hour.
// Source: https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-an-installation-access-token-for-a-github-app
func decodeInstallationToken(payload []byte, now temporal.Instant) (installationToken, error) {
	var wire struct {
		Token     string `json:"token"`
		ExpiresAt string `json:"expires_at"`
	}
	if err := json.Unmarshal(payload, &wire); err != nil {
		return installationToken{}, responseError(err)
	}
	expiresAt, err := temporal.ParseRFC3339(wire.ExpiresAt)
	if err != nil || !validBearer(wire.Token) {
		return installationToken{}, responseError(err)
	}
	token := installationToken{value: wire.Token, expiresAt: expiresAt}
	usable, err := token.usable(now)
	if err != nil || !usable {
		return installationToken{}, responseError(err)
	}
	return token, nil
}

func validBearer(value string) bool {
	if value == "" || len(value) > exchange.BearerAuthorizationTokenMaximumBytes {
		return false
	}
	for index := range value {
		if !validBearerByte(value[index]) {
			return false
		}
	}
	return true
}

func validBearerByte(character byte) bool {
	letter := character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z'
	digit := character >= '0' && character <= '9'
	return letter || digit || strings.ContainsRune("-._~+/=", rune(character))
}
