// Package chatgpt binds the shared AI SDK to OpenAI's direct API. The name
// identifies the provider adapter; it does not automate the ChatGPT website.
// Construct a Server here, then ai.NewClient(server). Application bridges own
// model choice, typed payload encoding and typed response interpretation.
//
// Requests use Authorization: Bearer and POST to the closed Responses, Chat
// Completions or Audio Transcriptions route. Transcription uses multipart form
// data; the other doors use JSON. Response bodies stream directly to the sink,
// including event streams selected by the caller's typed media contract.
// Sources:
// https://developers.openai.com/api/reference/resources/responses/methods/create
// https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create
// https://developers.openai.com/api/docs/guides/speech-to-text
// https://developers.openai.com/api/reference/overview#authentication
// https://developers.openai.com/api/reference/resources/audio/subresources/transcriptions/methods/create
package chatgpt

import (
	"github.com/deliri/primitive/v2026/ai"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/secretstore"
)

// ParseCredential admits owned OpenAI bearer-token material through ai.
func ParseCredential(material []byte) (ai.Credential, error) {
	return ai.ParseCredential(ai.OpenAI, material)
}

// CredentialFromSecret creates independently owned OpenAI credential bytes.
func CredentialFromSecret(value secretstore.Value) (ai.Credential, error) {
	return ai.CredentialFromSecret(ai.OpenAI, value)
}

// NewServer refuses a foreign provider and delegates native calls to ai.
func NewServer(client exchange.Client, credential ai.Credential) (*ai.HTTPServer, error) {
	provider, err := credential.Provider()
	if err != nil {
		return nil, err
	}
	if provider != ai.OpenAI {
		return nil, core.ErrAIBinding
	}
	return ai.NewHTTPServer(client, credential)
}

// Route binds only the supported direct OpenAI operation table.
func Route(operation ai.Operation) (ai.Route, error) {
	route := ai.Route{Provider: ai.OpenAI, Operation: operation}
	if err := route.Validate(); err != nil {
		return ai.Route{}, err
	}
	return route, nil
}
