package github

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

func contractError(cause error) error { return providerError(core.ErrGitHubContract, cause) }
func authenticationError(cause error) error {
	return providerError(core.ErrGitHubAuthentication, cause)
}
func responseError(cause error) error { return providerError(core.ErrGitHubResponse, cause) }
func bindingError(cause error) error  { return providerError(core.ErrGitHubBinding, cause) }

func providerError(identity, cause error) error {
	if cause == nil {
		return identity
	}
	return errors.Join(identity, cause)
}
