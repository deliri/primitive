// Package claude binds the shared AI SDK to Anthropic's direct Messages API.
// Model names, typed Messages payloads, output schemas and interpretation remain
// in the application bridge. Construct a Server here, then ai.NewClient(server).
//
// Supported protocol: POST /v1/messages, JSON or event-stream responses,
// workspace-scoped API keys using the supported x-api-key fallback, and the
// mandatory anthropic-version header. A multi-workspace key supplies the typed
// Anthropic workspace option. Cloud-platform credentials require their own
// adapter; this package does not infer a workspace or a cloud authority.
// Sources:
// https://platform.claude.com/docs/en/api/messages/create
// https://platform.claude.com/docs/en/manage-claude/authentication
// https://platform.claude.com/docs/en/api/overview#authentication
// https://platform.claude.com/docs/en/api/versioning
// https://platform.claude.com/docs/en/api/beta-headers
package claude

import (
	"github.com/deliri/primitive/v2026/ai"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/secretstore"
)

// ParseCredential admits owned Anthropic x-api-key material through ai.
func ParseCredential(material []byte) (ai.Credential, error) {
	return ai.ParseCredential(ai.Anthropic, material)
}

// CredentialFromSecret creates independently owned Anthropic credential bytes.
func CredentialFromSecret(value secretstore.Value) (ai.Credential, error) {
	return ai.CredentialFromSecret(ai.Anthropic, value)
}

// NewServer rejects an otherwise valid credential for a foreign provider.
func NewServer(client exchange.Client, credential ai.Credential) (*ai.HTTPServer, error) {
	provider, err := credential.Provider()
	if err != nil {
		return nil, err
	}
	if provider != ai.Anthropic {
		return nil, core.ErrAIBinding
	}
	return ai.NewHTTPServer(client, credential)
}

// Route binds the supported direct Messages API operation.
func Route() ai.Route { return ai.Route{Provider: ai.Anthropic, Operation: ai.Messages} }
