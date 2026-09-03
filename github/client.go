package github

import (
	"context"
	"errors"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	apiAuthority     = "https://" + core.GitHubAPIHost
	headerAPIVersion = "X-GitHub-Api-Version"
	headerLink       = "Link"
	headerUserAgent  = "User-Agent"
)

type observationSource func() (temporal.Observation, error)

type installationToken struct {
	value     string
	expiresAt temporal.Instant
}

func (t installationToken) usable(now temporal.Instant) (bool, error) {
	if !validBearer(t.value) {
		return false, nil
	}
	refreshLead, err := temporal.DurationFromMinutes(1)
	if err != nil {
		return false, authenticationError(err)
	}
	minimumExpiry, err := now.Add(refreshLead)
	if err != nil {
		return false, authenticationError(err)
	}
	comparison, err := t.expiresAt.Compare(minimumExpiry)
	if err != nil {
		return false, authenticationError(err)
	}
	return comparison == core.ComparisonGreater, nil
}

type clientConstruction struct {
	client     exchange.Client
	credential AppCredential
	observe    observationSource
	userAgent  UserAgent
	authority  core.HTTPEndpoint
}

type clientState struct {
	client     exchange.Client
	credential AppCredential
	observe    observationSource
	userAgent  UserAgent
	authority  core.HTTPEndpoint
	token      installationToken
	mutex      sync.Mutex
}

// Client is one validated GitHub socket. The zero value is invalid.
type Client struct{ state *clientState }

// NewClient constructs a credential-free GitHub client for public resources.
func NewClient(client exchange.Client, userAgent UserAgent) (Client, error) {
	authority, err := core.ParseHTTPEndpoint(apiAuthority)
	if err != nil {
		return Client{}, contractError(err)
	}
	return newClient(clientConstruction{client: client, authority: authority, userAgent: userAgent, observe: temporal.Observe})
}

// NewAppClient constructs a GitHub App client and takes its own credential copy.
func NewAppClient(client exchange.Client, userAgent UserAgent, credential AppCredential) (Client, error) {
	authority, err := core.ParseHTTPEndpoint(apiAuthority)
	if err != nil {
		return Client{}, contractError(err)
	}
	return newClient(clientConstruction{
		client: client, authority: authority, userAgent: userAgent,
		credential: credential, observe: temporal.Observe,
	})
}

func newClient(construction clientConstruction) (Client, error) {
	if err := errors.Join(construction.client.Validate(), construction.userAgent.Validate(), construction.authority.Validate()); err != nil || construction.observe == nil {
		return Client{}, contractError(err)
	}
	owned := AppCredential{}
	if construction.credential.state != nil {
		var err error
		owned, err = construction.credential.clone()
		if err != nil {
			return Client{}, err
		}
	}
	candidate := Client{state: &clientState{
		client: construction.client, authority: construction.authority,
		userAgent: construction.userAgent, credential: owned, observe: construction.observe,
	}}
	if err := candidate.Validate(); err != nil {
		_ = owned.Close()
		return Client{}, err
	}
	return candidate, nil
}

// Validate checks transport, authority, caller identity, and optional App auth.
func (c Client) Validate() error {
	if c.state == nil || c.state.observe == nil {
		return core.ErrGitHubContract
	}
	if err := errors.Join(c.state.client.Validate(), c.state.authority.Validate(), c.state.userAgent.Validate()); err != nil {
		return contractError(err)
	}
	if err := validateAuthority(c.state.authority); err != nil {
		return err
	}
	if c.state.credential.state != nil {
		return c.state.credential.Validate()
	}
	return nil
}

func validateAuthority(authority core.HTTPEndpoint) error {
	providerURL := authority.HTTPURL()
	if providerURL.Path != "" || providerURL.RawQuery != "" || providerURL.Fragment != "" {
		return core.ErrGitHubBinding
	}
	if providerURL.Host == core.GitHubAPIHost && providerURL.Scheme != core.SchemeHTTPS {
		return core.ErrGitHubBinding
	}
	if providerURL.Host != core.GitHubAPIHost && !isLoopbackAuthority(providerURL) {
		return core.ErrGitHubBinding
	}
	return nil
}

func isLoopbackAuthority(value url.URL) bool {
	host := value.Hostname()
	return host == "127.0.0.1" || host == "::1" || host == "localhost"
}

