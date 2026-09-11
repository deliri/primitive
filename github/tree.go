package github

import (
	"context"
	"errors"
	"io"
	"net/url"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

type treeDecodeState struct {
	seenSHA       bool
	seenURL       bool
	seenTree      bool
	seenTruncated bool
	truncated     bool
	entries       uint64
}

type treeDownloadResult struct {
	err      error
	response exchange.StreamResponse
}

type treeDownloadCall struct {
	ctx       context.Context
	client    exchange.Client
	writer    *io.PipeWriter
	completed chan<- treeDownloadResult
	media     core.HTTPMediaType
	headers   exchange.Headers
	target    core.HTTPEndpoint
	policy    exchange.StreamPolicy
}

// ReadTree streams one recursive Git tree through the caller-owned visitor.
// Primitive retains no repository-wide collection. The visitor is synchronous
// backpressure and must return; the caller owns any blocking work it performs.
func (c Client) ReadTree(ctx context.Context, request TreeRequest) (TreeObservation, error) {
	if ctx == nil {
		return TreeObservation{}, core.ErrGitHubContract
	}
	if err := errors.Join(c.Validate(), request.Validate()); err != nil {
		return TreeObservation{}, contractError(err)
	}
	headers, _, err := c.headers(ctx, false)
	if err != nil {
		return TreeObservation{}, err
	}
	target, err := c.target(repositoryPath(request.Repository)+"/git/trees/"+request.Commit.String(), url.Values{"recursive": []string{"1"}})
	if err != nil {
		return TreeObservation{}, err
	}
	media, err := exchange.StandardMediaTypeJSON.HTTPMediaType()
	if err != nil {
		return TreeObservation{}, contractError(err)
	}
	downloadContext, cancelDownload := context.WithCancel(ctx)
	defer cancelDownload()
	reader, writer := io.Pipe()
	completed := make(chan treeDownloadResult, 1)
	go downloadTree(treeDownloadCall{
		ctx: downloadContext, client: c.state.client, target: target, headers: headers, media: media,
		policy: exchange.StreamPolicy{Redirect: exchange.RedirectPolicy{Mode: exchange.RedirectReject}}, writer: writer, completed: completed,
	})
	joined := false
	defer func() {
		cancelDownload()
		// Pipe close releases a producer blocked in Write, including during panic.
		if err := reader.Close(); err != nil {
			panic(err) // io.PipeReader.Close has no failure mode.
		}
		if !joined {
			<-completed
		}
	}()
	entries, decodeErr := decodeTree(reader, request.Visitor)
	if decodeErr != nil {
		cancelDownload()
		_ = reader.CloseWithError(decodeErr)
	} else {
		decodeErr = reader.Close()
	}
	download := <-completed
	joined = true
	if err := errors.Join(decodeErr, download.err); err != nil {
		return TreeObservation{}, err
	}
	result := TreeObservation{
		Repository: request.Repository, Commit: request.Commit,
		Entries: entries, Bytes: download.response.Metadata.Bytes,
	}
	return result, result.Validate()
}

func downloadTree(call treeDownloadCall) {
	response, err := exchange.Download(exchange.DownloadCall{
		Context: call.ctx,
		Client:  call.client,
		Request: exchange.DownloadRequest{
			Target: call.target, Destination: call.writer,
			Semantics:                   exchange.RequestSemantics{Method: exchange.MethodGet, Replay: exchange.ReplaySingleAttempt},
			ExpectedResponseContentType: call.media, Headers: call.headers, ExpectedStatus: core.HTTPStatusOK(),
		},
		Policy: call.policy,
	})
	closeErr := call.writer.CloseWithError(err)
	call.completed <- treeDownloadResult{response: response, err: errors.Join(classifyExchangeError(err), closeErr)}
}
