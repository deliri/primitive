package ai_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/ai"
	"github.com/deliri/primitive/v2026/ai/chatgpt"
	"github.com/deliri/primitive/v2026/ai/claude"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/secretstore"
)

const testBackstop = 10 * time.Second

func TestRoutesExhaustClosedProviderOperationDomain(t *testing.T) {
	t.Parallel()
	for _, provider := range []ai.Provider{ai.ProviderUnknown, ai.OpenAI, ai.Anthropic, 255} {
		for _, operation := range []ai.Operation{ai.OperationUnknown, ai.Responses, ai.ChatCompletions, ai.AudioTranscriptions, ai.Messages, 255} {
			t.Run(fmt.Sprintf("provider-%d-operation-%d", provider, operation), func(t *testing.T) {
				t.Parallel()
				route := ai.Route{Provider: provider, Operation: operation}
				wantAdmitted := provider == ai.OpenAI && operation >= ai.Responses && operation <= ai.AudioTranscriptions || provider == ai.Anthropic && operation == ai.Messages
				endpoint, err := route.Endpoint()
				if !wantAdmitted {
					if !errors.Is(err, core.ErrAIBinding) || endpoint != (core.HTTPEndpoint{}) {
						t.Fatalf("Endpoint() = (%v, %v), want zero and binding rejection", endpoint, err)
					}
					return
				}
				wantURL := ""
				switch operation {
				case ai.Responses:
					wantURL = "https://api.openai.com/v1/responses"
				case ai.ChatCompletions:
					wantURL = "https://api.openai.com/v1/chat/completions"
				case ai.AudioTranscriptions:
					wantURL = "https://api.openai.com/v1/audio/transcriptions"
				case ai.Messages:
					wantURL = "https://api.anthropic.com/v1/messages"
				}
				if err != nil || endpoint.String() != wantURL {
					t.Fatalf("Endpoint() = (%v, %v), want %q and nil", endpoint, err, wantURL)
				}
			})
		}
	}
	if got := claude.Route(); got != (ai.Route{Provider: ai.Anthropic, Operation: ai.Messages}) {
		t.Fatalf("claude.Route() = %v, want Anthropic Messages", got)
	}
	if got, err := chatgpt.Route(ai.Messages); got != (ai.Route{}) || !errors.Is(err, core.ErrAIBinding) {
		t.Fatalf("chatgpt.Route(Messages) = (%v, %v), want zero and binding rejection", got, err)
	}
}

func TestCredentialCustodyAndProviderBindingLayerTriad(t *testing.T) {
	t.Parallel()
	for _, provider := range []ai.Provider{ai.OpenAI, ai.Anthropic} {
		t.Run(fmt.Sprintf("provider-%d-custody", provider), func(t *testing.T) {
			t.Parallel()
			material := []byte("sk-test-native-secret")
			credential, err := ai.ParseCredential(provider, material)
			if err != nil {
				t.Fatalf("ParseCredential() error = %v, want nil", err)
			}
			clear(material)
			if gotErr := credential.Validate(); gotErr != nil {
				t.Fatalf("Validate(after borrowed input cleared) error = %v, want nil", gotErr)
			}
			for _, format := range []string{"%v", "%+v", "%#v", "%s", "%q"} {
				if got := fmt.Sprintf(format, credential); strings.Contains(got, "sk-test-native-secret") || !strings.Contains(got, core.RedactedValueText) {
					t.Fatalf("credential formatting = %q, want redacted", got)
				}
			}
			client, clientErr := exchange.NewStandardClient()
			if clientErr != nil {
				t.Fatalf("NewStandardClient() error = %v, want nil", clientErr)
			}
			var native *ai.HTTPServer
			if provider == ai.OpenAI {
				native, err = chatgpt.NewServer(client, credential)
				foreign, foreignErr := claude.NewServer(client, credential)
				if foreign != nil || !errors.Is(foreignErr, core.ErrAIBinding) {
					t.Fatalf("claude.NewServer(OpenAI) = (%v, %v), want nil and binding refusal", foreign, foreignErr)
				}
			} else {
				native, err = claude.NewServer(client, credential)
				foreign, foreignErr := chatgpt.NewServer(client, credential)
				if foreign != nil || !errors.Is(foreignErr, core.ErrAIBinding) {
					t.Fatalf("chatgpt.NewServer(Anthropic) = (%v, %v), want nil and binding refusal", foreign, foreignErr)
				}
			}
			if err != nil {
				t.Fatalf("NewServer() error = %v, want nil", err)
			}
			if err := credential.Close(); err != nil {
				t.Fatalf("Credential.Close() error = %v, want nil", err)
			}
			if err := native.Validate(); err != nil {
				t.Fatalf("Server.Validate(after source key closed) error = %v, want nil", err)
			}
			if err := native.Close(); err != nil {
				t.Fatalf("Server.Close() error = %v, want nil", err)
			}
			if err := native.Validate(); !errors.Is(err, core.ErrAIAuthentication) {
				t.Fatalf("Server.Validate(after close) error = %v, want authentication refusal", err)
			}
			if err := client.Validate(); err != nil {
				t.Fatalf("borrowed transport.Validate() error = %v, want nil", err)
			}
		})
	}
	for _, material := range [][]byte{nil, []byte("\n"), []byte("a b"), []byte("=padding"), []byte("a=b"), {0xff}} {
		credential, err := ai.ParseCredential(ai.OpenAI, material)
		if !errors.Is(err, core.ErrAIAuthentication) || credential.Validate() == nil {
			t.Fatalf("ParseCredential(rejected) error = %v, want authentication rejection and zero capability", err)
		}
	}
}

