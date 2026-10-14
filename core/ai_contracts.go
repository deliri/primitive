package core

// AI protocol facts belong to Primitive; models, prompts and spend policy
// belong to the application. These constants impose no stream extent limit.
const (
	// Provider error classification preserves native HTTP facts and causes.
	// https://platform.claude.com/docs/en/api/errors
	// https://developers.openai.com/api/docs/guides/error-codes
	AIStatusAuthentication = 401
	AIStatusRateLimited    = 429
	AIStatusTimeout        = 408
	AIStatusServiceFailure = 500
	// Direct API authorities and routes:
	// https://developers.openai.com/api/reference/resources/responses/methods/create
	// https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create
	// https://developers.openai.com/api/docs/guides/speech-to-text
	// https://platform.claude.com/docs/en/api/messages/create
	AIOpenAIHost              = "api.openai.com"
	AIAnthropicHost           = "api.anthropic.com"
	AIResponsesPath           = "/v1/responses"
	AIChatCompletionsPath     = "/v1/chat/completions"
	AIAudioTranscriptionsPath = "/v1/audio/transcriptions"
	AIMessagesPath            = "/v1/messages"
	// The workspace-scoped key fallback and mandatory version header:
	// https://platform.claude.com/docs/en/manage-claude/authentication
	// https://platform.claude.com/docs/en/api/overview#authentication
	AIAnthropicKeyHeader     = "x-api-key"
	AIAnthropicVersionHeader = "anthropic-version"
	AIAnthropicVersion       = "2023-06-01"
	AIMultipartMediaType     = "multipart/form-data"
	// Optional request headers and native request correlation:
	// https://developers.openai.com/api/reference/overview
	// https://platform.claude.com/docs/en/api/overview#authentication
	// https://platform.claude.com/docs/en/api/beta-headers
	AIOpenAIOrganizationHeader = "OpenAI-Organization"
	AIOpenAIProjectHeader      = "OpenAI-Project"
	AIAnthropicWorkspaceHeader = "anthropic-workspace-id"
	AIAnthropicBetaHeader      = "anthropic-beta"
	AIOpenAIRequestIDHeader    = "x-request-id"
	AIAnthropicRequestIDHeader = "request-id"
)