// Close destroys App credential and cached-token bytes owned by this client.
func (c *Client) Close() error {
	if c == nil || c.state == nil {
		return core.ErrGitHubContract
	}
	c.state.mutex.Lock()
	defer c.state.mutex.Unlock()
	clearString(&c.state.token.value)
	c.state.token = installationToken{}
	err := error(nil)
	if c.state.credential.state != nil {
		err = c.state.credential.Close()
	}
	c.state = nil
	return err
}

func clearString(value *string) {
	if value != nil {
		*value = ""
	}
}

func (c Client) target(providerPath string, query url.Values) (core.HTTPEndpoint, error) {
	projected := c.state.authority.HTTPURL()
	projected.Path = providerPath
	projected.RawQuery = query.Encode()
	target, err := core.ParseHTTPEndpoint(projected.String())
	if err != nil {
		return core.HTTPEndpoint{}, bindingError(err)
	}
	return target, nil
}

func repositoryPath(repository Repository) string {
	return "/repos/" + url.PathEscape(repository.owner) + "/" + url.PathEscape(repository.name)
}

func sourcePath(value core.SourcePath) string {
	segments := strings.Split(value.String(), "/")
	for index := range segments {
		segments[index] = url.PathEscape(segments[index])
	}
	return strings.Join(segments, "/")
}

func (c Client) headers(ctx context.Context, captureLink bool) (exchange.Headers, exchange.HeaderSelection, error) {
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
	authorization, err := c.authorization(ctx)
	if err != nil {
		return exchange.Headers{}, exchange.HeaderSelection{}, err
	}
	if authorization != "" {
		token := []byte(authorization)
		header, headerErr := exchange.NewBearerAuthorizationHeader(exchange.BearerAuthorization{Token: token})
		clear(token)
		if headerErr != nil {
			return exchange.Headers{}, exchange.HeaderSelection{}, authenticationError(headerErr)
		}
		headers.Values = append(headers.Values, header)
	}
	selection := exchange.HeaderSelection{}
	if captureLink {
		name, nameErr := core.ParseHTTPHeaderName(headerLink)
		if nameErr != nil {
			return exchange.Headers{}, exchange.HeaderSelection{}, contractError(nameErr)
		}
		selection.Names = []core.HTTPHeaderName{name}
	}
	if err := errors.Join(headers.Validate(), selection.Validate()); err != nil {
		return exchange.Headers{}, exchange.HeaderSelection{}, contractError(err)
	}
	return headers, selection, nil
}

func boundedPolicy(maximum uint64) (exchange.NoBodyBoundedPolicy, error) {
	limit, limitErr := core.NewByteCount(maximum)
	timeout, timeoutErr := temporal.DurationFromSeconds(core.GitHubOperationCustodyTimeoutSeconds)
	if err := errors.Join(limitErr, timeoutErr); err != nil {
		return exchange.NoBodyBoundedPolicy{}, contractError(err)
	}
	return exchange.NoBodyBoundedPolicy{
		Operation: exchange.OperationPolicy{
			OperationTimeout: timeout,
			AttemptTimeout:   timeout,
			Retry:            exchange.RetryPolicy{MaximumAttempts: 1},
			Redirect:         exchange.RedirectPolicy{Mode: exchange.RedirectReject},
		},
		ResponseBodyLimit: limit,
	}, nil
}

func streamPolicy() (exchange.StreamPolicy, core.ByteCount, error) {
	limit, limitErr := core.NewByteCount(core.GitHubRecursiveTreeMaximumBytes)
	timeout, timeoutErr := temporal.DurationFromSeconds(core.GitHubOperationCustodyTimeoutSeconds)
	if err := errors.Join(limitErr, timeoutErr); err != nil {
		return exchange.StreamPolicy{}, core.ByteCount{}, contractError(err)
	}
	return exchange.StreamPolicy{
		OperationTimeout: timeout,
		AttemptTimeout:   timeout,
		ErrorBodyLimit:   limit,
		Redirect:         exchange.RedirectPolicy{Mode: exchange.RedirectReject},
	}, limit, nil
}

func expectedStatus(value int) (core.HTTPStatusCode, error) {
	status := core.HTTPStatusCode{}
	if err := status.AdmitInt(value); err != nil {
		return core.HTTPStatusCode{}, contractError(err)
	}
	return status, nil
}

func queryPage(page uint32) url.Values {
	return url.Values{
		"page":     []string{strconv.FormatUint(uint64(page), 10)},
		"per_page": []string{strconv.Itoa(core.GitHubTagPageMaximumEntries)},
	}
}

var _ core.Validatable = Client{}
