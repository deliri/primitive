package cloudflare

import (
	"errors"
	"github.com/deliri/primitive/v2026/core"
)

func contractError(cause error) error { return errors.Join(core.ErrCloudflareContract, cause) }
func authenticationError(cause error) error {
	return errors.Join(core.ErrCloudflareAuthentication, cause)
}
func verificationError(cause error) error { return errors.Join(core.ErrCloudflareVerification, cause) }
func responseError(cause error) error     { return errors.Join(core.ErrCloudflareResponse, cause) }
