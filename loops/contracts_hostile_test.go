package loops

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestCredentialHostileBoundaries(t *testing.T) {
	t.Parallel()
	tests := []struct {
		wantErr error
		name    string
		value   string
	}{
		{name: "undocumented prefix is not invented", value: "a"},
		{name: "observed hex dashboard spelling", value: "0123456789abcdef0123456789abcdef"},
		{name: "bearer token punctuation admitted", value: "key-._~+/="},
		{name: "exact custody maximum", value: strings.Repeat("k", core.LoopsCredentialCustodyMaximumBytes)},
		{name: "empty rejected", wantErr: core.ErrLoopsContract},
		{name: "one above custody maximum", value: strings.Repeat("k", core.LoopsCredentialCustodyMaximumBytes+1), wantErr: core.ErrLoopsContract},
		{name: "control byte rejected", value: "key\n", wantErr: core.ErrLoopsContract},
		{name: "interior space rejected", value: "key value", wantErr: core.ErrLoopsContract},
		{name: "visible non-bearer punctuation rejected", value: "key!", wantErr: core.ErrLoopsContract},
		{name: "non-ASCII rejected", value: "ключ", wantErr: core.ErrLoopsContract},
	}
	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseCredential([]byte(testCase.value))
			if testCase.wantErr != nil {
				if !errors.Is(err, testCase.wantErr) || got.Validate() == nil || got.value != nil {
					t.Fatalf("ParseCredential() = (%v, %v), want zero and %v", got, err, testCase.wantErr)
				}
				return
			}
			if err != nil || got.Validate() != nil || !bytes.Equal(got.value, []byte(testCase.value)) {
				t.Fatalf("ParseCredential() error = %v validation = %v, want exact admitted bytes", err, got.Validate())
			}
			if fmt.Sprint(got) != core.RedactedValueText || fmt.Sprintf("%+v", got) != core.RedactedValueText {
				t.Fatalf("credential format = %q, want redacted", fmt.Sprint(got))
			}
			if err := got.Close(); err != nil || got.Validate() == nil {
				t.Fatalf("Close() = %v, validation after close = %v, want nil and refusal", err, got.Validate())
			}
		})
	}
}

func TestCredentialIngestCopiesCallerBytes(t *testing.T) {
	t.Parallel()
	source := []byte("caller-owned-key")
	credential, err := ParseCredential(source)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = credential.Close() }()
	source[0] = 'X'
	if string(credential.value) != "caller-owned-key" {
		t.Fatalf("credential custody = %q, want independent copy", credential.value)
	}
	var nilCredential *Credential
	if got := nilCredential.Close(); !errors.Is(got, core.ErrLoopsContract) {
		t.Fatalf("nil credential Close() = %v, want %v", got, core.ErrLoopsContract)
	}
}

func TestWebhookSecretDecodesStandardWebhooksSpellings(t *testing.T) {
	t.Parallel()
	key := []byte("0123456789abcdefghijklmn")
	encoded := base64.StdEncoding.EncodeToString(key)
	for _, testCase := range []struct {
		wantErr error
		name    string
		value   string
		want    []byte
	}{
		{name: "prefixed dashboard spelling", value: "whsec_" + encoded, want: key},
		{name: "unprefixed base64 spelling", value: encoded, want: key},
		{name: "one-byte decoded key", value: "whsec_" + base64.StdEncoding.EncodeToString([]byte{7}), want: []byte{7}},
		{name: "prefix only has no key", value: "whsec_", wantErr: core.ErrLoopsContract},
		{name: "empty", wantErr: core.ErrLoopsContract},
		{name: "unpadded base64 rejected", value: "whsec_" + strings.TrimRight(base64.StdEncoding.EncodeToString([]byte{1, 2}), "="), wantErr: core.ErrLoopsContract},
		{name: "URL alphabet rejected", value: "whsec_" + base64.URLEncoding.EncodeToString([]byte{0xfb, 0xff, 0xfe}), wantErr: core.ErrLoopsContract},
		{name: "non-canonical trailing bits rejected", value: "whsec_AB==", wantErr: core.ErrLoopsContract},
		{name: "double prefix is not stripped twice", value: "whsec_whsec_" + encoded, wantErr: core.ErrLoopsContract},
		{name: "embedded newline rejected", value: "whsec_" + encoded[:4] + "\n" + encoded[4:], wantErr: core.ErrLoopsContract},
		{name: "one above custody maximum", value: strings.Repeat("A", core.LoopsWebhookSecretCustodyMaximumBytes+4), wantErr: core.ErrLoopsContract},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseWebhookSecret([]byte(testCase.value))
			if testCase.wantErr != nil {
				if !errors.Is(err, testCase.wantErr) || got.Validate() == nil || got.key != nil {
					t.Fatalf("ParseWebhookSecret() = (%v, %v), want zero and %v", got, err, testCase.wantErr)
				}
				return
			}
			if err != nil || got.Validate() != nil || !bytes.Equal(got.key, testCase.want) || fmt.Sprint(got) != core.RedactedValueText {
				t.Fatalf("ParseWebhookSecret() = (%v, %v), want decoded key and redacted format", got, err)
			}
			custody := got.key
			if err := got.Close(); err != nil || got.Validate() == nil {
				t.Fatalf("Close() = %v, want nil and closed refusal", err)
			}
			for index, value := range custody {
				if value != 0 {
					t.Fatalf("WebhookSecret.Close retained key byte %d = %#x, want 0", index, value)
				}
			}
		})
	}
}

