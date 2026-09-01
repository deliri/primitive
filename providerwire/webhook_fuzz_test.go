package providerwire

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	jsontext "encoding/json/jsontext"
	"errors"
	"net/url"
	"path"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
	stripesdk "github.com/stripe/stripe-go/v86"
)

func FuzzStripeWebhookReceiveSemanticBoundary(f *testing.F) {
	canonical := []byte(`{"id":"evt_fuzz","type":"charge.succeeded"}`)
	signed := stripesdk.GenerateTestSignedPayload(&stripesdk.UnsignedPayload{
		Payload: canonical, Secret: "whsec_test_123", Timestamp: time.Unix(1_800_000_000, 0),
	})
	f.Add(canonical, signed.Header)
	f.Add([]byte{}, "")
	f.Add([]byte(`{"id":"evt_fuzz","type":"charge.failed"}`), signed.Header)

	f.Fuzz(func(t *testing.T, data []byte, signature string) {
		receiver, secret := stripeWebhookTestReceiver(t)
		defer closeStripeWebhookTestReceiver(t, &receiver, &secret)
		request := webhookRequest(t, "https://api.example.test/v1/stripe", data, "application/json")
		request.Header.Set(StripeWebhookSignatureHeaderName, signature)
		tolerance, err := temporal.DurationFromMinutes(5)
		if err != nil {
			t.Fatalf("temporal.DurationFromMinutes() error = %v, want nil", err)
		}
		var destination bytes.Buffer
		observation, gotErr := receiver.Receive(StripeWebhookReceiveRequest{
			Request: request, Destination: &destination,
			ObservedAt: providerWebhookInstant(t, 1_800_000_000), Tolerance: tolerance,
		})
		if bytes.Equal(data, canonical) && gotErr == nil {
			if gotErr != nil || observation.Provider != ProviderStripe || !bytes.Equal(destination.Bytes(), canonical) {
				t.Fatalf("Stripe signed canonical Receive() = observation:%+v body:%q error:%v, want exact signed body and nil", observation, destination.Bytes(), gotErr)
			}
			return
		}
		wantErr := error(core.ErrProviderWireVerification)
		if len(data) > StripeWebhookMaximumBytes {
			wantErr = core.ErrExchangeBodyLimit
		} else if len(signature) == 0 || len(signature) > StripeWebhookSignatureMaximumBytes {
			wantErr = core.ErrProviderWireAuthentication
		}
		if !errors.Is(gotErr, wantErr) || observation.Validate() == nil || destination.Len() != 0 {
			t.Fatalf("Stripe mutated Receive() = observation:%+v bytes:%d error:%v, want zero, 0, %v", observation, destination.Len(), gotErr, wantErr)
		}
	})
}

func FuzzTwilioWebhookReceiveSemanticBoundary(f *testing.F) {
	const publicURL = "https://mycompany.com/myapp.php?foo=1&bar=2"
	canonical := []byte("property=value&boolean=true")
	signature := twilioFormTestSignature(f, publicURL)
	f.Add(canonical, signature)
	f.Add([]byte{}, "")
	f.Add([]byte("property=value&boolean=false"), signature)

	f.Fuzz(func(t *testing.T, data []byte, receivedSignature string) {
		receiver, token := twilioWebhookTestReceiver(t, publicURL, TwilioWebhookRepresentationForm)
		defer closeTwilioWebhookTestReceiver(t, &receiver, &token)
		request := webhookRequest(t, publicURL, data, "application/x-www-form-urlencoded")
		request.Header.Set(TwilioWebhookSignatureHeaderName, receivedSignature)
		var destination bytes.Buffer
		observation, gotErr := receiver.Receive(request, &destination)
		if bytes.Equal(data, canonical) && gotErr == nil {
			if gotErr != nil || observation.Provider != ProviderTwilio || !bytes.Equal(destination.Bytes(), canonical) {
				t.Fatalf("Twilio signed canonical Receive() = observation:%+v body:%q error:%v, want exact signed body and nil", observation, destination.Bytes(), gotErr)
			}
			return
		}
		wantErr := error(core.ErrProviderWireVerification)
		if len(data) > TwilioWebhookMaximumBytes {
			wantErr = core.ErrExchangeBodyLimit
		} else if len(receivedSignature) != TwilioWebhookSignatureBytes {
			wantErr = core.ErrProviderWireAuthentication
		}
		if !errors.Is(gotErr, wantErr) || observation.Validate() == nil || destination.Len() != 0 {
			t.Fatalf("Twilio mutated Receive() = observation:%+v bytes:%d error:%v, want zero, 0, %v", observation, destination.Len(), gotErr, wantErr)
		}
	})
}

