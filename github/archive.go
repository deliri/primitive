package github

import (
	"context"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

const (
	// GitHub documents 302 as the archive endpoint's success response.
	githubArchiveRedirectStatus = 302
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

type downloadDestination struct {
	destination io.Writer
	digest      *core.DigestWriter
}

func (d downloadDestination) Write(data []byte) (int, error) {
	written, destinationErr := d.destination.Write(data)
	if written < 0 || written > len(data) {
		return 0, errors.Join(core.ErrGitHubResponse, destinationErr)
	}
	if written > 0 {
		digestWritten, digestErr := d.digest.Write(data[:written])
		if digestErr != nil || digestWritten != written {
			return written, errors.Join(core.ErrGitHubResponse, digestErr, destinationErr)
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
	target, err := c.target(repositoryPath(repository)+"/tarball/"+commit.String(), nil)
	if err != nil {
		return core.HTTPEndpoint{}, err
	}
	location, locationErr := core.ParseHTTPHeaderName(headerLocation)
	status, statusErr := expectedStatus(githubArchiveRedirectStatus)
	if err := errors.Join(locationErr, statusErr); err != nil {
		return core.HTTPEndpoint{}, contractError(err)
	}
	response, err := exchange.Download(exchange.DownloadCall{
		Context: ctx,
		Client:  c.state.client,
		Request: exchange.DownloadRequest{
			Destination: io.Discard,
			Target:      target,
			Semantics: exchange.RequestSemantics{
				Method: exchange.MethodGet,
				Replay: exchange.ReplaySingleAttempt,
			},
			Headers:        headers,
			CaptureHeaders: exchange.HeaderSelection{Names: []core.HTTPHeaderName{location}},
			ExpectedStatus: status,
		},
		Policy: exchange.StreamPolicy{
			Redirect: exchange.RedirectPolicy{Mode: exchange.RedirectObserve},
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
	digest := core.NewDigestWriter()
	destination := downloadDestination{destination: request.Destination, digest: digest}
	response, transferErr := exchange.Download(exchange.DownloadCall{
		Context: ctx,
		Client:  c.state.client,
		Request: exchange.DownloadRequest{
			Target: target, Destination: destination, Buffer: request.Buffer,
			Semantics: exchange.RequestSemantics{Method: exchange.MethodGet, Replay: exchange.ReplaySingleAttempt},
			Headers:   headers,

			ExpectedStatus: core.HTTPStatusOK(),
		},
		Policy: exchange.StreamPolicy{

			Redirect: exchange.RedirectPolicy{Mode: exchange.RedirectReject},
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

var _ io.Writer = downloadDestination{}
