package cloudflare

import (
	"bytes"
	"errors"
	"fmt"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/secretstore"
)

func FuzzCloudflareSecretCustody(f *testing.F) {
	seed, err := ParseAPIToken([]byte("cloudflare-opaque-token"))
	if err != nil {
		f.Fatal(err)
	}
	f.Add(bytes.Clone(seed.value))
	if err := seed.Close(); err != nil {
		f.Fatal(err)
	}
	f.Add([]byte{})
	f.Add([]byte("header\r\ninjection"))
	f.Add(bytes.Repeat([]byte{'x'}, core.CloudflareSecretCustodyMaximumBytes+1))
	f.Fuzz(func(t *testing.T, data []byte) {
		// The real constructors see all bytes before any bounded oracle copies.
		token, tokenErr := ParseAPIToken(data)
		notification, notificationErr := ParseNotificationSecret(data)
		stream, streamErr := ParseStreamWebhookSecret(data)
		r2, r2Err := ParseR2Credentials(data, data)
		defer func() {
			for _, err := range []error{token.Close(), notification.Close(), stream.Close(), r2.Close()} {
				if err != nil {
					t.Error(err)
				}
			}
		}()
		for _, gotErr := range []error{tokenErr, notificationErr, streamErr, r2Err} {
			if gotErr != nil && !errors.Is(gotErr, core.ErrCloudflareAuthentication) {
				t.Fatalf("credential error=%v, want authentication identity", gotErr)
			}
		}
		if tokenErr != nil && len(token.value) != 0 || notificationErr != nil && len(notification.value) != 0 || streamErr != nil && stream.value != (secretstore.Value{}) || r2Err != nil && (len(r2.accessKey) != 0 || len(r2.secretKey) != 0) {
			t.Fatalf("refused custody lengths = (%d,%d,%d,%d), Stream zero = %v, want zero custody", len(token.value), len(notification.value), len(r2.accessKey), len(r2.secretKey), stream.value == (secretstore.Value{}))
		}
		if len(data) > core.CloudflareSecretCustodyMaximumBytes {
			if tokenErr == nil || notificationErr == nil || streamErr == nil || r2Err == nil {
				t.Fatalf("%d-byte secret errors = (%v,%v,%v,%v), want typed refusals", len(data), tokenErr, notificationErr, streamErr, r2Err)
			}
			return
		}
		for _, tc := range []struct {
			err   error
			name  string
			value []byte
		}{
			{name: "API token", value: token.value, err: tokenErr}, {name: "notification header", value: notification.value, err: notificationErr},
			{name: "R2 key", value: r2.secretKey, err: r2Err},
		} {
			if tc.err == nil && !bytes.Equal(tc.value, data) {
				t.Fatalf("%s retained bytes changed, want exact admitted input", tc.name)
			}
			if len(data) > 0 && len(tc.value) > 0 && &data[0] == &tc.value[0] {
				t.Fatalf("%s custody aliases input, want independent ownership", tc.name)
			}
		}
		if tokenErr == nil {
			if token.Validate() != nil {
				t.Fatalf("admitted token Validate() = %v, want nil", token.Validate())
			}
			value, err := secretstore.NewValue(data)
			if err != nil {
				t.Fatal(err)
			}
			fromSecret, err := APITokenFromSecret(value)
			if destroyErr := value.Destroy(); destroyErr != nil {
				t.Fatal(destroyErr)
			}
			if err != nil || !bytes.Equal(fromSecret.value, data) || fromSecret.Validate() != nil {
				t.Fatalf("secret handoff error=%v, want exact independently owned token", err)
			}
			if err := fromSecret.Close(); err != nil {
				t.Fatal(err)
			}
		}
		if streamErr == nil {
			material, err := stream.value.CopyBytes()
			if err != nil || !bytes.Equal(material, data) {
				t.Fatalf("Stream HMAC material error=%v, want exact source bytes", err)
			}
			clear(material)
		}
		for _, text := range []string{fmt.Sprintf("%+v", token), fmt.Sprintf("%#v", notification), fmt.Sprintf("%s", stream), fmt.Sprintf("%v", r2)} {
			if text != core.RedactedValueText {
				t.Fatalf("secret formatting=%q, want redacted", text)
			}
		}
		copies := [][]byte{token.value, notification.value, r2.accessKey, r2.secretKey}
		for _, err := range []error{token.Close(), notification.Close(), stream.Close(), r2.Close()} {
			if err != nil {
				t.Error(err)
			}
		}
		for _, copy := range copies {
			for _, b := range copy {
				if b != 0 {
					t.Fatalf("closed credential erased = %v, want true", b == 0)
				}
			}
		}
	})
}

func TestCloudflareSecretBoundsAndByteAlphabet(t *testing.T) {
	t.Parallel()
	for _, size := range []int{0, 1, core.CloudflareSecretCustodyMaximumBytes - 1, core.CloudflareSecretCustodyMaximumBytes, core.CloudflareSecretCustodyMaximumBytes + 1} {
		data := bytes.Repeat([]byte{'a'}, size)
		got, err := ParseStreamWebhookSecret(data)
		want := size > 0 && size <= core.CloudflareSecretCustodyMaximumBytes
		if (err == nil) != want {
			t.Fatalf("secret length %d error=%v, want admitted=%t", size, err, want)
		}
		if err := got.Close(); err != nil {
			t.Fatal(err)
		}
	}
	for raw := range 256 {
		got, err := ParseNotificationSecret([]byte{byte(raw)})
		want := raw >= 0x21 && raw != 0x7f
		if (err == nil) != want {
			t.Fatalf("notification byte=%d error=%v, want admitted=%t", raw, err, want)
		}
		if err := got.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestStreamHMACKeyIsOpaqueAndNotificationsUseHeaderGrammar(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name string
		key  []byte
	}{
		{name: "documented example contains spaces", key: []byte("secret from the Cloudflare API")},
		{name: "UTF8 bytes remain literal HMAC key", key: []byte("clé privée")},
		{name: "binary zero remains valid HMAC key material", key: []byte{0}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseStreamWebhookSecret(tc.key)
			if err != nil {
				t.Fatalf("Stream key admission error=%v, want nil", err)
			}
			copy := got
			if err := got.Close(); err != nil {
				t.Fatal(err)
			}
			if !errors.Is(copy.Validate(), core.ErrCloudflareAuthentication) {
				t.Fatalf("closed key copy Validate() = %v, want %v", copy.Validate(), core.ErrCloudflareAuthentication)
			}
		})
	}
	got, err := ParseNotificationSecret([]byte("internal spaces"))
	if err != nil {
		t.Fatalf("notification header secret error=%v, want nil", err)
	}
	if err := got.Close(); err != nil {
		t.Fatal(err)
	}
}
