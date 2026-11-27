package googleidentity

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// Delegation uses Google's server-to-server JWT bearer flow and a Google-managed
// signing key. The attached runtime credential authorizes IAM, the signed
// assertion authorizes the OAuth exchange, and the OAuth bearer authorizes Gmail.
// Use a distinct attached runtime and delegated signer to stay within Google's
// short-lived credential self-impersonation rules.
// https://developers.google.com/identity/protocols/oauth2/service-account
// https://docs.cloud.google.com/iam/docs/best-practices-for-managing-service-account-keys#domain-wide-delegation
// https://docs.cloud.google.com/iam/docs/reference/credentials/rest/v1/projects.serviceAccounts/signJwt
// https://docs.cloud.google.com/iam/docs/service-account-creds#self-impersonation
// https://docs.cloud.google.com/appengine/migration-center/run/compare-gae-with-run#metadata-server
// https://developers.google.com/workspace/gmail/api/auth/scopes
const (
	delegationAssertionSeconds = 300
	delegationLifetimeMaximum  = 3600
	workspaceEmailMaximum      = 254
)

// ServiceAccountEmail identifies one service account in the public Google universe.
type ServiceAccountEmail struct{ address core.OAuthEmail }

func ParseServiceAccountEmail(value string) (ServiceAccountEmail, error) {
	address, err := parseWorkspaceEmail(value)
	if err != nil {
		return ServiceAccountEmail{}, err
	}
	_, domain, _ := strings.Cut(value, "@")
	if !strings.HasSuffix(domain, ".iam.gserviceaccount.com") &&
		domain != "developer.gserviceaccount.com" && domain != "appspot.gserviceaccount.com" {
		return ServiceAccountEmail{}, core.ErrGoogleIdentityContract
	}
	return ServiceAccountEmail{address: address}, nil
}

func (a ServiceAccountEmail) String() string { return a.address.String() }
func (a ServiceAccountEmail) Validate() error {
	_, err := ParseServiceAccountEmail(a.String())
	return err
}

// WorkspaceSubject identifies the single Workspace mailbox asserted in sub.
type WorkspaceSubject struct{ address core.OAuthEmail }

func ParseWorkspaceSubject(value string) (WorkspaceSubject, error) {
	address, err := parseWorkspaceEmail(value)
	if err != nil {
		return WorkspaceSubject{}, err
	}
	return WorkspaceSubject{address: address}, nil
}

func (s WorkspaceSubject) Email() core.OAuthEmail { return s.address }
func (s WorkspaceSubject) Validate() error {
	_, err := ParseWorkspaceSubject(s.address.String())
	return err
}

func parseWorkspaceEmail(value string) (core.OAuthEmail, error) {
	if len(value) == 0 || len(value) > workspaceEmailMaximum || !strings.Contains(value, "@") {
		return core.OAuthEmail{}, core.ErrGoogleIdentityContract
	}
	// Restrict the address to ASCII account names and path-safe characters.
	// Google profile equality is case-insensitive. The JWT preserves the input.
	for _, ch := range value {
		if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9') || strings.ContainsRune("@._+-'", ch)) {
			return core.OAuthEmail{}, core.ErrGoogleIdentityContract
		}
	}
	if address, err := core.ParseOAuthEmail(value); err == nil {
		return address, nil
	}
	return core.OAuthEmail{}, core.ErrGoogleIdentityContract
}

type DelegatedScope uint8

const (
	DelegatedScopeInvalid DelegatedScope = iota
	DelegatedScopeGmailReadOnly
	GmailReadOnlyScope = "https://www.googleapis.com/auth/gmail.readonly"
)

func (DelegatedScope) OffWireEnum()    {}
func (s DelegatedScope) IsValid() bool { return s == DelegatedScopeGmailReadOnly }
func (s DelegatedScope) Validate() error {
	_, err := s.wire()
	return err
}

func (s DelegatedScope) String() string {
	if s.IsValid() {
		return GmailReadOnlyScope
	}
	return ""
}

func (s DelegatedScope) wire() (string, error) {
	if !s.IsValid() {
		return "", core.ErrGoogleIdentityContract
	}
	return s.String(), nil
}

// DelegatedAccessRequest is one dated assertion and exchange intent.
type DelegatedAccessRequest struct {
	Signer     ServiceAccountEmail
	Subject    WorkspaceSubject
	Scope      DelegatedScope
	ObservedAt temporal.Instant
	Policy     Policy
}

