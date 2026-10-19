package ai_test

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/ai"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

// BenchmarkRequestValidateAnthropicMessages measures the local route, stream
// and optional-header admission every Client and HTTPServer RoundTrip repeats
// before any transport work. The fixture carries both Anthropic option headers;
// a post-loop foreign-option refusal proves the header projection really ran.
func BenchmarkRequestValidateAnthropicMessages(b *testing.B) {
	request := streamingRequest(b, ai.Route{Provider: ai.Anthropic, Operation: ai.Messages}, bytes.NewReader([]byte(`{"model":"m"}`)), io.Discard)
	for _, option := range []struct {
		name  ai.OptionalHeaderName
		value string
	}{{name: ai.AnthropicWorkspace, value: "workspace-benchmark"}, {name: ai.AnthropicBeta, value: "beta-benchmark"}} {
		value, err := exchange.NewHeaderValue(option.value)
		if err != nil {
			b.Fatal(err)
		}
		request.Options = append(request.Options, ai.OptionalHeader{Name: option.name, Value: value})
	}
	if len(request.Options) != 2 {
		b.Fatalf("options = %d, want 2", len(request.Options))
	}
	b.ReportAllocs()
	for b.Loop() {
		if err := request.Validate(); err != nil {
			b.Fatalf("Request.Validate() error = %v, want nil", err)
		}
	}
	foreign := request
	foreign.Options = append([]ai.OptionalHeader{{Name: ai.OpenAIProject, Value: request.Options[0].Value}}, request.Options...)
	if err := foreign.Validate(); !errors.Is(err, core.ErrAIBinding) {
		b.Fatalf("foreign option Validate() error = %v, want %v", err, core.ErrAIBinding)
	}
}
