package ai_test

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deliri/primitive/v2026/ai"
	"github.com/deliri/primitive/v2026/ai/claude"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

// This target proves the SDK request-validation seam, not network behavior.
// TLS execution and native stream ownership have separate native tests.
func FuzzOptionalProviderRequestSemanticBinding(f *testing.F) {
	for _, seed := range []struct {
		provider ai.Provider
		kind     ai.OptionalHeaderName
		version  ai.APIVersion
		value    string
	}{
		{provider: ai.OpenAI, kind: ai.OpenAIOrganization, value: "org-owned"},
		{provider: ai.OpenAI, kind: ai.OpenAIProject, value: "proj-owned"},
		{provider: ai.Anthropic, kind: ai.AnthropicWorkspace, version: ai.Anthropic20230601, value: "wrkspc_owned"},
		{provider: ai.Anthropic, kind: ai.AnthropicBeta, value: "structured-outputs-2025-11-13"},
	} {
		route := ai.Route{Provider: seed.provider, Operation: ai.Responses}
		if seed.provider == ai.Anthropic {
			route = claude.Route()
		}
		request := streamingRequest(f, route, bytes.NewReader(nil), io.Discard)
		value, err := exchange.NewHeaderValue(seed.value)
		if err != nil {
			f.Fatalf("optional header seed = %v, want admitted", err)
		}
		request.Options = []ai.OptionalHeader{{Name: seed.kind, Value: value}}
		request.Version = seed.version
		if err := request.Validate(); err != nil {
			f.Fatalf("typed request seed = %v, want admitted", err)
		}
		wire, err := value.Value()
		if err != nil {
			f.Fatalf("typed field seed projection = %v, want admitted", err)
		}
		f.Add(uint8(seed.provider), uint8(seed.kind), uint8(seed.version), wire, false)
	}
	f.Add(uint8(ai.OpenAI), uint8(ai.AnthropicWorkspace), uint8(ai.APIVersionDefault), "foreign", false)
	f.Add(uint8(ai.Anthropic), uint8(ai.OpenAIProject), uint8(ai.APIVersionDefault), "foreign", false)
	f.Add(uint8(ai.OpenAI), uint8(ai.OpenAIProject), uint8(ai.Anthropic20230601), "proj-owned", false)
	f.Add(uint8(ai.Anthropic), uint8(ai.AnthropicBeta), uint8(ai.APIVersionDefault), "header\r\ninjection", false)
	f.Add(uint8(ai.Anthropic), uint8(255), uint8(ai.APIVersionDefault), "future", false)
	f.Add(uint8(ai.Anthropic), uint8(ai.AnthropicBeta), uint8(255), "future-version", false)
	f.Add(uint8(ai.OpenAI), uint8(ai.OpenAIOrganization), uint8(ai.APIVersionDefault), "duplicate", true)
	f.Fuzz(func(t *testing.T, selector, rawKind, rawVersion uint8, wire string, duplicate bool) {
		provider := ai.OpenAI
		if selector&1 == 0 {
			provider = ai.Anthropic
		}
		route := ai.Route{Provider: provider, Operation: ai.Responses}
		if provider == ai.Anthropic {
			route = claude.Route()
		}
		request := streamingRequest(t, route, bytes.NewReader(nil), io.Discard)
		value, valueErr := exchange.NewHeaderValue(wire)
		kind, version := ai.OptionalHeaderName(rawKind), ai.APIVersion(rawVersion)
		request.Options = []ai.OptionalHeader{{Name: kind, Value: value}}
		request.Version = version
		if duplicate {
			request.Options = append(request.Options, request.Options[0])
		}
		// Independent closed-field ownership and version oracle. Exchange owns
		// native HTTP field grammar; this package owns provider/option binding.
		knownForProvider := provider == ai.OpenAI && (kind == ai.OpenAIOrganization || kind == ai.OpenAIProject) || provider == ai.Anthropic && (kind == ai.AnthropicWorkspace || kind == ai.AnthropicBeta)
		versionAdmitted := version == ai.APIVersionDefault || provider == ai.Anthropic && version == ai.Anthropic20230601
		wantAdmitted := valueErr == nil && knownForProvider && versionAdmitted && !duplicate
		gotErr := request.Validate()
		if (gotErr == nil) != wantAdmitted {
			t.Fatalf("optional request admission = %v, want exact provider/version/header binding %t", gotErr, wantAdmitted)
		}
		seam := &seamServer{err: io.ErrUnexpectedEOF}
		client, err := ai.NewClient(seam)
		if err != nil {
			t.Fatalf("seam fixture = %v, want admitted", err)
		}
		result, err := client.RoundTrip(t.Context(), request, streamPolicy())
		if !wantAdmitted {
			if seam.calls != 0 || result.Stream.Metadata.Attempts != 0 || (!errors.Is(err, core.ErrAIContract) && !errors.Is(err, core.ErrAIBinding)) || errors.Is(err, core.ErrAIContract) && errors.Is(err, core.ErrAIBinding) {
				t.Fatalf("optional refusal = (%v,calls %d,metadata %+v), want one typed identity and zero effects", err, seam.calls, result.Stream.Metadata)
			}
			return
		}
		if !errors.Is(err, io.ErrUnexpectedEOF) || seam.calls != 1 {
			t.Fatalf("admitted request delegation = (%v,%d calls), want preserved native seam cause and exactly one call", err, seam.calls)
		}
	})
}

