package googleidentity

import (
	"context"
	json "encoding/json/v2"
	"errors"
	"net/http"

	"cloud.google.com/go/auth"
	"cloud.google.com/go/auth/credentials"
	"github.com/deliri/primitive/v2026/core"
)

// #nosec G101 -- Public Google OAuth endpoint, not a credential.
const googleServiceAccountTokenURL = "https://oauth2.googleapis.com/token"
const googleServiceAccountUniverse = "googleapis.com"

// serviceAccountDocument owns the documented credential-file shape. Only
// service-account credentials enter this adapter; no ambient discovery runs.
type serviceAccountDocument struct {
	Type                    credentials.CredType `json:"type"`
	ProjectID               string               `json:"project_id,omitempty"`
	PrivateKeyID            string               `json:"private_key_id,omitempty"`
	PrivateKey              string               `json:"private_key"`
	ClientEmail             string               `json:"client_email"`
	ClientID                string               `json:"client_id,omitempty"`
	AuthURI                 string               `json:"auth_uri,omitempty"`
	TokenURI                string               `json:"token_uri,omitempty"`
	AuthProviderX509CertURL string               `json:"auth_provider_x509_cert_url,omitempty"`
	ClientX509CertURL       string               `json:"client_x509_cert_url,omitempty"`
	UniverseDomain          string               `json:"universe_domain,omitempty"`
	QuotaProjectID          string               `json:"quota_project_id,omitempty"`
}

func (d serviceAccountDocument) Validate() error {
	if d.Type != credentials.ServiceAccount || d.ClientEmail == "" || d.PrivateKey == "" {
		return core.ErrGoogleIdentityContract
	}
	if d.TokenURI != "" {
		if _, err := core.ParseHTTPEndpoint(d.TokenURI); err != nil {
			return contractError(err)
		}
	}
	return nil
}

// serviceAccountAudienceClaim is the compiler-owned payload supplied to the
// SDK's additional-claims field. The SDK signs and sends it; no map escapes.
type serviceAccountAudienceClaim struct {
	TargetAudience Audience `json:"target_audience"`
}

func (c serviceAccountAudienceClaim) Validate() error { return c.TargetAudience.Validate() }

func serviceAccountToken(ctx context.Context, client *http.Client, audience Audience, document []byte) (Token, error) {
	wire, err := core.DecodeStrictJSONBytes[serviceAccountDocument](document, core.ExtensibleJSONLimits())
	if err != nil {
		return Token{}, contractError(err)
	}
	if wire.UniverseDomain != "" && wire.UniverseDomain != googleServiceAccountUniverse {
		return serviceAccountIAMToken(ctx, client, audience, document, wire)
	}
	options, err := serviceAccountOptions(wire, audience, client)
	if err != nil {
		return Token{}, err
	}
	defer clear(options.PrivateKey)
	provider, err := auth.New2LOTokenProvider(&options)
	if err != nil {
		return Token{}, contractError(err)
	}
	return tokenFromGoogleProvider(ctx, provider)
}
func serviceAccountOptions(wire serviceAccountDocument, audience Audience, client *http.Client) (auth.Options2LO, error) {
	endpoint := wire.TokenURI
	if endpoint == "" {
		endpoint = googleServiceAccountTokenURL
	}
	options := auth.Options2LO{Email: wire.ClientEmail, PrivateKey: []byte(wire.PrivateKey), PrivateKeyID: wire.PrivateKeyID, TokenURL: endpoint, UseIDToken: true, Client: client}
	claim := serviceAccountAudienceClaim{TargetAudience: audience}
	if err := claim.Validate(); err != nil {
		clear(options.PrivateKey)
		return auth.Options2LO{}, err
	}
	encoded, err := core.MarshalCanonicalJSONDocument(claim)
	if err != nil {
		clear(options.PrivateKey)
		return auth.Options2LO{}, contractError(err)
	}
	// Google's SDK requires its map-shaped PrivateClaims field. Encode from the
	// typed owner so the wire key is defined once and remains compiler-visible.
	if err := json.Unmarshal(encoded, &options.PrivateClaims); err != nil {
		clear(options.PrivateKey)
		return auth.Options2LO{}, contractError(errors.Join(core.ErrJSONContract, err))
	}
	return options, nil
}
func tokenFromGoogleProvider(ctx context.Context, provider auth.TokenProvider) (Token, error) {
	token, err := provider.Token(ctx)
	if err != nil {
		return Token{}, contractError(err)
	}
	if token == nil {
		return Token{}, core.ErrGoogleIdentityContract
	}
	return newToken(token.Value)
}
func (serviceAccountDocument) googleIdentityProtocolFact()      {}
func (serviceAccountAudienceClaim) googleIdentityProtocolFact() {}