func FuzzTwilioJSONWebhookReceiveSemanticBoundary(f *testing.F) {
	const publicURL = "https://mycompany.com/json-callback?tenant=one"
	canonical := []byte(`{"CallSid":"CA123","CallStatus":"completed"}`)
	digest := sha256.Sum256(canonical)
	canonicalDigest := hex.EncodeToString(digest[:])
	_, signature := twilioJSONTestSignature(f, publicURL, canonical)
	f.Add(canonical, canonicalDigest, signature)
	f.Add([]byte{}, "", "")
	f.Add(canonical, canonicalDigest[:len(canonicalDigest)-1], signature)
	f.Add([]byte(`{"CallSid":"CA999"}`), canonicalDigest, signature)

	f.Fuzz(func(t *testing.T, data []byte, bodyDigest, receivedSignature string) {
		receiver, token := twilioWebhookTestReceiver(t, publicURL, TwilioWebhookRepresentationJSON)
		defer closeTwilioWebhookTestReceiver(t, &receiver, &token)
		target := publicURL + "&" + TwilioBodySHA256QueryName + "=" + url.QueryEscape(bodyDigest)
		request := webhookRequest(t, target, data, "application/json")
		request.Header.Set(TwilioWebhookSignatureHeaderName, receivedSignature)
		var destination bytes.Buffer
		observation, gotErr := receiver.Receive(request, &destination)
		if bytes.Equal(data, canonical) && bodyDigest == canonicalDigest && gotErr == nil {
			if gotErr != nil || observation.Provider != ProviderTwilio || !bytes.Equal(destination.Bytes(), canonical) {
				t.Fatalf("Twilio JSON signed canonical Receive() = observation:%+v body:%q error:%v, want exact signed body and nil", observation, destination.Bytes(), gotErr)
			}
			return
		}
		wantErr := error(core.ErrProviderWireVerification)
		if !validTwilioBodySHA256(bodyDigest) {
			wantErr = core.ErrProviderWireBinding
		} else if len(data) > TwilioWebhookMaximumBytes {
			wantErr = core.ErrExchangeBodyLimit
		} else if len(receivedSignature) != TwilioWebhookSignatureBytes {
			wantErr = core.ErrProviderWireAuthentication
		}
		if !errors.Is(gotErr, wantErr) || observation.Validate() == nil || destination.Len() != 0 {
			t.Fatalf("Twilio JSON mutated Receive() = observation:%+v bytes:%d error:%v, want zero, 0, %v", observation, destination.Len(), gotErr, wantErr)
		}
	})
}

func FuzzPlunkWebhookReceiveSemanticBoundary(f *testing.F) {
	f.Add([]byte(`{"event":"delivered"}`), "plunk-shared-secret")
	f.Add([]byte{}, "plunk-shared-secret")
	f.Add([]byte{0xff, 0x00}, "foreign-secret")

	f.Fuzz(func(t *testing.T, data []byte, bearer string) {
		receiver, secret := plunkWebhookTestReceiver(t)
		defer closePlunkWebhookTestReceiver(t, &receiver, &secret)
		request := webhookRequest(t, "https://api.example.test/v1/plunk", data, "application/json")
		request.Header.Set("Authorization", "Bearer "+bearer)
		var destination bytes.Buffer
		observation, gotErr := receiver.Receive(request, &destination)
		if bearer == "plunk-shared-secret" && len(data) <= PlunkWebhookMaximumBytes {
			if gotErr != nil || observation.Provider != ProviderPlunk || observation.Bytes.Uint64() != uint64(len(data)) || !bytes.Equal(destination.Bytes(), data) {
				t.Fatalf("Plunk authenticated Receive() = observation:%+v body:%q error:%v, want exact %d-byte opaque body and nil", observation, destination.Bytes(), gotErr, len(data))
			}
			return
		}
		wantErr := error(core.ErrProviderWireAuthentication)
		if bearer == "plunk-shared-secret" {
			wantErr = core.ErrExchangeBodyLimit
		}
		if !errors.Is(gotErr, wantErr) || observation.Validate() == nil || destination.Len() != 0 {
			t.Fatalf("Plunk refused Receive() = observation:%+v bytes:%d error:%v, want zero, 0, %v", observation, destination.Len(), gotErr, wantErr)
		}
	})
}

