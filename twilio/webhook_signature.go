package twilio

import (
	"crypto/hmac"
	"crypto/sha1"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"hash"
	"io"
	"net"
	"net/url"
	"strings"

	"github.com/deliri/primitive/v2026/core"
)

type webhookSignatureHashes struct {
	WithPort    hash.Hash
	WithoutPort hash.Hash
}

func newWebhookSignatureHashes(endpoint string, token []byte) (webhookSignatureHashes, error) {
	public, err := url.Parse(endpoint)
	if err != nil {
		return webhookSignatureHashes{}, bindingError(err)
	}
	withPort, withoutPort := *public, *public
	if public.Port() == "" {
		withPort.Host = net.JoinHostPort(public.Hostname(), core.TwilioWebhookHTTPSDefaultPort)
	} else {
		// Native Go preserves IPv6 brackets; the SDK's strings.Split host
		// spelling cannot safely represent IPv6 when removing an explicit port.
		withoutPort.Host = strings.TrimSuffix(net.JoinHostPort(public.Hostname(), ""), core.TwilioWebhookPortSeparator)
	}
	hashes := webhookSignatureHashes{WithPort: hmac.New(sha1.New, token), WithoutPort: hmac.New(sha1.New, token)}
	_, firstErr := io.WriteString(hashes.WithPort, withPort.String())
	_, secondErr := io.WriteString(hashes.WithoutPort, withoutPort.String())
	return hashes, errors.Join(firstErr, secondErr)
}

func (h webhookSignatureHashes) Verify(signature string) error {
	var encoded [28]byte
	if len(signature) != len(encoded) {
		return core.ErrTwilioAuthentication
	}
	copy(encoded[:], signature)
	var expected [sha1.Size]byte
	count, err := base64.StdEncoding.Strict().Decode(expected[:], encoded[:])
	if err != nil || count != len(expected) {
		return authenticationError(err)
	}
	var first, second [sha1.Size]byte
	firstMatch := subtle.ConstantTimeCompare(h.WithPort.Sum(first[:0]), expected[:])
	secondMatch := subtle.ConstantTimeCompare(h.WithoutPort.Sum(second[:0]), expected[:])
	if firstMatch|secondMatch != 1 {
		return core.ErrTwilioVerification
	}
	return nil
}

func (webhookSignatureHashes) twilioInternalFlow() {}

var _ internalFlow = webhookSignatureHashes{}