func TestWebhookSecretCustodyMaximumIsExact(t *testing.T) {
	t.Parallel()
	encodedAtMaximum := strings.Repeat("A", core.LoopsWebhookSecretCustodyMaximumBytes)
	secret, err := ParseWebhookSecret([]byte(encodedAtMaximum))
	if err != nil || secret.Validate() != nil || len(secret.key) != base64.StdEncoding.DecodedLen(core.LoopsWebhookSecretCustodyMaximumBytes) {
		t.Fatalf("ParseWebhookSecret(exact maximum) = (%d key bytes, %v), want admitted", len(secret.key), err)
	}
	_ = secret.Close()
	_, err = ParseWebhookSecret([]byte(encodedAtMaximum + "A"))
	if !errors.Is(err, core.ErrLoopsContract) {
		t.Fatalf("ParseWebhookSecret(one above maximum) error = %v, want %v", err, core.ErrLoopsContract)
	}
}

func TestContactIdentifierHostileBoundaries(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct {
		name  string
		value string
		kind  ContactIdentifierKind
		want  bool
	}{
		{name: "email bytes are opaque", kind: ContactIdentifierEmail, value: "person+tag@example.com", want: true},
		{name: "user identity", kind: ContactIdentifierUserID, value: "usr_123", want: true},
		{name: "unicode identity", kind: ContactIdentifierUserID, value: "用户", want: true},
		{name: "query delimiters remain data", kind: ContactIdentifierEmail, value: "a&b=c?d#e@example.com", want: true},
		{name: "exact provider maximum", kind: ContactIdentifierUserID, value: strings.Repeat("u", core.LoopsContactIdentifierMaximumBytes), want: true},
		{name: "one above provider maximum", kind: ContactIdentifierUserID, value: strings.Repeat("u", core.LoopsContactIdentifierMaximumBytes+1)},
		{name: "empty value", kind: ContactIdentifierEmail},
		{name: "control character", kind: ContactIdentifierEmail, value: "a\r\nb@example.com"},
		{name: "invalid UTF-8", kind: ContactIdentifierUserID, value: "\xff"},
		{name: "unknown kind", kind: contactIdentifierKindLimit, value: "usr_1"},
		{name: "zero kind", value: "usr_1"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseContactIdentifier(testCase.kind, testCase.value)
			if !testCase.want {
				if !errors.Is(err, core.ErrLoopsContract) || !got.IsZero() {
					t.Fatalf("ParseContactIdentifier() = (%+v, %v), want zero and contract refusal", got, err)
				}
				return
			}
			if err != nil || got.Kind() != testCase.kind || got.String() != testCase.value {
				t.Fatalf("ParseContactIdentifier() = (%+v, %v), want exact identity", got, err)
			}
		})
	}
}

func TestClosedEnumsExhaustTheirDomains(t *testing.T) {
	t.Parallel()
	for kind := range contactIdentifierKindLimit + 2 {
		want := kind == ContactIdentifierEmail || kind == ContactIdentifierUserID
		if kind.IsValid() != want || (kind.String() != "") != want {
			t.Fatalf("ContactIdentifierKind(%d) valid/string = %t/%q, want %t", kind, kind.IsValid(), kind.String(), want)
		}
	}
	for filter := range propertyListLimit + 2 {
		want := filter == PropertyListAll || filter == PropertyListCustom
		if filter.IsValid() != want || (filter.String() != "") != want {
			t.Fatalf("PropertyListFilter(%d) valid/string = %t/%q, want %t", filter, filter.IsValid(), filter.String(), want)
		}
	}
	for size := range 256 {
		want := size >= core.LoopsPageSizeMinimum && size <= core.LoopsPageSizeMaximum
		got, err := ParsePageSize(uint8(size))
		if (err == nil) != want || (err == nil && (uint8(got) != uint8(size) || got.String() != strconv.Itoa(size))) || (err != nil && (got != 0 || !errors.Is(err, core.ErrLoopsContract))) {
			t.Fatalf("ParsePageSize(%d) = (%d %q, %v), want admitted %t", size, got, got.String(), err, want)
		}
	}
}

