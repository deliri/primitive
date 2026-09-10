package github

import (
	"context"
	"errors"
	"net/url"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

// ReadFile streams GitHub's raw contents representation at an immutable commit.
// Exchange validates status and media type before releasing bytes. In particular,
// a directory's JSON listing cannot become file content. Go owns the copy loop.
// GitHub may resolve a symlink to its target under this documented representation.
func (c Client) ReadFile(ctx context.Context, request FileRequest) (FileObservation, error) {
	if err := errors.Join(c.Validate(), request.Validate()); err != nil {
		return FileObservation{}, contractError(err)
	}
	target, err := c.target(repositoryPath(request.Repository)+"/contents/"+request.Path.String(), url.Values{"ref": []string{request.Commit.String()}})
	if err != nil {
		return FileObservation{}, err
	}
	headers, _, err := c.headers(ctx, false)
	if err != nil {
		return FileObservation{}, err
	}
	media, mediaErr := core.ParseHTTPMediaType(core.GitHubRawContentMediaType)
	if err := mediaErr; err != nil {
		return FileObservation{}, contractError(err)
	}
	digest := core.NewDigestWriter()
	response, transferErr := exchange.Download(exchange.DownloadCall{
		Context: ctx, Client: c.state.client, Policy: exchange.StreamPolicy{Redirect: exchange.RedirectPolicy{Mode: exchange.RedirectReject}},
		Request: exchange.DownloadRequest{
			Target: target, Destination: downloadDestination{destination: request.Destination, digest: digest}, Buffer: request.Buffer,
			Semantics: exchange.RequestSemantics{Method: exchange.MethodGet, Replay: exchange.ReplaySingleAttempt},
			Headers:   headers, ExpectedStatus: core.HTTPStatusOK(), ExpectedResponseContentType: media,
		},
	})
	sha256, length, sealErr := digest.Seal()
	observation := FileObservation{Repository: request.Repository, Commit: request.Commit, Path: request.Path, Length: length, SHA256: sha256}
	if transferErr == nil && response.Metadata.Bytes != length {
		transferErr = core.ErrGitHubResponse
	}
	return observation, errors.Join(classifyExchangeError(transferErr), sealErr, observation.Validate())
}
