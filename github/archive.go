package github

import (
	"context"
	"errors"
	"io"
	"net/url"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	// GitHub documents 302 as the archive endpoint's success response.
	githubArchiveRedirectStatus = 302
	// GitHub publishes no redirect-body bound; this is Primitive custody for a
	// body whose useful capability is carried by Location.
	githubArchiveRedirectResponseMaximumBytes = 64 * 1024
	// GitHub publishes no archive transfer timeout; this Primitive custody bound
	// fits within the documented five-minute private-link lifetime.
	githubArchiveOperationTimeoutSeconds = 5 * 60
)

// GitHub archive protocol source:
// https://docs.github.com/en/rest/repos/contents?apiVersion=2026-03-10#download-a-repository-archive-tar
//
// The provider contract is GET /repos/{owner}/{repo}/tarball/{ref}, a 302
// response, and a second GET to the returned Location. Private-repository
// locations expire after five minutes and GitHub App installation tokens need
// Contents:read. Primitive therefore authenticates only the API request,
// observes rather than follows its redirect, and performs the second GET as a
// distinct credential-free transfer.

type archiveDestination struct {
	destination io.Writer
	digest      *core.DigestWriter
}

func (d archiveDestination) Write(data []byte) (int, error) {
	written, destinationErr := d.destination.Write(data)
	if written < 0 || written > len(data) {
		return 0, core.ErrGitHubResponse
	}
	if written > 0 {
		digestWritten, digestErr := d.digest.Write(data[:written])
		if digestErr != nil || digestWritten != written {
			return written, errors.Join(core.ErrGitHubResponse, digestErr)
		}
	}
	if destinationErr != nil {
		return written, destinationErr
	}
	if written != len(data) {
		return written, io.ErrShortWrite
	}
	return written, nil
}

// ReadTarArchive streams GitHub's tar archive for one immutable commit into
// the caller-owned destination. It retains no repository-wide collection.
func (c Client) ReadTarArchive(ctx context.Context, request TarArchiveRequest) (TarArchiveObservation, error) {
	if err := errors.Join(c.Validate(), request.Validate()); err != nil {
		return TarArchiveObservation{}, contractError(err)
	}
	target, err := c.readTarArchiveLocation(ctx, request.Repository, request.Commit)
	if err != nil {
		return TarArchiveObservation{}, err
	}
	return c.downloadTarArchive(ctx, target, request)
}

func (c Client) readTarArchiveLocation(ctx context.Context, repository Repository, commit core.BuildCommit) (core.HTTPEndpoint, error) {
	headers, _, err := c.headers(ctx, false)
	if err != nil {
		return core.HTTPEndpoint{}, err
	}
	target, err := c.target(repositoryPath(repository)+"/tarball/"+url.PathEscape(commit.String()), nil)
	if err != nil {
		return core.HTTPEndpoint{}, err
	}
	location, locationErr := core.ParseHTTPHeaderName(headerLocation)
	limit, limitErr := core.NewByteCount(githubArchiveRedirectResponseMaximumBytes)
	timeout, timeoutErr := temporal.DurationFromSeconds(core.GitHubOperationCustodyTimeoutSeconds)
	status, statusErr := expectedStatus(githubArchiveRedirectStatus)
	if err := errors.Join(locationErr, limitErr, timeoutErr, statusErr); err != nil {
		return core.HTTPEndpoint{}, contractError(err)
	}
	response, err := exchange.SendNoBodyBounded(exchange.NoBodyBoundedCall{
		Context: ctx,
		Client:  c.state.client,
		Request: exchange.NoBodyBoundedRequest{
			Target: target,
			Semantics: exchange.RequestSemantics{
				Method: exchange.MethodGet,
				Replay: exchange.ReplaySingleAttempt,
			},
			Headers:        headers,
			CaptureHeaders: exchange.HeaderSelection{Names: []core.HTTPHeaderName{location}},
			ExpectedStatus: status,
		},
		Policy: exchange.NoBodyBoundedPolicy{
			Operation: exchange.OperationPolicy{
				OperationTimeout: timeout,
				AttemptTimeout:   timeout,
				Retry:            exchange.RetryPolicy{MaximumAttempts: 1},
				Redirect:         exchange.RedirectPolicy{Mode: exchange.RedirectObserve},
			},
			ResponseBodyLimit: limit,
		},
	})
	if err != nil {
		return core.HTTPEndpoint{}, classifyExchangeError(err)
	}
	return archiveLocation(response.Metadata.Headers, location)
}

