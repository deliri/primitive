package github

import (
	"context"
	"encoding/base64"
	json "encoding/json/v2"
	"errors"
	"math"
	"net/url"
	"strconv"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

const (
	githubLinkNextRelation = `rel="next"`
	// GitHub documents Link targets using its canonical numeric repository
	// resource path even when the initiating request used owner/name.
	// Source: https://docs.github.com/en/rest/using-the-rest-api/using-pagination-in-the-rest-api
	githubRepositoryResourcePathPrefix = "/repositories/"
	githubTagsPathSuffix               = "/tags"
)

type tagCommitWire struct {
	SHA string `json:"sha"`
	URL string `json:"url"`
}

type tagWire struct {
	Name       string        `json:"name"`
	Commit     tagCommitWire `json:"commit"`
	ZipballURL string        `json:"zipball_url"`
	TarballURL string        `json:"tarball_url"`
	NodeID     string        `json:"node_id"`
}

type headWire struct {
	SHA string `json:"sha"`
}

type contentsLinksWire struct {
	Self string `json:"self"`
	Git  string `json:"git"`
	HTML string `json:"html"`
}

type contentsWire struct {
	Links       contentsLinksWire `json:"_links"`
	Name        string            `json:"name"`
	Path        string            `json:"path"`
	SHA         string            `json:"sha"`
	URL         string            `json:"url"`
	HTMLURL     string            `json:"html_url"`
	GitURL      string            `json:"git_url"`
	DownloadURL string            `json:"download_url"`
	Type        string            `json:"type"`
	Content     string            `json:"content"`
	Encoding    string            `json:"encoding"`
	Size        uint64            `json:"size"`
}

type boundedRequest struct {
	ctx         context.Context
	target      core.HTTPEndpoint
	maximum     uint64
	status      core.HTTPStatusCode
	method      exchange.Method
	captureLink bool
}

// ReadTagPage retrieves one provider-bounded page without deciding which tags
// are releases or which version is newest.
func (c Client) ReadTagPage(ctx context.Context, request TagPageRequest) (TagPage, error) {
	if err := errors.Join(c.Validate(), request.Validate()); err != nil {
		return TagPage{}, contractError(err)
	}
	target, err := c.target(repositoryPath(request.Repository)+"/tags", queryPage(request.Page))
	if err != nil {
		return TagPage{}, err
	}
	response, err := c.sendBounded(boundedRequest{
		ctx: ctx, target: target, method: exchange.MethodGet, status: core.HTTPStatusOK(),
		maximum: core.GitHubTagPageResponseCustodyMaximumBytes, captureLink: true,
	})
	if err != nil {
		return TagPage{}, err
	}
	tags, err := decodeTagPage(response.Body)
	if err != nil {
		return TagPage{}, err
	}
	next, err := c.nextTagPage(response.Metadata.Headers, request)
	if err != nil {
		return TagPage{}, err
	}
	page := TagPage{Repository: request.Repository, Page: request.Page, NextPage: next, Tags: tags}
	return page, page.Validate()
}

func decodeTagPage(payload []byte) ([]Tag, error) {
	var wire []tagWire
	if err := json.Unmarshal(payload, &wire, json.RejectUnknownMembers(true)); err != nil {
		return nil, responseError(err)
	}
	if len(wire) > core.GitHubTagPageMaximumEntries {
		return nil, core.ErrGitHubResponse
	}
	tags := make([]Tag, len(wire))
	for index := range wire {
		name, nameErr := ParseReference(wire[index].Name)
		commit, commitErr := core.ParseBuildCommit(wire[index].Commit.SHA)
		if err := errors.Join(nameErr, commitErr); err != nil {
			return nil, responseError(err)
		}
		tags[index] = Tag{Name: name, Commit: commit}
	}
	return tags, nil
}

func (c Client) nextTagPage(headers exchange.CapturedHeaders, request TagPageRequest) (uint32, error) {
	for _, header := range headers.Values {
		if header.Name.String() != headerLink {
			continue
		}
		if len(header.Values) != 1 || request.Page == math.MaxUint32 {
			return 0, core.ErrGitHubResponse
		}
		value, err := header.Values[0].Value()
		if err != nil {
			return 0, responseError(err)
		}
		next := request.Page + 1
		for entry := range strings.SplitSeq(value, ",") {
			candidate := strings.TrimSpace(entry)
			if !strings.Contains(candidate, githubLinkNextRelation) {
				continue
			}
			if err := c.validateNextTagLink(candidate, request.Repository, next); err != nil {
				return 0, err
			}
			return next, nil
		}
		return 0, nil
	}
	return 0, nil
}

func (c Client) validateNextTagLink(candidate string, repository Repository, next uint32) error {
	const suffix = `>; rel="next"`
	if !strings.HasPrefix(candidate, "<") || !strings.HasSuffix(candidate, suffix) {
		return core.ErrGitHubBinding
	}
	parsed, err := url.Parse(strings.TrimSuffix(strings.TrimPrefix(candidate, "<"), suffix))
	if err != nil {
		return bindingError(err)
	}
	authority := c.state.authority.HTTPURL()
	if parsed.Scheme != authority.Scheme || parsed.Host != authority.Host || parsed.User != nil || parsed.Fragment != "" {
		return core.ErrGitHubBinding
	}
	query, err := url.ParseQuery(parsed.RawQuery)
	if err != nil || query.Encode() != queryPage(next).Encode() {
		return core.ErrGitHubBinding
	}
	expectedPath := repositoryPath(repository) + githubTagsPathSuffix
	if parsed.Path == expectedPath || validGitHubRepositoryResourceTagsPath(parsed.Path) {
		return nil
	}
	return core.ErrGitHubBinding
}

func validGitHubRepositoryResourceTagsPath(path string) bool {
	resource, found := strings.CutPrefix(path, githubRepositoryResourcePathPrefix)
	if !found {
		return false
	}
	identifier, found := strings.CutSuffix(resource, githubTagsPathSuffix)
	if !found || identifier == "" || strings.Contains(identifier, "/") {
		return false
	}
	value, err := strconv.ParseUint(identifier, 10, 64)
	return err == nil && value != 0
}

// ReadHead observes the first commit returned by GitHub's repository commit
// listing. It does not attach product release or completion meaning.
func (c Client) ReadHead(ctx context.Context, request HeadRequest) (HeadObservation, error) {
	if err := errors.Join(c.Validate(), request.Validate()); err != nil {
		return HeadObservation{}, contractError(err)
	}
	target, err := c.target(repositoryPath(request.Repository)+"/commits", url.Values{"per_page": []string{"1"}})
	if err != nil {
		return HeadObservation{}, err
	}
	response, err := c.sendBounded(boundedRequest{
		ctx: ctx, target: target, method: exchange.MethodGet, status: core.HTTPStatusOK(),
		maximum: core.GitHubCommitResponseCustodyMaximumBytes,
	})
	if err != nil {
		return HeadObservation{}, err
	}
	commit, err := decodeHead(response.Body)
	if err != nil {
		return HeadObservation{}, err
	}
	result := HeadObservation{Repository: request.Repository, Commit: commit}
	return result, result.Validate()
}

func decodeHead(payload []byte) (core.BuildCommit, error) {
	var wire []headWire
	if err := json.Unmarshal(payload, &wire); err != nil || len(wire) != 1 {
		return core.BuildCommit{}, responseError(err)
	}
	commit, err := core.ParseBuildCommit(wire[0].SHA)
	if err != nil {
		return core.BuildCommit{}, responseError(err)
	}
	return commit, nil
}

// ReadFile retrieves one bounded inline contents object at one immutable commit.
func (c Client) ReadFile(ctx context.Context, request FileRequest) (FileObservation, error) {
	if err := errors.Join(c.Validate(), request.Validate()); err != nil {
		return FileObservation{}, contractError(err)
	}
	target, err := c.target(repositoryPath(request.Repository)+"/contents/"+sourcePath(request.Path), url.Values{"ref": []string{request.Commit.String()}})
	if err != nil {
		return FileObservation{}, err
	}
	response, err := c.sendBounded(boundedRequest{
		ctx: ctx, target: target, method: exchange.MethodGet, status: core.HTTPStatusOK(),
		maximum: core.GitHubContentsResponseCustodyMaximumBytes,
	})
	if err != nil {
		return FileObservation{}, err
	}
	return fileObservation(request, response.Body)
}

func fileObservation(request FileRequest, payload []byte) (FileObservation, error) {
	var wire contentsWire
	if err := json.Unmarshal(payload, &wire, json.RejectUnknownMembers(true)); err != nil {
		return FileObservation{}, responseError(err)
	}
	maximum, err := request.MaximumBytes.Uint64()
	if err != nil || wire.Type != "file" || wire.Encoding != "base64" || wire.Path != request.Path.String() || wire.Size > maximum {
		return FileObservation{}, responseError(err)
	}
	encoded := strings.ReplaceAll(wire.Content, "\n", "")
	content, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || uint64(len(content)) != wire.Size {
		return FileObservation{}, responseError(err)
	}
	length, err := core.NewByteLength(uint64(len(content)))
	if err != nil {
		return FileObservation{}, responseError(err)
	}
	result := FileObservation{
		Repository: request.Repository, Commit: request.Commit, Path: request.Path,
		Length: length, SHA256: core.SHA256Of(content), Content: content,
	}
	return result, result.Validate()
}

func (c Client) sendBounded(request boundedRequest) (exchange.BoundedResponse, error) {
	headers, capture, err := c.headers(request.ctx, request.captureLink)
	if err != nil {
		return exchange.BoundedResponse{}, err
	}
	policy, policyErr := boundedPolicy(request.maximum)
	media, mediaErr := exchange.StandardMediaTypeJSON.HTTPMediaType()
	if err := errors.Join(policyErr, mediaErr); err != nil {
		return exchange.BoundedResponse{}, err
	}
	response, err := exchange.SendNoBodyBounded(exchange.NoBodyBoundedCall{
		Context: request.ctx,
		Client:  c.state.client,
		Request: exchange.NoBodyBoundedRequest{
			Target: request.target, Semantics: exchange.RequestSemantics{Method: request.method, Replay: exchange.ReplaySingleAttempt},
			ExpectedResponseContentType: media, Headers: headers, CaptureHeaders: capture, ExpectedStatus: request.status,
		},
		Policy: policy,
	})
	if err != nil {
		return response, classifyExchangeError(err)
	}
	return response, response.Validate()
}

func classifyExchangeError(err error) error {
	if status, ok := errors.AsType[exchange.StatusError](err); ok {
		observed, observedErr := status.Status().Int()
		if observedErr == nil && observed == 404 {
			return errors.Join(core.ErrGitHubNotFound, err)
		}
	}
	return err
}
