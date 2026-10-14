package ai

import (
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

// OptionalHeaderName is the closed provider field allowlist. Its value remains
// nominal Exchange header data; it cannot introduce an arbitrary header name.
// Sources: https://developers.openai.com/api/reference/overview#authentication
// https://platform.claude.com/docs/en/api/overview#authentication
// https://platform.claude.com/docs/en/api/beta-headers
type OptionalHeaderName uint8

const (
	OptionalHeaderUnknown OptionalHeaderName = iota
	OpenAIOrganization
	OpenAIProject
	AnthropicWorkspace
	AnthropicBeta
)

// OffWireEnum marks header selection as an SDK policy enum.
func (OptionalHeaderName) OffWireEnum() {}

// Validate refuses an unspecified or unknown optional provider field.
func (h OptionalHeaderName) Validate() error {
	switch h {
	case OpenAIOrganization, OpenAIProject, AnthropicWorkspace, AnthropicBeta:
		return nil
	default:
		return core.ErrAIContract
	}
}

// IsValid reports membership in the closed optional provider-header domain.
func (h OptionalHeaderName) IsValid() bool {
	return h >= OpenAIOrganization && h <= AnthropicBeta
}

// String renders the compiler-owned native header name; unknown fields render
// empty and must be refused by Validate before execution.
func (h OptionalHeaderName) String() string {
	switch h {
	case OpenAIOrganization:
		return core.AIOpenAIOrganizationHeader
	case OpenAIProject:
		return core.AIOpenAIProjectHeader
	case AnthropicWorkspace:
		return core.AIAnthropicWorkspaceHeader
	case AnthropicBeta:
		return core.AIAnthropicBetaHeader
	default:
		return ""
	}
}

// OptionalHeader binds one closed provider field to admitted HTTP field data.
type OptionalHeader struct {
	Name  OptionalHeaderName
	Value exchange.HeaderValue
}

func (h OptionalHeader) native(provider Provider) (exchange.Header, error) {
	if err := h.Value.Validate(); err != nil {
		return exchange.Header{}, errors.Join(core.ErrAIContract, err)
	}
	var name core.HTTPHeaderName
	var err error
	switch h.Name {
	case OpenAIOrganization:
		if provider != OpenAI {
			return exchange.Header{}, core.ErrAIBinding
		}
		name, err = core.ParseHTTPHeaderName(core.AIOpenAIOrganizationHeader)
	case OpenAIProject:
		if provider != OpenAI {
			return exchange.Header{}, core.ErrAIBinding
		}
		name, err = core.ParseHTTPHeaderName(core.AIOpenAIProjectHeader)
	case AnthropicWorkspace:
		if provider != Anthropic {
			return exchange.Header{}, core.ErrAIBinding
		}
		name, err = core.ParseHTTPHeaderName(core.AIAnthropicWorkspaceHeader)
	case AnthropicBeta:
		if provider != Anthropic {
			return exchange.Header{}, core.ErrAIBinding
		}
		name, err = core.ParseHTTPHeaderName(core.AIAnthropicBetaHeader)
	default:
		return exchange.Header{}, core.ErrAIContract
	}
	if err != nil {
		return exchange.Header{}, errors.Join(core.ErrAIContract, err)
	}
	return exchange.Header{Name: name, Values: []exchange.HeaderValue{h.Value}}, nil
}

// APIVersion is a closed supported Anthropic version. Zero selects the
// pinned 2023-06-01 default; a future arm requires its source and tests.
// Source: https://platform.claude.com/docs/en/api/versioning
type APIVersion uint8

const (
	APIVersionDefault APIVersion = iota
	Anthropic20230601
)

// OffWireEnum marks explicit API-version selection as SDK policy.
func (APIVersion) OffWireEnum() {}

// Validate admits only the supported default or explicit pinned version.
func (v APIVersion) Validate() error {
	switch v {
	case APIVersionDefault, Anthropic20230601:
		return nil
	default:
		return core.ErrAIContract
	}
}

// IsValid reports whether the explicit or default pinned version is supported.
func (v APIVersion) IsValid() bool {
	return v == APIVersionDefault || v == Anthropic20230601
}

// String projects the supported compiler-owned Anthropic version token. Default
// and explicit selection currently identify the same documented API version.
func (v APIVersion) String() string {
	if !v.IsValid() {
		return ""
	}
	return core.AIAnthropicVersion
}

func optionalHeaders(request Request) (exchange.Headers, error) {
	if request.Version.Validate() != nil || request.Route.Provider == OpenAI && request.Version != APIVersionDefault {
		return exchange.Headers{}, core.ErrAIBinding
	}
	if len(request.Options) > int(AnthropicBeta) {
		return exchange.Headers{}, core.ErrAIContract
	}
	headers := exchange.Headers{}
	for index, option := range request.Options {
		for prior := range index {
			if request.Options[prior].Name == option.Name {
				return exchange.Headers{}, core.ErrAIContract
			}
		}
		header, err := option.native(request.Route.Provider)
		if err != nil {
			return exchange.Headers{}, err
		}
		headers.Values = append(headers.Values, header)
	}
	return headers, nil
}