func FuzzCredentialPublicDoorsSemanticCustody(f *testing.F) {
	seed, err := ai.ParseCredential(ai.OpenAI, []byte("sk-typed-valid-seed"))
	if err != nil {
		f.Fatalf("ParseCredential(seed) error = %v, want nil", err)
	}
	if err := seed.Close(); err != nil {
		f.Fatalf("seed.Close() error = %v, want nil", err)
	}
	f.Add([]byte("sk-typed-valid-seed"))
	f.Add([]byte{})
	f.Add([]byte("header\r\ninjection"))
	f.Fuzz(func(t *testing.T, material []byte) {
		for _, provider := range []ai.Provider{ai.OpenAI, ai.Anthropic, ai.ProviderUnknown} {
			credential, err := ai.ParseCredential(provider, material)
			name, nameErr := core.ParseHTTPHeaderName(core.AIAnthropicKeyHeader)
			if nameErr != nil {
				t.Fatalf("key header setup error = %v, want nil", nameErr)
			}
			wantAdmitted := provider == ai.OpenAI && (exchange.BearerAuthorization{Token: material}).Validate() == nil || provider == ai.Anthropic && (exchange.SecretHeader{Name: name, Value: material, Scheme: exchange.SecretHeaderLiteral}).Validate() == nil
			if !wantAdmitted {
				wantIdentity := core.ErrAIAuthentication
				if provider == ai.ProviderUnknown {
					wantIdentity = core.ErrAIContract
				}
				if !errors.Is(err, wantIdentity) || credential.Validate() == nil {
					t.Fatalf("rejected credential error = %v, want typed zero refusal", err)
				}
				continue
			}
			if err != nil || credential.Validate() != nil {
				t.Fatalf("accepted credential error = %v, want valid owned capability", err)
			}
			value, valueErr := secretstore.NewValue(material)
			if valueErr != nil {
				_ = credential.Close()
				continue
			}
			var fromSecret ai.Credential
			if provider == ai.OpenAI {
				fromSecret, err = chatgpt.CredentialFromSecret(value)
			} else {
				fromSecret, err = claude.CredentialFromSecret(value)
			}
			destroyErr := value.Destroy()
			gotProvider, providerErr := fromSecret.Provider()
			if err != nil || destroyErr != nil || providerErr != nil || gotProvider != provider {
				t.Fatalf("secret projection = (%d, %v, %v, %v), want owned provider %d", gotProvider, err, destroyErr, providerErr, provider)
			}
			if err := errors.Join(credential.Close(), fromSecret.Close()); err != nil {
				t.Fatalf("credential cleanup error = %v, want nil", err)
			}
		}
	})
}

type generatedSource struct {
	left    int64
	read    int64
	maximum int
}

func (r *generatedSource) Read(p []byte) (int, error) {
	if r.left == 0 {
		return 0, io.EOF
	}
	if int64(len(p)) > r.left {
		p = p[:r.left]
	}
	for i := range p {
		p[i] = byte((r.read + int64(i)) % 251)
	}
	r.left -= int64(len(p))
	r.read += int64(len(p))
	if len(p) > r.maximum {
		r.maximum = len(p)
	}
	return len(p), nil
}