// Native TLS fixtures supply foreign provider facts to the real SDK transport.
// extent selects a finite test workload; production has no transfer-size cap.
func FuzzNativeProviderResponsePreservesStatusBodyAndCorrelation(f *testing.F) {
	for _, provider := range []ai.Provider{ai.OpenAI, ai.Anthropic} {
		route := ai.Route{Provider: provider, Operation: ai.Responses}
		if provider == ai.Anthropic {
			route = claude.Route()
		}
		if err := route.Validate(); err != nil {
			f.Fatalf("typed route seed = %v, want admitted", err)
		}
		for _, status := range []int{200, 204, 304, 400, 401, 408, 429, 500, 529, 599} {
			var nominal core.HTTPStatusCode
			if err := nominal.AdmitInt(status); err != nil {
				f.Fatalf("typed native status seed = %v, want admitted", err)
			}
			native, err := nominal.Int()
			if err != nil {
				f.Fatalf("native status projection = %v, want admitted", err)
			}
			f.Add(uint8(provider), uint16(native-200), uint32(32769))
		}
	}
	for _, extent := range []uint32{0, 32767, 32768, 32769, 3*32768 + 7} {
		f.Add(uint8(ai.OpenAI), uint16(0), extent)
	}
	f.Fuzz(func(t *testing.T, selector uint8, statusSelector uint16, extentSelector uint32) {
		provider := ai.OpenAI
		if selector&1 == 0 {
			provider = ai.Anthropic
		}
		nativeStatus := 200 + int(statusSelector)%400
		extent := int64(extentSelector % (3*32768 + 8))
		// Go forbids response bodies for these statuses. This is a protocol fact,
		// not a transfer quota or a provider-specific body assumption.
		if nativeStatus == http.StatusNoContent || nativeStatus == http.StatusNotModified {
			extent = 0
		}
		var nativeFact core.HTTPStatusCode
		if err := nativeFact.AdmitInt(nativeStatus); err != nil {
			t.Fatalf("native status fixture = %v, want admitted", err)
		}
		requestID := "req-owned-native-fuzz"
		header := core.AIOpenAIRequestIDHeader
		if provider == ai.Anthropic {
			header = core.AIAnthropicRequestIDHeader
		}
		observed := make(chan error, 1)
		fixture := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			_, readErr := io.Copy(io.Discard, request.Body)
			writer.Header().Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeJSON().String())
			writer.Header().Set(header, requestID)
			writer.Header().Set("Retry-After", "1")
			writer.WriteHeader(nativeStatus)
			_, writeErr := io.CopyBuffer(writer, &generatedSource{left: extent}, make([]byte, 32768))
			observed <- errors.Join(readErr, writeErr)
		}))
		defer fixture.Close()
		client := nativeClient(t, fixture, provider)
		ordinary, failure := &conservingSink{}, &conservingSink{}
		route := ai.Route{Provider: provider, Operation: ai.Responses}
		if provider == ai.Anthropic {
			route = claude.Route()
		}
		request := streamingRequest(t, route, bytes.NewReader(nil), ordinary)
		request.Stream.FailureDestination = failure
		response, err := client.RoundTrip(t.Context(), request, streamPolicy())
		if nativeErr := <-observed; nativeErr != nil {
			t.Fatalf("native fixture transfer = %v, want complete finite fixture", nativeErr)
		}
		gotStatus, statusErr := response.Stream.Metadata.Status.Int()
		if statusErr != nil || gotStatus != nativeStatus || response.Stream.Metadata.Attempts != 1 || response.Stream.Metadata.Bytes.Uint64() != uint64(extent) {
			t.Fatalf("native metadata = (%+v,%v), want exact status %d/one attempt/%d acknowledged bytes", response.Stream.Metadata, statusErr, nativeStatus, extent)
		}
		var wantIdentity core.ErrorIdentity
		if nativeStatus == http.StatusOK {
			if err != nil || ordinary.written != extent || failure.written != 0 || response.Validate() != nil {
				t.Fatalf("native success = (%v,ordinary %d,failure %d), want admitted exact %d bytes to ordinary sink only", err, ordinary.written, failure.written, extent)
			}
		} else {
			// Distinct native status families retain one SDK identity, their actual
			// StatusError and every failure byte in the explicitly owned failure sink.
			switch {
			case nativeStatus == 401:
				wantIdentity = core.ErrAIAuthentication
			case nativeStatus == 429:
				wantIdentity = core.ErrAIRateLimited
			case nativeStatus == 408 || nativeStatus >= 500:
				wantIdentity = core.ErrAIUnavailable
			default:
				wantIdentity = core.ErrAIRequestRejected
			}
			var status exchange.StatusError
			if !errors.Is(err, wantIdentity) || !errors.As(err, &status) || ordinary.written != 0 || failure.written != extent {
				t.Fatalf("native refusal = (%v,ordinary %d,failure %d), want %v/status cause/exact %d isolated bytes", err, ordinary.written, failure.written, wantIdentity, extent)
			}
			value, nativeErr := status.Status().Int()
			if nativeErr != nil || value != nativeStatus {
				t.Fatalf("native status cause = (%d,%v), want %d", value, nativeErr, nativeStatus)
			}
		}
		for _, identity := range []core.ErrorIdentity{core.ErrAIContract, core.ErrAIAuthentication, core.ErrAIBinding, core.ErrAIRateLimited, core.ErrAIUnavailable, core.ErrAIRequestRejected} {
			if errors.Is(err, identity) != (identity == wantIdentity) {
				t.Fatalf("native response identity = %v also matches %v, want exactly %v", err, identity, wantIdentity)
			}
		}
		var gotID, gotRetry string
		for _, field := range response.Stream.Metadata.Headers.Values {
			if len(field.Values) != 1 {
				t.Fatalf("native captured cardinality = %d, want one", len(field.Values))
			}
			value, err := field.Values[0].Value()
			if err != nil {
				t.Fatalf("native captured value = %v, want admitted", err)
			}
			if field.Name.String() == "X-Request-Id" || field.Name.String() == "Request-Id" {
				gotID = value
			}
			if field.Name.String() == "Retry-After" {
				gotRetry = value
			}
		}
		if gotID != requestID || gotRetry != "1" {
			t.Fatalf("native correlation = (%q,%q), want exact request ID and retry hint", gotID, gotRetry)
		}
	})
}