func TestCursorAndPageBoundaries(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct {
		name  string
		value string
		want  bool
	}{
		{name: "provider cuid", value: "clx42l54f20i1la0lfooe3z12", want: true},
		{name: "opaque punctuation", value: "a+b/c=d&e", want: true},
		{name: "exact maximum", value: strings.Repeat("c", core.LoopsCursorMaximumBytes), want: true},
		{name: "one above maximum", value: strings.Repeat("c", core.LoopsCursorMaximumBytes+1)},
		{name: "empty"},
		{name: "space", value: "a b"},
		{name: "non-ASCII", value: "ç"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			got, err := ParseCursor(testCase.value)
			if (err == nil) != testCase.want || (err == nil && got.String() != testCase.value) || (err != nil && (!got.IsZero() || !errors.Is(err, core.ErrLoopsContract))) {
				t.Fatalf("ParseCursor(%q) = (%q, %v), want admitted %t", testCase.value, got.String(), err, testCase.want)
			}
		})
	}
	cursor, err := ParseCursor("next")
	if err != nil {
		t.Fatal(err)
	}
	for _, testCase := range []struct {
		name string
		page Page
		want bool
	}{
		{name: "zero page omits both parameters", want: true},
		{name: "minimum size", page: Page{Size: core.LoopsPageSizeMinimum}, want: true},
		{name: "maximum size with cursor", page: Page{Size: core.LoopsPageSizeMaximum, Cursor: cursor}, want: true},
		{name: "cursor alone", page: Page{Cursor: cursor}, want: true},
		{name: "below minimum", page: Page{Size: core.LoopsPageSizeMinimum - 1}},
		{name: "above maximum", page: Page{Size: core.LoopsPageSizeMaximum + 1}},
		{name: "forged cursor", page: Page{Cursor: Cursor{value: "a b"}}},
	} {
		if (testCase.page.Validate() == nil) != testCase.want {
			t.Fatalf("%s: Page.Validate() = %v, want admitted %t", testCase.name, testCase.page.Validate(), testCase.want)
		}
	}
}

func TestDeliveryIDAndTimestampGrammar(t *testing.T) {
	t.Parallel()
	for _, testCase := range []struct {
		name  string
		value string
		want  bool
	}{
		{name: "standard webhooks message id", value: "msg_p5jXN8AQM9LWM0D4loKWxJek", want: true},
		{name: "exact maximum", value: strings.Repeat("i", core.LoopsWebhookIDMaximumBytes), want: true},
		{name: "one above maximum", value: strings.Repeat("i", core.LoopsWebhookIDMaximumBytes+1)},
		{name: "signed content separator", value: "msg.1"},
		{name: "space", value: "msg 1"},
		{name: "empty"},
	} {
		got, err := ParseDeliveryID(testCase.value)
		if (err == nil) != testCase.want || (err == nil && got.String() != testCase.value) || (err != nil && !errors.Is(err, core.ErrLoopsVerification)) {
			t.Fatalf("%s: ParseDeliveryID() = (%q, %v), want admitted %t", testCase.name, got.String(), err, testCase.want)
		}
	}
	for _, testCase := range []struct {
		name  string
		value string
		want  int64
		ok    bool
	}{
		{name: "epoch", value: "0", ok: true},
		{name: "ordinary seconds", value: "1614265330", want: 1614265330, ok: true},
		{name: "largest representable instant", value: "9223372036", want: 9223372036, ok: true},
		{name: "first overflowing instant", value: "9223372037"},
		{name: "leading zero", value: "01614265330"},
		{name: "sign", value: "+1614265330"},
		{name: "negative", value: "-1"},
		{name: "fraction", value: "1614265330.5"},
		{name: "exponent", value: "1e9"},
		{name: "overlong", value: strings.Repeat("9", core.LoopsWebhookTimestampMaximumBytes+1)},
		{name: "int64 overflow", value: "9223372036854775808"},
		{name: "empty"},
	} {
		got, err := parseSignatureTimestamp(testCase.value)
		if !testCase.ok {
			if !errors.Is(err, core.ErrLoopsVerification) || got.IsSet() {
				t.Fatalf("%s: parseSignatureTimestamp() = (%v, %v), want zero verification refusal", testCase.name, got, err)
			}
			continue
		}
		if nanoseconds, nanosecondsErr := got.Nanoseconds(); err != nil || nanosecondsErr != nil || nanoseconds != testCase.want*1_000_000_000 {
			t.Fatalf("%s: parseSignatureTimestamp() = (%v, %v), want %d seconds", testCase.name, got, err, testCase.want)
		}
	}
}