type conservingSink struct {
	written int64
	maximum int
}

func (w *conservingSink) Write(p []byte) (int, error) {
	for i, value := range p {
		if value != byte((w.written+int64(i))%251) {
			return 0, core.ErrAIBinding
		}
	}
	w.written += int64(len(p))
	if len(p) > w.maximum {
		w.maximum = len(p)
	}
	return len(p), nil
}

type nativeFacts struct {
	received      int64
	method        string
	host          string
	path          string
	authorization string
	key           string
	version       string
	cause         error
}

func TestNativeHTTPStreamsConserveBytesAcrossWorkingWindows(t *testing.T) {
	t.Parallel()
	for _, route := range []ai.Route{{Provider: ai.OpenAI, Operation: ai.Responses}, {Provider: ai.OpenAI, Operation: ai.ChatCompletions}, {Provider: ai.OpenAI, Operation: ai.AudioTranscriptions}, claude.Route()} {
		for _, extent := range []int64{0, 32767, 32768, 32769, 8*32768 + 7, 4*1024*1024 + 1} {
			t.Run(fmt.Sprintf("provider-%d-operation-%d-extent-%d", route.Provider, route.Operation, extent), func(t *testing.T) {
				t.Parallel()
				facts := make(chan nativeFacts, 1)
				server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
					received := &conservingSink{}
					_, cause := io.CopyBuffer(received, request.Body, make([]byte, 32768))
					w.Header().Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeJSON().String())
					_, writeErr := io.CopyBuffer(w, &generatedSource{left: extent}, make([]byte, 32768))
					facts <- nativeFacts{received: received.written, method: request.Method, host: request.Host, path: request.URL.Path, authorization: request.Header.Get("Authorization"), key: request.Header.Get(core.AIAnthropicKeyHeader), version: request.Header.Get(core.AIAnthropicVersionHeader), cause: errors.Join(cause, writeErr)}
				}))
				defer server.Close()
				client := nativeClient(t, server, route.Provider)
				source := &generatedSource{left: extent}
				sink := &conservingSink{}
				request := streamingRequest(t, route, source, sink)
				result, err := client.RoundTrip(t.Context(), request, streamPolicy())
				var observation nativeFacts
				select {
				case observation = <-facts:
				case <-time.After(testBackstop):
					t.Fatal("native HTTP handler did not finish, want joined observation")
				}
				if err != nil || observation.cause != nil || source.read != extent || observation.received != extent || sink.written != extent || result.Stream.Metadata.Bytes.Uint64() != uint64(extent) || result.Stream.Metadata.Attempts != 1 {
					t.Fatalf("native stream = error:%v server:%v source:%d received:%d sink:%d metadata:%v, want %d conserved bytes and one attempt", err, observation.cause, source.read, observation.received, sink.written, result.Stream.Metadata, extent)
				}
				target, targetErr := route.Endpoint()
				if targetErr != nil || observation.method != http.MethodPost || observation.host != target.HTTPURL().Host || observation.path != target.HTTPURL().Path {
					t.Fatalf("native route = (%s,%s,%s), want exact POST authority/path", observation.method, observation.host, observation.path)
				}
				if route.Provider == ai.OpenAI {
					if observation.authorization != "Bearer sk-native-test" || observation.key != "" || observation.version != "" {
						t.Fatal("OpenAI headers differ, want only owned bearer authentication")
					}
				} else if observation.authorization != "" || observation.key != "sk-native-test" || observation.version != core.AIAnthropicVersion {
					t.Fatal("Claude headers differ, want owned key and documented version")
				}
				if sink.maximum > len(request.Stream.Buffer) || source.maximum > 32768 {
					t.Fatalf("working windows = source:%d sink:%d, want fixed windows while extent grows", source.maximum, sink.maximum)
				}
			})
		}
	}
}

