package ai

import (
	"context"
	"fmt"
	"io"
	"sync"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

// HTTPServer projects a provider credential into one native Exchange call.
// It borrows the transport and owns only its independently cleared key copy.
type HTTPServer struct {
	mu         sync.RWMutex
	client     exchange.Client
	credential Credential
}

// NewHTTPServer borrows a validated Exchange transport and copies credential
// material into its own independently clearable bytes. It starts no workers.
func NewHTTPServer(client exchange.Client, credential Credential) (*HTTPServer, error) {
	if err := client.Validate(); err != nil {
		return nil, contractError(err)
	}
	if err := credential.Validate(); err != nil {
		return nil, err
	}
	owned, err := ParseCredential(credential.provider, credential.value)
	if err != nil {
		return nil, err
	}
	return &HTTPServer{client: client, credential: owned}, nil
}

// Validate checks transport and credential authority under the custody lock.
func (s *HTTPServer) Validate() error {
	if s == nil {
		return core.ErrAIContract
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.validateUnlocked()
}

func (s *HTTPServer) validateUnlocked() error {
	if s == nil {
		return core.ErrAIContract
	}
	if err := s.client.Validate(); err != nil {
		return contractError(err)
	}
	return s.credential.Validate()
}

// Format prevents recursive diagnostics from projecting owned secret bytes.
func (*HTTPServer) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

// Close waits for every in-flight RoundTrip, then clears owned credential bytes.
// Callers cancel or release blocked borrowed readers/writers before joining;
// a reader or writer must not call Close on its own active server call.
// The native lock owns this ordering: https://pkg.go.dev/sync#RWMutex
func (s *HTTPServer) Close() error {
	if s == nil {
		return core.ErrAIContract
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.credential.Close()
}

// RoundTrip retains credential custody through one synchronous Exchange call.
// Response and provider-error bodies stream to the caller's distinct sinks;
// request-id and retry-after are captured without enabling implicit retries.
// Sources: https://developers.openai.com/api/reference/overview
// https://platform.claude.com/docs/en/api/overview#response-headers
func (s *HTTPServer) RoundTrip(ctx context.Context, request Request, policy exchange.StreamPolicy) (Response, error) {
	if s == nil {
		return Response{}, core.ErrAIContract
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if err := contextstate.Validate(ctx); err != nil {
		return Response{}, err
	}
	if err := s.validateUnlocked(); err != nil {
		return Response{}, err
	}
	if err := request.Validate(); err != nil {
		return Response{}, err
	}
	if err := validatePolicy(policy); err != nil {
		return Response{}, err
	}
	if request.Route.Provider != s.credential.provider {
		return Response{}, core.ErrAIBinding
	}
	headers, err := optionalHeaders(request)
	if err != nil {
		return Response{}, err
	}
	stream := request.Stream
	stream.Headers = headers
	secret, err := s.secretHeader()
	if err != nil {
		return Response{}, err
	}
	stream.SecretHeaders = []exchange.SecretHeader{secret}
	if request.Route.Provider == Anthropic {
		name, parseErr := core.ParseHTTPHeaderName(core.AIAnthropicVersionHeader)
		if parseErr != nil {
			return Response{}, contractError(parseErr)
		}
		version, versionErr := exchange.NewHeaderValue(request.Version.String())
		if versionErr != nil {
			return Response{}, contractError(versionErr)
		}
		stream.Headers.Values = append(stream.Headers.Values, exchange.Header{Name: name, Values: []exchange.HeaderValue{version}})
	}
	stream.CaptureHeaders, err = responseHeaders(request)
	if err != nil {
		return Response{}, err
	}
	response, err := exchange.RoundTripStream(exchange.StreamRoundTripCall{Context: ctx, Client: s.client, Request: stream, Policy: policy})
	result := Response{Stream: response}
	if err != nil {
		return result, providerError(err)
	}
	return result, result.Validate()
}

func (s *HTTPServer) secretHeader() (exchange.SecretHeader, error) {
	switch s.credential.provider {
	case OpenAI:
		name, err := exchange.StandardHeaderAuthorization.Name()
		if err != nil {
			return exchange.SecretHeader{}, contractError(err)
		}
		return exchange.SecretHeader{Name: name, Value: s.credential.value, Scheme: exchange.SecretHeaderBearer}, nil
	case Anthropic:
		name, err := core.ParseHTTPHeaderName(core.AIAnthropicKeyHeader)
		if err != nil {
			return exchange.SecretHeader{}, contractError(err)
		}
		return exchange.SecretHeader{Name: name, Value: s.credential.value, Scheme: exchange.SecretHeaderLiteral}, nil
	default:
		return exchange.SecretHeader{}, core.ErrAIAuthentication
	}
}

func responseHeaders(request Request) (exchange.HeaderSelection, error) {
	names := append([]core.HTTPHeaderName(nil), request.Stream.CaptureHeaders.Names...)
	retry, err := exchange.StandardHeaderRetryAfter.Name()
	if err != nil {
		return exchange.HeaderSelection{}, contractError(err)
	}
	requestID, err := core.ParseHTTPHeaderName(core.AIOpenAIRequestIDHeader)
	if request.Route.Provider == Anthropic {
		requestID, err = core.ParseHTTPHeaderName(core.AIAnthropicRequestIDHeader)
	}
	if err != nil {
		return exchange.HeaderSelection{}, contractError(err)
	}
	for _, name := range []core.HTTPHeaderName{retry, requestID} {
		found := false
		for _, prior := range names {
			if name == prior {
				found = true
				break
			}
		}
		if !found {
			names = append(names, name)
		}
	}
	selected := exchange.HeaderSelection{Names: names}
	if err := selected.Validate(); err != nil {
		return exchange.HeaderSelection{}, contractError(err)
	}
	return selected, nil
}
