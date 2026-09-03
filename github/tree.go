package github

import (
	"context"
	"encoding/json/jsontext"
	json "encoding/json/v2"
	"errors"
	"io"
	"net/url"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

type treeEntryWire struct {
	Path string `json:"path"`
	Mode string `json:"mode"`
	Type string `json:"type"`
	SHA  string `json:"sha"`
	URL  string `json:"url"`
	Size uint64 `json:"size,omitzero"`
}

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
	limit     core.ByteCount
}

type treeDecoder struct {
	visitor TreeVisitor
	decoder *jsontext.Decoder
	state   treeDecodeState
	maximum uint64
}

// ReadTree streams one recursive Git tree through the caller-owned visitor.
// Primitive retains no repository-wide collection.
func (c Client) ReadTree(ctx context.Context, request TreeRequest) (TreeObservation, error) {
	if err := errors.Join(c.Validate(), request.Validate()); err != nil {
		return TreeObservation{}, contractError(err)
	}
	headers, _, err := c.headers(ctx, false)
	if err != nil {
		return TreeObservation{}, err
	}
	target, err := c.target(repositoryPath(request.Repository)+"/git/trees/"+url.PathEscape(request.Commit.String()), url.Values{"recursive": []string{"1"}})
	if err != nil {
		return TreeObservation{}, err
	}
	policy, limit, err := streamPolicy()
	if err != nil {
		return TreeObservation{}, err
	}
	media, err := exchange.StandardMediaTypeJSON.HTTPMediaType()
	if err != nil {
		return TreeObservation{}, contractError(err)
	}
	reader, writer := io.Pipe()
	completed := make(chan treeDownloadResult, 1)
	go downloadTree(treeDownloadCall{
		ctx: ctx, client: c.state.client, target: target, headers: headers, media: media,
		policy: policy, limit: limit, writer: writer, completed: completed,
	})
	entries, decodeErr := decodeTree(reader, request.MaximumEntries, request.Visitor)
	if decodeErr != nil {
		_ = reader.CloseWithError(decodeErr)
	} else {
		decodeErr = reader.Close()
	}
	download := <-completed
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
			ExpectedResponseContentType: call.media, Headers: call.headers, ResponseBodyLimit: call.limit, ExpectedStatus: core.HTTPStatusOK(),
		},
		Policy: call.policy,
	})
	closeErr := call.writer.CloseWithError(err)
	call.completed <- treeDownloadResult{response: response, err: errors.Join(classifyExchangeError(err), closeErr)}
}

func decodeTree(source io.Reader, maximum uint64, visitor TreeVisitor) (uint64, error) {
	decoding := treeDecoder{decoder: jsontext.NewDecoder(source), maximum: maximum, visitor: visitor}
	if err := requireToken(decoding.decoder, jsontext.KindBeginObject); err != nil {
		return 0, err
	}
	if err := decoding.decodeObject(); err != nil {
		return 0, err
	}
	return decoding.finish()
}

func (d *treeDecoder) decodeObject() error {
	for d.decoder.PeekKind() != jsontext.KindEndObject {
		name, err := d.decoder.ReadToken()
		if err != nil || name.Kind() != jsontext.KindString {
			return responseError(err)
		}
		if err := d.decodeMember(name.String()); err != nil {
			return err
		}
	}
	return requireToken(d.decoder, jsontext.KindEndObject)
}

func (d *treeDecoder) finish() (uint64, error) {
	if _, err := d.decoder.ReadToken(); !errors.Is(err, io.EOF) {
		return 0, responseError(err)
	}
	if !d.state.seenSHA || !d.state.seenURL || !d.state.seenTree || !d.state.seenTruncated || d.state.truncated {
		return 0, core.ErrGitHubResponse
	}
	return d.state.entries, nil
}

func (d *treeDecoder) decodeMember(name string) error {
	switch name {
	case "sha":
		return decodeIgnoredString(d.decoder, &d.state.seenSHA)
	case "url":
		return decodeIgnoredString(d.decoder, &d.state.seenURL)
	case "tree":
		if d.state.seenTree {
			return core.ErrGitHubResponse
		}
		d.state.seenTree = true
		return d.decodeEntries()
	case "truncated":
		if d.state.seenTruncated {
			return core.ErrGitHubResponse
		}
		d.state.seenTruncated = true
		if err := json.UnmarshalDecode(d.decoder, &d.state.truncated); err != nil {
			return responseError(err)
		}
		return nil
	default:
		return core.ErrGitHubResponse
	}
}

func decodeIgnoredString(decoder *jsontext.Decoder, seen *bool) error {
	if *seen {
		return core.ErrGitHubResponse
	}
	*seen = true
	var value string
	if err := json.UnmarshalDecode(decoder, &value); err != nil || value == "" {
		return responseError(err)
	}
	return nil
}

func (d *treeDecoder) decodeEntries() error {
	if err := requireToken(d.decoder, jsontext.KindBeginArray); err != nil {
		return err
	}
	for d.decoder.PeekKind() != jsontext.KindEndArray {
		if d.state.entries == d.maximum || d.state.entries == core.GitHubRecursiveTreeMaximumEntries {
			return core.ErrGitHubResponse
		}
		var wire treeEntryWire
		if err := json.UnmarshalDecode(d.decoder, &wire, json.RejectUnknownMembers(true)); err != nil {
			return responseError(err)
		}
		entry, err := projectTreeEntry(wire)
		if err != nil {
			return err
		}
		if err := d.visitor.VisitGitHubTreeEntry(entry); err != nil {
			return err
		}
		d.state.entries++
	}
	return requireToken(d.decoder, jsontext.KindEndArray)
}

func projectTreeEntry(wire treeEntryWire) (TreeEntry, error) {
	path, err := core.ParseSourcePath(wire.Path)
	if err != nil {
		return TreeEntry{}, responseError(err)
	}
	var kind TreeEntryKind
	switch wire.Type {
	case "blob":
		kind = TreeEntryBlob
	case "tree":
		kind = TreeEntryDirectory
	case "commit":
		kind = TreeEntrySubmodule
	default:
		return TreeEntry{}, core.ErrGitHubResponse
	}
	entry := TreeEntry{Path: path, Kind: kind}
	return entry, entry.Validate()
}

func requireToken(decoder *jsontext.Decoder, kind jsontext.Kind) error {
	token, err := decoder.ReadToken()
	if err != nil || token.Kind() != kind {
		return responseError(err)
	}
	return nil
}