func FuzzPayPalWebhookReceiveSemanticBoundary(f *testing.F) {
	f.Add([]byte(`{"id":"WH-EVENT-123"}`))
	f.Add([]byte{})
	f.Add([]byte(`[]`))
	f.Add([]byte(`{"id":`))

	f.Fuzz(func(t *testing.T, data []byte) {
		verificationCalls := uint64(0)
		receiver, token := payPalWebhookTestReceiver(t, &verificationCalls, PayPalWebhookVerificationSuccess)
		defer closePayPalWebhookTestReceiver(t, &receiver, &token)
		request := payPalWebhookRequest(t, data, true)
		var destination bytes.Buffer
		observation, gotErr := receiver.Receive(request, &destination, providerOperationPolicy(t))
		// doctrine:local-allowed=external-wire
		value := jsontext.Value(data)
		accepted := len(data) != 0 && len(data) <= PayPalWebhookMaximumBytes && value.IsValid() && value.Kind() == jsontext.KindBeginObject
		if accepted {
			if gotErr != nil || verificationCalls != 1 || observation.Provider != ProviderPayPal || !bytes.Equal(destination.Bytes(), data) {
				t.Fatalf("PayPal admitted Receive() = calls:%d observation:%+v body:%q error:%v, want 1 and exact authenticated body", verificationCalls, observation, destination.Bytes(), gotErr)
			}
			return
		}
		wantErr := core.ErrProviderWireContract
		if len(data) > PayPalWebhookMaximumBytes {
			wantErr = core.ErrExchangeBodyLimit
		}
		if !errors.Is(gotErr, wantErr) || verificationCalls != 0 || observation.Validate() == nil || destination.Len() != 0 {
			t.Fatalf("PayPal refused Receive() = calls:%d observation:%+v bytes:%d error:%v, want 0, zero, 0, %v", verificationCalls, observation, destination.Len(), gotErr, wantErr)
		}
	})
}

func FuzzPayPalWebhookHeaderSemanticBoundary(f *testing.F) {
	f.Add("SHA256withRSA", "transmission-123", "signature-123", "2026-08-31T20:00:00Z")
	f.Add("", "transmission-123", "signature-123", "2026-08-31T20:00:00Z")
	f.Add("SHA256withRSA", "123", "signature-123", "2026-08-31T20:00:00Z")
	f.Add("SHA256withRSA", "transmission-123", "signature-123", "not-a-time")

	f.Fuzz(func(t *testing.T, algorithm, transmissionID, signature, transmissionTime string) {
		verificationCalls := uint64(0)
		receiver, token := payPalWebhookTestReceiver(t, &verificationCalls, PayPalWebhookVerificationSuccess)
		defer closePayPalWebhookTestReceiver(t, &receiver, &token)
		body := []byte(`{"id":"WH-EVENT-123"}`)
		request := payPalWebhookRequest(t, body, true)
		request.Header.Set(PayPalAuthAlgorithmHeaderName, algorithm)
		request.Header.Set(PayPalTransmissionIDHeaderName, transmissionID)
		request.Header.Set(PayPalTransmissionSignatureHeaderName, signature)
		request.Header.Set(PayPalTransmissionTimeHeaderName, transmissionTime)
		var destination bytes.Buffer
		observation, gotErr := receiver.Receive(request, &destination, providerOperationPolicy(t))
		wantAccepted := payPalAuthAlgorithmOracle(algorithm) && payPalTransmissionMemberOracle(transmissionID, PayPalTransmissionIDMaximumBytes) &&
			payPalTransmissionMemberOracle(signature, PayPalTransmissionSignatureMaximumBytes) && payPalTransmissionTimeOracle(transmissionTime)
		if wantAccepted {
			if gotErr != nil || verificationCalls != 1 || observation.Provider != ProviderPayPal || !bytes.Equal(destination.Bytes(), body) {
				t.Fatalf("PayPal admitted header Receive() = calls:%d observation:%+v body:%q error:%v, want 1, exact body, nil", verificationCalls, observation, destination.Bytes(), gotErr)
			}
			return
		}
		if !errors.Is(gotErr, core.ErrProviderWireContract) || verificationCalls != 0 || observation.Validate() == nil || destination.Len() != 0 {
			t.Fatalf("PayPal refused header Receive() = calls:%d observation:%+v bytes:%d error:%v, want 0, zero, 0, %v", verificationCalls, observation, destination.Len(), gotErr, core.ErrProviderWireContract)
		}
	})
}

