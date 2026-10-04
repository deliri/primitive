package cloudflare

import (
	"strings"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
)

// AccountID binds credentialed operations to a Cloudflare account.
type AccountID struct{ value string }

func ParseAccountID(value string) (AccountID, error) {
	if !hexIdentity(value) {
		return AccountID{}, core.ErrCloudflareBinding
	}
	return AccountID{value: value}, nil
}
func (a AccountID) Validate() error { _, err := ParseAccountID(a.value); return err }
func (a AccountID) String() string  { return a.value }

func hexIdentity(value string) bool {
	if len(value) != core.CloudflareIdentityCharacters {
		return false
	}
	for _, r := range value {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
			return false
		}
	}
	return true
}

// ImageID supports generated UUIDs and documented UTF-8 custom paths. It is
// escaped by the SDK only when constructing a URL, never path-cleaned.
type ImageID struct{ value string }

func ParseImageID(value string) (ImageID, error) {
	if value == "" || !utf8.ValidString(value) || utf8.RuneCountInString(value) > core.CloudflareImageIDMaximumCharacters {
		return ImageID{}, core.ErrCloudflareBinding
	}
	if strings.HasPrefix(value, "/") || strings.HasSuffix(value, "/") || strings.ContainsAny(value, "\x00\r\n") {
		return ImageID{}, core.ErrCloudflareBinding
	}
	return ImageID{value: value}, nil
}
func (id ImageID) Validate() error { _, err := ParseImageID(id.value); return err }
func (id ImageID) String() string  { return id.value }

type StreamVideoID struct{ value string }

func ParseStreamVideoID(value string) (StreamVideoID, error) {
	if !hexIdentity(value) {
		return StreamVideoID{}, core.ErrCloudflareBinding
	}
	return StreamVideoID{value: value}, nil
}
func (id StreamVideoID) Validate() error { _, err := ParseStreamVideoID(id.value); return err }
func (id StreamVideoID) String() string  { return id.value }
