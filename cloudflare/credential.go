package cloudflare

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/secretstore"
)

// APIToken owns an opaque Cloudflare bearer credential. Closing it invalidates
// copies; constructed server capabilities acquire independent secret custody.
type APIToken struct{ value []byte }

func ParseAPIToken(value []byte) (APIToken, error) {
	if len(value) > core.CloudflareSecretCustodyMaximumBytes {
		return APIToken{}, core.ErrCloudflareAuthentication
	}
	candidate := APIToken{value: bytes.Clone(value)}
	if err := candidate.Validate(); err != nil {
		clear(candidate.value)
		return APIToken{}, err
	}
	return candidate, nil
}

func APITokenFromSecret(value secretstore.Value) (APIToken, error) {
	material, err := value.CopyBytes()
	if err != nil {
		return APIToken{}, authenticationError(err)
	}
	defer clear(material)
	return ParseAPIToken(material)
}

func (c APIToken) Validate() error {
	if len(c.value) > core.CloudflareSecretCustodyMaximumBytes {
		return core.ErrCloudflareAuthentication
	}
	if err := (exchange.BearerAuthorization{Token: c.value}).Validate(); err != nil {
		return authenticationError(err)
	}
	return nil
}

func (c *APIToken) Close() error {
	if c == nil {
		return core.ErrCloudflareContract
	}
	clear(c.value)
	*c = APIToken{}
	return nil
}

func (APIToken) Format(state fmt.State, _ rune) { _, _ = io.WriteString(state, core.RedactedValueText) }

// NotificationSecret is exclusively the cf-webhook-auth shared secret.
type NotificationSecret struct{ value []byte }

func ParseNotificationSecret(value []byte) (NotificationSecret, error) {
	candidate := NotificationSecret{value: value}
	if err := candidate.Validate(); err != nil {
		return NotificationSecret{}, err
	}
	return NotificationSecret{value: bytes.Clone(value)}, nil
}
func (s NotificationSecret) Validate() error {
	if len(s.value) == 0 || len(s.value) > core.CloudflareSecretCustodyMaximumBytes || strings.Trim(string(s.value), " \t") != string(s.value) {
		return core.ErrCloudflareAuthentication
	}
	if _, err := exchange.NewHeaderValue(string(s.value)); err != nil {
		return authenticationError(err)
	}
	return nil
}

func (s *NotificationSecret) Close() error {
	if s == nil {
		return core.ErrCloudflareContract
	}
	clear(s.value)
	*s = NotificationSecret{}
	return nil
}

func (NotificationSecret) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

// StreamWebhookSecret is the HMAC key returned by Stream's webhook API. It is
// used as literal bytes, not decoded as hex or base64.
type StreamWebhookSecret struct{ value secretstore.Value }

func ParseStreamWebhookSecret(value []byte) (StreamWebhookSecret, error) {
	if len(value) == 0 || len(value) > core.CloudflareSecretCustodyMaximumBytes {
		return StreamWebhookSecret{}, core.ErrCloudflareAuthentication
	}
	owned, err := secretstore.NewValue(value)
	if err != nil {
		return StreamWebhookSecret{}, authenticationError(err)
	}
	return StreamWebhookSecret{value: owned}, nil
}
func (s StreamWebhookSecret) Validate() error {
	if err := s.value.Validate(); err != nil {
		return authenticationError(err)
	}
	return nil
}
func (s *StreamWebhookSecret) Close() error {
	if s == nil {
		return core.ErrCloudflareContract
	}
	if s.value == (secretstore.Value{}) {
		return nil
	}
	err := s.value.Destroy()
	*s = StreamWebhookSecret{}
	return err
}

func (StreamWebhookSecret) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

func visibleSecret(value []byte) bool {
	if len(value) == 0 || len(value) > core.CloudflareSecretCustodyMaximumBytes {
		return false
	}
	for _, character := range value {
		if character < 0x21 || character > 0x7e {
			return false
		}
	}
	return true
}
