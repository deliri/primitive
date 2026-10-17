package loops

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func FuzzParseCredentialSemanticClosure(f *testing.F) {
	f.Add([]byte("0123456789abcdef0123456789abcdef"))
	f.Add([]byte("key-._~+/="))
	f.Add([]byte("key value"))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, value []byte) {
		credential, err := ParseCredential(value)
		bearer := (exchange.BearerAuthorization{Token: value}).Validate() == nil
		want := len(value) >= core.LoopsCredentialMinimumBytes && len(value) <= core.LoopsCredentialCustodyMaximumBytes && bearer
		if err != nil {
			if want || !errors.Is(err, core.ErrLoopsContract) || credential.Validate() == nil || credential.value != nil {
				t.Fatalf("ParseCredential(%q) = rejected %v, want admission %t and zero refusal", value, err, want)
			}
			return
		}
		if !want || credential.Validate() != nil || !bytes.Equal(credential.value, value) {
			t.Fatalf("ParseCredential(%q) = admitted, want admission %t with exact bytes", value, want)
		}
		_ = credential.Close()
	})
}

func FuzzParseWebhookSecretSemanticClosure(f *testing.F) {
	f.Add([]byte(officialWebhookSecret))
	f.Add([]byte(strings.TrimPrefix(officialWebhookSecret, "whsec_")))
	f.Add([]byte("whsec_"))
	f.Add([]byte("whsec_AB=="))
	f.Add([]byte("whsec_TWZ\nLUTk="))
	f.Fuzz(func(t *testing.T, value []byte) {
		secret, err := ParseWebhookSecret(value)
		encoded := bytes.TrimPrefix(value, []byte("whsec_"))
		decoded, decodeErr := base64.StdEncoding.Strict().DecodeString(string(encoded))
		printable := len(value) > 0 && bytes.IndexFunc(value, func(r rune) bool { return r < 0x21 || r > 0x7e }) < 0
		want := printable && len(value) <= core.LoopsWebhookSecretCustodyMaximumBytes && decodeErr == nil && len(decoded) >= core.LoopsWebhookSecretMinimumKeyBytes &&
			base64.StdEncoding.EncodeToString(decoded) == string(encoded)
		if err != nil {
			if want || !errors.Is(err, core.ErrLoopsContract) || secret.key != nil {
				t.Fatalf("ParseWebhookSecret(%q) = rejected %v, want admission %t", value, err, want)
			}
			return
		}
		if !want || !bytes.Equal(secret.key, decoded) {
			t.Fatalf("ParseWebhookSecret(%q) = admitted %d key bytes, want admission %t with stdlib decoding", value, len(secret.key), want)
		}
		_ = secret.Close()
	})
}

// independentSignatureVerdict is a deliberately separate model of the header
// grammar: well-formed iff every space entry is version,value with non-empty
// parts, each v1 value is canonical padded base64 of 32 bytes, and some v1
// entry exists; matched iff some v1 value equals expected.
func independentSignatureVerdict(header string, expected []byte) (wellFormed, matched bool) {
	sawV1 := false
	for entry := range strings.SplitSeq(header, " ") {
		comma := strings.IndexByte(entry, ',')
		if comma <= 0 || comma == len(entry)-1 {
			return false, false
		}
		if entry[:comma] != "v1" {
			continue
		}
		decoded, err := base64.StdEncoding.DecodeString(entry[comma+1:])
		if err != nil || len(decoded) != sha256.Size || base64.StdEncoding.EncodeToString(decoded) != entry[comma+1:] {
			return false, false
		}
		sawV1 = true
		matched = matched || bytes.Equal(decoded, expected)
	}
	return sawV1, matched
}

func FuzzSignatureHeaderSemanticClosure(f *testing.F) {
	expected := sha256.Sum256([]byte("expected"))
	other := sha256.Sum256([]byte("other"))
	genuine := "v1," + base64.StdEncoding.EncodeToString(expected[:])
	foreign := "v1," + base64.StdEncoding.EncodeToString(other[:])
	for _, header := range []string{genuine, foreign + " " + genuine, "v2,x " + genuine, "v1", genuine + " ", " " + genuine, foreign + "  " + genuine, "v1," + strings.Repeat("a", 64), officialWebhookSignature, ""} {
		f.Add(header)
	}
	f.Fuzz(func(t *testing.T, header string) {
		wantWellFormed, wantMatched := independentSignatureVerdict(header, expected[:])
		grammarErr := validateSignatureHeader(header)
		if (grammarErr == nil) != wantWellFormed || (grammarErr != nil && !errors.Is(grammarErr, core.ErrLoopsVerification)) {
			t.Fatalf("validateSignatureHeader(%q) = %v, want well-formed %t", header, grammarErr, wantWellFormed)
		}
		if !wantWellFormed {
			return
		}
		verifyErr := verifySignatureDigest(header, expected[:])
		if (verifyErr == nil) != wantMatched || (verifyErr != nil && !errors.Is(verifyErr, core.ErrLoopsVerification)) {
			t.Fatalf("verifySignatureDigest(%q) = %v, want matched %t", header, verifyErr, wantMatched)
		}
	})
}

func FuzzWebhookIdentityHeadersSemanticClosure(f *testing.F) {
	f.Add(officialWebhookID, officialWebhookTimestamp)
	f.Add("msg.1", "01")
	f.Add("", "9223372037")
	f.Add(strings.Repeat("i", core.LoopsWebhookIDMaximumBytes), "9223372036")
	f.Fuzz(func(t *testing.T, id, timestamp string) {
		delivery, idErr := ParseDeliveryID(id)
		wantID := len(id) >= 1 && len(id) <= core.LoopsWebhookIDMaximumBytes && !strings.Contains(id, ".") &&
			strings.IndexFunc(id, func(r rune) bool { return r < 0x21 || r > 0x7e }) < 0
		if (idErr == nil) != wantID || (idErr == nil && delivery.String() != id) || (idErr != nil && (!errors.Is(idErr, core.ErrLoopsVerification) || delivery != (DeliveryID{}))) {
			t.Fatalf("ParseDeliveryID(%q) = (%q, %v), want admission %t", id, delivery.String(), idErr, wantID)
		}
		instant, tsErr := parseSignatureTimestamp(timestamp)
		seconds, parseErr := strconv.ParseInt(timestamp, 10, 64)
		wantTS := parseErr == nil && seconds >= 0 && strconv.FormatInt(seconds, 10) == timestamp && seconds <= 9223372036
		if (tsErr == nil) != wantTS {
			t.Fatalf("parseSignatureTimestamp(%q) = %v, want admission %t", timestamp, tsErr, wantTS)
		}
		if tsErr != nil {
			if !errors.Is(tsErr, core.ErrLoopsVerification) || instant.IsSet() {
				t.Fatalf("parseSignatureTimestamp(%q) refusal = (%v, %v), want zero verification refusal", timestamp, instant, tsErr)
			}
			return
		}
		nanoseconds, err := instant.Nanoseconds()
		if err != nil || nanoseconds != seconds*1_000_000_000 {
			t.Fatalf("parseSignatureTimestamp(%q) = %d ns, want exact %d s", timestamp, nanoseconds, seconds)
		}
	})
}