func (r DelegatedAccessRequest) Validate() error {
	nanos, err := r.ObservedAt.Nanoseconds()
	if err != nil || nanos < int64(temporal.NanosecondsPerSecond) {
		return core.ErrGoogleIdentityContract
	}
	margin, err := temporal.DurationFromSeconds(delegationLifetimeMaximum)
	if err != nil {
		return contractError(err)
	}
	if _, err := r.ObservedAt.Add(margin); err != nil {
		return contractError(err)
	}
	return errors.Join(r.Signer.Validate(), r.Subject.Validate(), r.Scope.Validate(), r.Policy.Validate())
}

// DelegatedAccessToken holds one short-lived OAuth bearer in memory. Copies
// share destruction. CopyBearer gives one independent transient copy.
type DelegatedAccessToken struct {
	secret   core.OAuthClientSecret
	start    temporal.Instant
	expiry   temporal.Instant
	lifetime temporal.Duration
}

func (t DelegatedAccessToken) ExpiresAt() temporal.Instant { return t.expiry }
func (t DelegatedAccessToken) Lifetime() temporal.Duration { return t.lifetime }
func (t DelegatedAccessToken) Validate() error {
	end, err := t.start.Add(t.lifetime)
	if err != nil || t.lifetime.IsZero() || end != t.expiry {
		return core.ErrGoogleDelegationResponse
	}
	return t.secret.Validate()
}

// FreshAt permits reuse before the lesser of 60 seconds or a tenth of the
// minted lifetime remains. A backward wall-clock correction forces renewal.
func (t DelegatedAccessToken) FreshAt(at temporal.Instant) bool {
	now, nowErr := at.Nanoseconds()
	start, startErr := t.start.Nanoseconds()
	end, endErr := t.expiry.Nanoseconds()
	if nowErr != nil || startErr != nil || endErr != nil || now < start || t.Validate() != nil {
		return false
	}
	margin := min(int64(60*temporal.NanosecondsPerSecond), (end-start)/10)
	return now < end-margin
}

func (t DelegatedAccessToken) CopyBearer() (core.OAuthClientSecret, error) {
	if err := t.Validate(); err != nil {
		return core.OAuthClientSecret{}, err
	}
	raw, err := t.secret.CopyBytes()
	if err != nil {
		return core.OAuthClientSecret{}, err
	}
	defer clear(raw)
	return core.NewOAuthClientSecret(raw)
}

func (t *DelegatedAccessToken) Close() error {
	if t == nil {
		return core.ErrGoogleIdentityContract
	}
	return t.secret.Close()
}
func (DelegatedAccessToken) Format(s fmt.State, _ rune) {
	_, _ = io.WriteString(s, core.RedactedValueText)
}
func (DelegatedAccessToken) MarshalJSON() ([]byte, error) {
	return nil, errors.Join(core.ErrGoogleIdentityContract, core.ErrJSONContract)
}

// AcquireDelegatedAccess obtains a single bearer using the runtime metadata
// identity and Google's managed signer. Its caller owns caching and retries.
func AcquireDelegatedAccess(ctx context.Context, client Client, request DelegatedAccessRequest) (DelegatedAccessToken, error) {
	if err := errors.Join(validateAcquisition(client, request), contextstate.Validate(ctx)); err != nil {
		return DelegatedAccessToken{}, contractError(err)
	}
	ctx, cancel, err := temporal.WithTimeout(temporal.TimeoutRequest{Parent: ctx, Duration: request.Policy.OperationTimeout})
	if err != nil {
		return DelegatedAccessToken{}, contractError(err)
	}
	defer cancel()
	runtime, err := AcquireGoogleCloudAccessToken(ctx, client, GoogleCloudAccessTokenRequest{Policy: request.Policy})
	if err != nil {
		return DelegatedAccessToken{}, err
	}
	assertion, err := signDelegatedAssertion(ctx, client.exchange, request, runtime)
	if err != nil {
		return DelegatedAccessToken{}, err
	}
	defer clear(assertion.bytes)
	return exchangeDelegatedAssertion(ctx, client.exchange, request, assertion)
}

func (ServiceAccountEmail) googleIdentityProtocolFact()       {}
func (WorkspaceSubject) googleIdentityProtocolFact()          {}
func (DelegatedAccessRequest) googleIdentityProtocolFact()    {}
func (DelegatedAccessToken) googleIdentityCapabilityWrapper() {}
