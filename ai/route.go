package ai

import (
	"github.com/deliri/primitive/v2026/core"
)

// Provider is a closed native HTTP authority, not an application model choice.
type Provider uint8

const (
	ProviderUnknown Provider = iota
	OpenAI
	Anthropic
)

// Validate refuses an unspecified or unsupported direct API provider.
func (p Provider) Validate() error {
	switch p {
	case OpenAI, Anthropic:
		return nil
	default:
		return core.ErrAIContract
	}
}

// OffWireEnum marks adapter authority as compiler-owned execution policy.
func (Provider) OffWireEnum() {}

// Operation identifies a documented synchronous or streaming HTTP door.
type Operation uint8

const (
	OperationUnknown Operation = iota
	Responses
	ChatCompletions
	AudioTranscriptions
	Messages
	operationLimit
)

// Validate refuses an unspecified or unsupported direct API operation.
func (o Operation) Validate() error {
	switch o {
	case Responses, ChatCompletions, AudioTranscriptions, Messages:
		return nil
	default:
		return core.ErrAIContract
	}
}

// OffWireEnum marks native route selection rather than provider payload data.
func (Operation) OffWireEnum() {}

// Route seals a direct provider and operation against the closed route table.
// Sources: https://developers.openai.com/api/reference/resources/responses/methods/create
// https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create
// https://developers.openai.com/api/reference/resources/audio/subresources/transcriptions/methods/create
// https://platform.claude.com/docs/en/api/messages/create
type Route struct {
	Provider  Provider
	Operation Operation
}

// Validate refuses unknown or cross-provider operation bindings.
func (r Route) Validate() error {
	if r.Provider.Validate() != nil || r.Operation.Validate() != nil {
		return core.ErrAIBinding
	}
	if routeFacts()[r.Operation] != r {
		return core.ErrAIBinding
	}
	return nil
}

// Unkeyed rows and the enum-derived array length force additions to bind a
// provider at compile time; no boolean inference silently grants a new door.
func routeFacts() [operationLimit]Route {
	return [...]Route{{}, {Provider: OpenAI, Operation: Responses},
		{Provider: OpenAI, Operation: ChatCompletions},
		{Provider: OpenAI, Operation: AudioTranscriptions},
		{Provider: Anthropic, Operation: Messages}}
}

// Endpoint projects the exact documented HTTPS authority and operation path.
func (r Route) Endpoint() (core.HTTPEndpoint, error) {
	if err := r.Validate(); err != nil {
		return core.HTTPEndpoint{}, err
	}
	switch r.Operation {
	case Responses:
		return core.ParseHTTPEndpoint(core.SchemeHTTPS + "://" + core.AIOpenAIHost + core.AIResponsesPath)
	case ChatCompletions:
		return core.ParseHTTPEndpoint(core.SchemeHTTPS + "://" + core.AIOpenAIHost + core.AIChatCompletionsPath)
	case AudioTranscriptions:
		return core.ParseHTTPEndpoint(core.SchemeHTTPS + "://" + core.AIOpenAIHost + core.AIAudioTranscriptionsPath)
	case Messages:
		return core.ParseHTTPEndpoint(core.SchemeHTTPS + "://" + core.AIAnthropicHost + core.AIMessagesPath)
	default:
		return core.HTTPEndpoint{}, core.ErrAIBinding
	}
}