func nativeClient(t testing.TB, server *httptest.Server, provider ai.Provider) ai.Client {
	t.Helper()
	transport := server.Client().Transport.(*http.Transport).Clone()
	// Connect to the native TLS fixture while exercising the documented SDK
	// authority/path. Verify the fixture's actual certificate, never a live key.
	host, _, addressErr := net.SplitHostPort(server.Listener.Addr().String())
	if addressErr != nil {
		t.Fatalf("native TLS address error = %v, want nil", addressErr)
	}
	transport.TLSClientConfig = transport.TLSClientConfig.Clone()
	transport.TLSClientConfig.ServerName = host
	transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
		return (&net.Dialer{}).DialContext(ctx, network, server.Listener.Addr().String())
	}
	t.Cleanup(transport.CloseIdleConnections)
	native, err := exchange.NewClient(&http.Client{Transport: transport})
	if err != nil {
		t.Fatalf("exchange.NewClient() error = %v, want nil", err)
	}
	credential, err := ai.ParseCredential(provider, []byte("sk-native-test"))
	if err != nil {
		t.Fatalf("ParseCredential() error = %v, want nil", err)
	}
	adapter, err := ai.NewHTTPServer(native, credential)
	if err != nil {
		t.Fatalf("NewHTTPServer() error = %v, want nil", err)
	}
	if err := credential.Close(); err != nil {
		t.Fatalf("Credential.Close() error = %v, want nil", err)
	}
	t.Cleanup(func() {
		if err := adapter.Close(); err != nil {
			t.Errorf("adapter.Close() error = %v, want nil", err)
		}
	})
	client, err := ai.NewClient(adapter)
	if err != nil {
		t.Fatalf("ai.NewClient() error = %v, want nil", err)
	}
	return client
}

func streamingRequest(t testing.TB, route ai.Route, source io.Reader, sink io.Writer) ai.Request {
	t.Helper()
	target, err := route.Endpoint()
	if err != nil {
		t.Fatalf("Route.Endpoint() error = %v, want nil", err)
	}
	media := core.HTTPMediaTypeJSON()
	if route.Operation == ai.AudioTranscriptions {
		media, err = core.ParseHTTPMediaType(core.AIMultipartMediaType + "; boundary=typed-native-boundary")
		if err != nil {
			t.Fatalf("ParseHTTPMediaType() error = %v, want nil", err)
		}
	}
	return ai.Request{Route: route, Stream: exchange.StreamRoundTripRequest{Target: target, Source: source, Destination: sink, Buffer: make([]byte, 32768), Semantics: exchange.RequestSemantics{Method: exchange.MethodPost, Replay: exchange.ReplaySingleAttempt}, RequestContentType: media, ExpectedResponseContentType: core.HTTPMediaTypeJSON(), ExpectedStatus: core.HTTPStatusOK()}}
}

func streamPolicy() exchange.StreamPolicy {
	return exchange.StreamPolicy{Redirect: exchange.RedirectPolicy{Mode: exchange.RedirectReject}}
}

type seamServer struct {
	calls    int
	err      error
	response ai.Response
}

func (*seamServer) Validate() error { return nil }
func (s *seamServer) RoundTrip(context.Context, ai.Request, exchange.StreamPolicy) (ai.Response, error) {
	s.calls++
	return s.response, s.err
}