func FuzzPayPalWebhookCertificateBindingSemanticBoundary(f *testing.F) {
	f.Add("https://api.paypal.com/v1/notifications/certs/CERT-123")
	f.Add("https://api.sandbox.paypal.com/v1/notifications/certs/CERT-123")
	f.Add("http://api.paypal.com/v1/notifications/certs/CERT-123")
	f.Add("/v1/notifications/certs/CERT-123")

	f.Fuzz(func(t *testing.T, certificate string) {
		verificationCalls := uint64(0)
		receiver, token := payPalWebhookTestReceiver(t, &verificationCalls, PayPalWebhookVerificationSuccess)
		defer closePayPalWebhookTestReceiver(t, &receiver, &token)
		body := []byte(`{"id":"WH-EVENT-123"}`)
		request := payPalWebhookRequest(t, body, true)
		request.Header.Set(PayPalCertificateURLHeaderName, certificate)
		var destination bytes.Buffer
		observation, gotErr := receiver.Receive(request, &destination, providerOperationPolicy(t))
		if payPalLiveCertificateBindingOracle(certificate) {
			if gotErr != nil || verificationCalls != 1 || observation.Provider != ProviderPayPal || !bytes.Equal(destination.Bytes(), body) {
				t.Fatalf("PayPal admitted certificate Receive() = calls:%d observation:%+v body:%q error:%v, want 1, exact body, nil", verificationCalls, observation, destination.Bytes(), gotErr)
			}
			return
		}
		wantErr := error(core.ErrProviderWireBinding)
		if len(certificate) == 0 || len(certificate) > PayPalCertificateURLMaximumBytes {
			wantErr = core.ErrProviderWireAuthentication
		} else if !payPalCertificateURLOracle(certificate) {
			wantErr = core.ErrProviderWireContract
		}
		if !errors.Is(gotErr, wantErr) || verificationCalls != 0 || observation.Validate() == nil || destination.Len() != 0 {
			t.Fatalf("PayPal refused certificate Receive() = calls:%d observation:%+v bytes:%d error:%v, want 0, zero, 0, %v", verificationCalls, observation, destination.Len(), gotErr, wantErr)
		}
	})
}

func FuzzPayPalWebhookFieldParsersSemanticBoundary(f *testing.F) {
	f.Add("SHA256withRSA", "https://api.paypal.com/v1/notifications/certs/CERT-1", "transmission-123", "signature-123", "2026-08-31T20:00:00Z")
	f.Add("", "/relative", "123", "123", "not-a-time")

	f.Fuzz(func(t *testing.T, algorithm, certificate, transmissionID, signature, transmissionTime string) {
		gotAlgorithm, algorithmErr := ParsePayPalAuthAlgorithm(algorithm)
		wantAlgorithm := payPalAuthAlgorithmOracle(algorithm)
		if wantAlgorithm {
			if algorithmErr != nil || gotAlgorithm.Validate() != nil || gotAlgorithm.String() != algorithm {
				t.Fatalf("ParsePayPalAuthAlgorithm() = (%q, %v), want %q and nil", gotAlgorithm, algorithmErr, algorithm)
			}
		} else if !errors.Is(algorithmErr, core.ErrProviderWireContract) || gotAlgorithm != "" {
			t.Fatalf("ParsePayPalAuthAlgorithm(rejected) = (%q, %v), want zero and %v", gotAlgorithm, algorithmErr, core.ErrProviderWireContract)
		}

		gotCertificate, certificateErr := ParsePayPalCertificateURL(certificate)
		wantCertificate := payPalCertificateURLOracle(certificate)
		if wantCertificate {
			if certificateErr != nil || gotCertificate.Validate() != nil || gotCertificate.String() != certificate {
				t.Fatalf("ParsePayPalCertificateURL() = (%q, %v), want %q and nil", gotCertificate, certificateErr, certificate)
			}
		} else if !errors.Is(certificateErr, core.ErrProviderWireContract) || gotCertificate != "" {
			t.Fatalf("ParsePayPalCertificateURL(rejected) = (%q, %v), want zero and %v", gotCertificate, certificateErr, core.ErrProviderWireContract)
		}

		gotTransmissionID, transmissionIDErr := ParsePayPalTransmissionID(transmissionID)
		wantTransmissionID := payPalTransmissionMemberOracle(transmissionID, PayPalTransmissionIDMaximumBytes)
		if wantTransmissionID {
			if transmissionIDErr != nil || gotTransmissionID.Validate() != nil || gotTransmissionID.String() != transmissionID {
				t.Fatalf("ParsePayPalTransmissionID() = (%q, %v), want %q and nil", gotTransmissionID, transmissionIDErr, transmissionID)
			}
		} else if !errors.Is(transmissionIDErr, core.ErrProviderWireContract) || gotTransmissionID != "" {
			t.Fatalf("ParsePayPalTransmissionID(rejected) = (%q, %v), want zero and %v", gotTransmissionID, transmissionIDErr, core.ErrProviderWireContract)
		}

		gotSignature, signatureErr := ParsePayPalTransmissionSignature(signature)
		wantSignature := payPalTransmissionMemberOracle(signature, PayPalTransmissionSignatureMaximumBytes)
		if wantSignature {
			if signatureErr != nil || gotSignature.Validate() != nil || gotSignature.String() != signature {
				t.Fatalf("ParsePayPalTransmissionSignature() = (%q, %v), want %q and nil", gotSignature, signatureErr, signature)
			}
		} else if !errors.Is(signatureErr, core.ErrProviderWireContract) || gotSignature != "" {
			t.Fatalf("ParsePayPalTransmissionSignature(rejected) = (%q, %v), want zero and %v", gotSignature, signatureErr, core.ErrProviderWireContract)
		}

		gotTransmissionTime, transmissionTimeErr := ParsePayPalTransmissionTime(transmissionTime)
		wantTransmissionTime := payPalTransmissionTimeOracle(transmissionTime)
		if wantTransmissionTime {
			if transmissionTimeErr != nil || gotTransmissionTime.Validate() != nil || gotTransmissionTime.String() != transmissionTime {
				t.Fatalf("ParsePayPalTransmissionTime() = (%q, %v), want %q and nil", gotTransmissionTime, transmissionTimeErr, transmissionTime)
			}
		} else if !errors.Is(transmissionTimeErr, core.ErrProviderWireContract) || gotTransmissionTime != "" {
			t.Fatalf("ParsePayPalTransmissionTime(rejected) = (%q, %v), want zero and %v", gotTransmissionTime, transmissionTimeErr, core.ErrProviderWireContract)
		}
	})
}

