package github

import (
	"context"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestGitHubErrorIdentitiesSurviveLocalContext(t *testing.T) {
	t.Parallel()

	tests := []struct {
		got  error
		want error
		name string
	}{
		{name: "contract identity survives absent diagnostic", got: contractError(nil), want: core.ErrGitHubContract},
		{name: "authentication identity survives absent diagnostic", got: authenticationError(nil), want: core.ErrGitHubAuthentication},
		{name: "response identity survives absent diagnostic", got: responseError(nil), want: core.ErrGitHubResponse},
		{name: "binding identity survives absent diagnostic", got: bindingError(nil), want: core.ErrGitHubBinding},
		{name: "cancellation remains independently reachable", got: responseError(context.Canceled), want: context.Canceled},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			if !errors.Is(testCase.got, testCase.want) {
				t.Fatalf("GitHub wrapped error = %v, want errors.Is(..., %v)", testCase.got, testCase.want)
			}
		})
	}
}