func TestClientRefusesForeignRoutesBeforeServerExecutionLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		mutate  func(*ai.Request)
		wantErr error
	}{
		{name: "neutral missing source", mutate: func(r *ai.Request) { r.Stream.Source = nil }, wantErr: core.ErrAIContract},
		{name: "foreign host", mutate: func(r *ai.Request) {
			r.Stream.Target, _ = core.ParseHTTPEndpoint("https://foreign.example/v1/responses")
		}, wantErr: core.ErrAIBinding},
		{name: "foreign operation path", mutate: func(r *ai.Request) {
			r.Stream.Target, _ = core.ParseHTTPEndpoint("https://api.openai.com/v1/chat/completions")
		}, wantErr: core.ErrAIBinding},
		{name: "query injected", mutate: func(r *ai.Request) {
			r.Stream.Target, _ = core.ParseHTTPEndpoint("https://api.openai.com/v1/responses?forward=secret")
		}, wantErr: core.ErrAIBinding},
		{name: "method changed", mutate: func(r *ai.Request) { r.Stream.Semantics.Method = exchange.MethodGet }, wantErr: core.ErrAIBinding},
		{name: "provider operation contradiction", mutate: func(r *ai.Request) { r.Route.Provider = ai.Anthropic }, wantErr: core.ErrAIBinding},
		{name: "missing destination", mutate: func(r *ai.Request) { r.Stream.Destination = nil }, wantErr: core.ErrAIContract},
		{name: "wrong request media", mutate: func(r *ai.Request) { r.Stream.RequestContentType = core.HTTPMediaTypeOctetStream() }, wantErr: core.ErrAIBinding},
		{name: "caller credential injection", mutate: func(r *ai.Request) {
			h, _ := exchange.NewBearerAuthorizationHeader(exchange.BearerAuthorization{Token: []byte("foreign")})
			r.Stream.Headers = exchange.Headers{Values: []exchange.Header{h}}
		}, wantErr: core.ErrAIBinding},
		{name: "unkeyed replay amplification", mutate: func(r *ai.Request) { r.Stream.Semantics.Replay = exchange.ReplayIdempotent }, wantErr: core.ErrAIContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			server := &seamServer{}
			client, err := ai.NewClient(server)
			if err != nil {
				t.Fatalf("NewClient() error = %v, want nil", err)
			}
			request := streamingRequest(t, ai.Route{Provider: ai.OpenAI, Operation: ai.Responses}, bytes.NewReader([]byte{1}), io.Discard)
			tc.mutate(&request)
			got, err := client.RoundTrip(t.Context(), request, streamPolicy())
			if !errors.Is(err, tc.wantErr) || server.calls != 0 || got.Stream.Metadata.Attempts != 0 {
				t.Fatalf("RoundTrip(refused) = error:%v calls:%d attempts:%d, want %v and zero effects", err, server.calls, got.Stream.Metadata.Attempts, tc.wantErr)
			}
		})
	}
	server := &seamServer{err: io.ErrUnexpectedEOF}
	client, err := ai.NewClient(server)
	if err != nil {
		t.Fatalf("NewClient() error = %v, want nil", err)
	}
	request := streamingRequest(t, claude.Route(), bytes.NewReader([]byte{1}), io.Discard)
	_, err = client.RoundTrip(t.Context(), request, streamPolicy())
	if !errors.Is(err, io.ErrUnexpectedEOF) || server.calls != 1 {
		t.Fatalf("replaceable server = error:%v calls:%d, want native cause and exactly one delegation", err, server.calls)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err = client.RoundTrip(ctx, request, streamPolicy())
	if !errors.Is(err, context.Canceled) || server.calls != 1 {
		t.Fatalf("cancelled call = error:%v calls:%d, want native cancellation and no further delegation", err, server.calls)
	}
}

func FuzzRouteAndTargetBindingSemanticClosure(f *testing.F) {
	for _, route := range []ai.Route{{Provider: ai.OpenAI, Operation: ai.Responses}, {Provider: ai.OpenAI, Operation: ai.ChatCompletions}, {Provider: ai.OpenAI, Operation: ai.AudioTranscriptions}, claude.Route()} {
		endpoint, err := route.Endpoint()
		if err != nil {
			f.Fatalf("Route.Endpoint(seed) error = %v, want nil", err)
		}
		f.Add(uint8(route.Provider), uint8(route.Operation), endpoint.String())
	}
	f.Fuzz(func(t *testing.T, provider, operation uint8, targetText string) {
		route := ai.Route{Provider: ai.Provider(provider), Operation: ai.Operation(operation)}
		endpoint, err := route.Endpoint()
		if err != nil {
			if !errors.Is(err, core.ErrAIBinding) || endpoint != (core.HTTPEndpoint{}) {
				t.Fatalf("route refusal = (%v, %v), want zero and typed binding rejection", endpoint, err)
			}
			return
		}
		request := streamingRequest(t, route, bytes.NewReader(nil), io.Discard)
		foreign, parseErr := core.ParseHTTPEndpoint(targetText)
		if parseErr != nil {
			return
		}
		request.Stream.Target = foreign
		gotErr := request.Validate()
		wantAdmitted := foreign.HTTPURL() == endpoint.HTTPURL()
		if wantAdmitted != (gotErr == nil) {
			t.Fatalf("request target admission = %v, want exact authority/path equality %t", gotErr, wantAdmitted)
		}
		if gotErr != nil && !errors.Is(gotErr, core.ErrAIBinding) {
			t.Fatalf("foreign target error = %v, want binding rejection", gotErr)
		}
	})
}