func payPalAuthAlgorithmOracle(value string) bool {
	if len(value) == 0 || len(value) > PayPalAuthAlgorithmMaximumBytes {
		return false
	}
	for index := range len(value) {
		if !(value[index] >= '0' && value[index] <= '9' || value[index] >= 'A' && value[index] <= 'Z' || value[index] >= 'a' && value[index] <= 'z') {
			return false
		}
	}
	return true
}

func payPalTransmissionMemberOracle(value string, maximum int) bool {
	if len(value) < 2 || len(value) > maximum || !(value[0] >= '0' && value[0] <= '9' || value[0] >= 'A' && value[0] <= 'Z' || value[0] >= 'a' && value[0] <= 'z' || value[0] == '_') {
		return false
	}
	nondecimal := false
	for index := range len(value) {
		if value[index] < 0x21 || value[index] > 0x7e {
			return false
		}
		nondecimal = nondecimal || value[index] < '0' || value[index] > '9'
	}
	return nondecimal
}

func payPalTransmissionTimeOracle(value string) bool {
	if len(value) == 0 || len(value) > PayPalTransmissionTimeMaximumBytes {
		return false
	}
	_, err := time.Parse(time.RFC3339Nano, value)
	return err == nil
}

func payPalCertificateURLOracle(value string) bool {
	if len(value) == 0 || len(value) > PayPalCertificateURLMaximumBytes {
		return false
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed == nil || !parsed.IsAbs() || parsed.Opaque != "" || parsed.Host == "" || parsed.Hostname() == "" ||
		parsed.User != nil || parsed.Fragment != "" || parsed.RawFragment != "" {
		return false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}
	if parsed.Port() == "" {
		return true
	}
	port, err := strconv.ParseUint(parsed.Port(), 10, 16)
	return err == nil && port != 0
}

func payPalLiveCertificateBindingOracle(value string) bool {
	if !payPalCertificateURLOracle(value) {
		return false
	}
	parsed, err := url.Parse(value)
	return err == nil && parsed != nil && parsed.Scheme == "https" && parsed.Host == "api.paypal.com" && parsed.RawPath == "" &&
		parsed.RawQuery == "" && path.Clean(parsed.Path) == parsed.Path && strings.HasPrefix(parsed.Path, payPalWebhookCertificatePathPrefix)
}