func archiveLocation(headers exchange.CapturedHeaders, location core.HTTPHeaderName) (core.HTTPEndpoint, error) {
	if err := headers.Validate(); err != nil || len(headers.Values) != 1 || headers.Values[0].Name != location || len(headers.Values[0].Values) != 1 {
		return core.HTTPEndpoint{}, bindingError(err)
	}
	value, err := headers.Values[0].Values[0].Value()
	if err != nil {
		return core.HTTPEndpoint{}, bindingError(err)
	}
	target, err := core.ParseHTTPEndpoint(value)
	if err != nil {
		return core.HTTPEndpoint{}, bindingError(err)
	}
	parsed := target.HTTPURL()
	if parsed.Scheme != core.SchemeHTTPS && !isLoopbackAuthority(parsed) {
		return core.HTTPEndpoint{}, core.ErrGitHubBinding
	}
	return target, nil
}

func (c Client) downloadTarArchive(ctx context.Context, target core.HTTPEndpoint, request TarArchiveRequest) (TarArchiveObservation, error) {
	headers, err := c.archiveDownloadHeaders()
	if err != nil {
		return TarArchiveObservation{}, err
	}
	timeout, err := temporal.DurationFromSeconds(githubArchiveOperationTimeoutSeconds)
	if err != nil {
		return TarArchiveObservation{}, contractError(err)
	}
	digest := core.NewDigestWriter()
	destination := archiveDestination{destination: request.Destination, digest: digest}
	response, transferErr := exchange.Download(exchange.DownloadCall{
		Context: ctx,
		Client:  c.state.client,
		Request: exchange.DownloadRequest{
			Target: target, Destination: destination,
			Semantics:         exchange.RequestSemantics{Method: exchange.MethodGet, Replay: exchange.ReplaySingleAttempt},
			Headers:           headers,
			ResponseBodyLimit: request.MaximumBytes,
			ExpectedStatus:    core.HTTPStatusOK(),
		},
		Policy: exchange.StreamPolicy{
			OperationTimeout: timeout,
			AttemptTimeout:   timeout,
			ErrorBodyLimit:   request.MaximumBytes,
			Redirect:         exchange.RedirectPolicy{Mode: exchange.RedirectReject},
		},
	})
	sha256, length, sealErr := digest.Seal()
	state := ArchiveTransferComplete
	if transferErr != nil || sealErr != nil {
		state = ArchiveTransferIncomplete
	}
	observation := TarArchiveObservation{
		Repository: request.Repository, Commit: request.Commit,
		SHA256: sha256, Length: length, State: state,
	}
	if transferErr == nil && response.Metadata.Bytes != length {
		transferErr = core.ErrGitHubResponse
		observation.State = ArchiveTransferIncomplete
	}
	if transferErr == nil && length.Uint64() == 0 {
		transferErr = core.ErrGitHubResponse
		observation.State = ArchiveTransferIncomplete
	}
	validationErr := observation.Validate()
	if err := errors.Join(classifyExchangeError(transferErr), sealErr, validationErr); err != nil {
		return observation, err
	}
	return observation, nil
}

func (c Client) archiveDownloadHeaders() (exchange.Headers, error) {
	name, nameErr := core.ParseHTTPHeaderName(headerUserAgent)
	value, valueErr := exchange.NewHeaderValue(c.state.userAgent.String())
	if err := errors.Join(nameErr, valueErr); err != nil {
		return exchange.Headers{}, contractError(err)
	}
	headers := exchange.Headers{Values: []exchange.Header{{Name: name, Values: []exchange.HeaderValue{value}}}}
	if err := headers.Validate(); err != nil {
		return exchange.Headers{}, contractError(err)
	}
	return headers, nil
}

var _ io.Writer = archiveDestination{}
