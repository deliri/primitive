package ai_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/deliri/primitive/v2026/ai"
	"github.com/deliri/primitive/v2026/core"
)

func TestAIHeaderAndVersionEnumsExhaustTypedByteDomains(t *testing.T) {
	t.Parallel()
	for raw := range 256 {
		t.Run(fmt.Sprintf("header-and-version-byte-%d", raw), func(t *testing.T) {
			t.Parallel()
			header, version := ai.OptionalHeaderName(raw), ai.APIVersion(raw)
			wantHeader := raw >= int(ai.OpenAIOrganization) && raw <= int(ai.AnthropicBeta)
			wantVersion := raw == int(ai.APIVersionDefault) || raw == int(ai.Anthropic20230601)
			if header.IsValid() != wantHeader || (header.Validate() == nil) != wantHeader || (header.String() != "") != wantHeader {
				t.Fatalf("header enum %d = admitted:%t error:%v projection:%q, want closed admission %t", raw, header.IsValid(), header.Validate(), header.String(), wantHeader)
			}
			if version.IsValid() != wantVersion || (version.Validate() == nil) != wantVersion || (version.String() != "") != wantVersion {
				t.Fatalf("version enum %d = admitted:%t error:%v projection:%q, want pinned admission %t", raw, version.IsValid(), version.Validate(), version.String(), wantVersion)
			}
			if !wantHeader && !errors.Is(header.Validate(), core.ErrAIContract) {
				t.Fatalf("unknown header error = %v, want AI contract", header.Validate())
			}
			if !wantVersion && !errors.Is(version.Validate(), core.ErrAIContract) {
				t.Fatalf("unknown version error = %v, want AI contract", version.Validate())
			}
			if wantVersion && version.String() != core.AIAnthropicVersion {
				t.Fatalf("version projection = %q, want compiler-owned %q", version.String(), core.AIAnthropicVersion)
			}
			header.OffWireEnum()
			version.OffWireEnum()
		})
	}
	for _, row := range []struct {
		header ai.OptionalHeaderName
		want   string
	}{
		{header: ai.OpenAIOrganization, want: core.AIOpenAIOrganizationHeader},
		{header: ai.OpenAIProject, want: core.AIOpenAIProjectHeader},
		{header: ai.AnthropicWorkspace, want: core.AIAnthropicWorkspaceHeader},
		{header: ai.AnthropicBeta, want: core.AIAnthropicBetaHeader},
	} {
		if got := row.header.String(); got != row.want {
			t.Fatalf("typed native header projection = %q, want compiler-owned %q", got, row.want)
		}
	}
}
