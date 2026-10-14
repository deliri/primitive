package ai_test

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/ai"
	"github.com/deliri/primitive/v2026/core"
)

func TestCredentialRefusalSelectsExactlyOneAIIdentity(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		provider ai.Provider
		material []byte
		want     core.ErrorIdentity
	}{
		{"unknown provider", ai.ProviderUnknown, []byte("key"), core.ErrAIContract},
		{"future provider", 255, []byte("key"), core.ErrAIContract},
		{"unknown provider and absent key", ai.ProviderUnknown, nil, core.ErrAIContract},
		{"OpenAI missing key", ai.OpenAI, nil, core.ErrAIAuthentication},
		{"Anthropic missing key", ai.Anthropic, nil, core.ErrAIAuthentication},
		{"OpenAI header injection", ai.OpenAI, []byte("key\r\n"), core.ErrAIAuthentication},
		{"Anthropic header injection", ai.Anthropic, []byte("key\r\n"), core.ErrAIAuthentication},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			credential, err := ai.ParseCredential(tc.provider, tc.material)
			if !errors.Is(err, tc.want) || credential.Validate() == nil {
				t.Fatalf("credential refusal = %v, want zero credential and %v", err, tc.want)
			}
			for _, identity := range [...]core.ErrorIdentity{core.ErrAIContract, core.ErrAIAuthentication, core.ErrAIBinding, core.ErrAIRateLimited, core.ErrAIUnavailable, core.ErrAIRequestRejected} {
				if errors.Is(err, identity) != (identity == tc.want) {
					t.Fatalf("credential refusal = %v also matched %v, want only %v", err, identity, tc.want)
				}
			}
		})
	}
}
