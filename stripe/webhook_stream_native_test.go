package stripe

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
	stripesdk "github.com/stripe/stripe-go/v86"
)

func stripeScratchParent(t testing.TB, directory string) core.AbsolutePath {
	t.Helper()
	parent, err := core.ParseAbsolutePath(directory)
	if err != nil {
		t.Fatal(err)
	}
	return parent
}

type stripeFailureReader struct{ source *bytes.Reader }

func (r stripeFailureReader) Read(data []byte) (int, error) {
	count, err := r.source.Read(data)
	if count != 0 {
		return count, io.ErrUnexpectedEOF
	}
	return count, err
}

func TestStripeCloseZeroesSharedCustodyAndRefusesCopiedReceivers(t *testing.T) {
	t.Parallel()
	for index := range 64 {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			t.Parallel()
			receiver, secret := stripeWebhookTestReceiver(t)
			defer func() {
				if err := secret.Close(); err != nil {
					t.Error(err)
				}
			}()
			copyReceiver := receiver
			custody := receiver.state.secret.value
			if err := receiver.Close(); err != nil {
				t.Fatal(err)
			}
			for _, value := range custody {
				if value != 0 {
					t.Fatal("Close retained owned secret bytes")
				}
			}
			if !errors.Is(copyReceiver.Validate(), core.ErrStripeContract) {
				t.Fatal("copied receiver stayed authenticated after Close")
			}
			body := bytes.NewReader(bytes.Repeat([]byte{byte(index)}, index+1))
			request := httptest.NewRequest(http.MethodPost, "https://example.test/stripe", body)
			request.Header.Set("Content-Type", "application/json")
			call, err := exchange.NewSocketServerCall(httptest.NewRecorder(), request)
			if err != nil {
				t.Fatal(err)
			}
			tolerance, err := temporal.DurationFromSeconds(1)
			if err != nil {
				t.Fatal(err)
			}
			var destination bytes.Buffer
			observation, err := copyReceiver.Receive(WebhookReceiveRequest{ScratchParent: stripeScratchParent(t, t.TempDir()), Call: call, Destination: &destination, ObservedAt: temporal.InstantFromNanoseconds(1), Tolerance: tolerance})
			if !errors.Is(err, core.ErrStripeContract) || observation != (InboundObservation{}) || destination.Len() != 0 || body.Len() != index+1 {
				t.Fatal("closed receiver crossed native ingress")
			}
		})
	}
}

func TestStreamedStripeSourceFailureWithCompleteBytesReleasesNothing(t *testing.T) {
	t.Parallel()
	for index := range 64 {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			t.Parallel()
			receiver, secret := stripeWebhookTestReceiver(t)
			defer func() {
				if err := errors.Join(receiver.Close(), secret.Close()); err != nil {
					t.Error(err)
				}
			}()
			body := bytes.Repeat([]byte{byte(index)}, index+1)
			signed := stripesdk.GenerateTestSignedPayload(&stripesdk.UnsignedPayload{Payload: body, Secret: "whsec_test_123", Timestamp: time.Unix(1_800_000_000, 0)})
			parent := stripeScratchParent(t, t.TempDir())
			request := httptest.NewRequest(http.MethodPost, "https://example.test/stripe", stripeFailureReader{source: bytes.NewReader(body)})
			request.Header.Set("Content-Type", "application/json")
			request.Header.Set(core.StripeWebhookSignatureHeaderName, signed.Header)
			call, err := exchange.NewSocketServerCall(httptest.NewRecorder(), request)
			if err != nil {
				t.Fatal(err)
			}
			tolerance, err := temporal.DurationFromSeconds(1)
			if err != nil {
				t.Fatal(err)
			}
			var destination bytes.Buffer
			observation, err := receiver.Receive(WebhookReceiveRequest{ScratchParent: parent, Call: call, Destination: &destination, ObservedAt: temporal.InstantFromNanoseconds(1_800_000_000 * int64(temporal.NanosecondsPerSecond)), Tolerance: tolerance})
			if !errors.Is(err, io.ErrUnexpectedEOF) || observation != (InboundObservation{}) || destination.Len() != 0 {
				t.Fatalf("native source failure escaped authentication: %v", err)
			}
			entries, err := os.ReadDir(parent.String())
			if err != nil || len(entries) != 0 {
				t.Fatalf("failed source left scratch residue: %v", err)
			}
		})
	}
}

func TestStreamedStripeWebhookMatchesOfficialSDKAndWithholdsRefusals(t *testing.T) {
	t.Parallel()
	for index := range 64 {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			t.Parallel()
			size := [...]int{1, 63, 4095, 4096, 4097, 65536, 1 << 20, (6 << 20) + 1}[index%8]
			body := bytes.Repeat([]byte{byte(index)}, size)
			secretText := "whsec_" + strings.Repeat(strconv.Itoa(index), 32)
			secret, err := ParseWebhookSecret([]byte(secretText))
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := secret.Close(); err != nil {
					t.Error(err)
				}
			}()
			receiver, err := NewWebhookReceiver(secret)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := receiver.Close(); err != nil {
					t.Error(err)
				}
			}()
			seconds := int64(1_800_000_000 + index)
			signed := stripesdk.GenerateTestSignedPayload(&stripesdk.UnsignedPayload{Payload: body, Secret: secretText, Timestamp: time.Unix(seconds, 0)})
			parent := stripeScratchParent(t, t.TempDir())
			tolerance, err := temporal.DurationFromSeconds(1)
			if err != nil {
				t.Fatal(err)
			}
			for variant := range 4 {
				header := signed.Header
				wantErr := error(nil)
				switch variant {
				case 1:
					header += ",v1=broken,v1=" + strings.Repeat("0", 64)
				case 2:
					header = "t=" + strconv.FormatInt(seconds, 10) + ",v1=" + strings.Repeat("0", 64)
					wantErr = core.ErrStripeVerification
				case 3:
					header += ",t=" + strconv.FormatInt(seconds, 10)
					wantErr = core.ErrStripeVerification
				}
				if variant != 3 {
					oracle := stripesdk.ValidatePayload(body, header, secretText, stripesdk.WithIgnoreTolerance())
					if (oracle == nil) != (wantErr == nil) {
						t.Fatalf("SDK oracle disagrees: %v", oracle)
					}
				}
				request := httptest.NewRequest(http.MethodPost, "https://example.test/stripe", bytes.NewReader(body))
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set(core.StripeWebhookSignatureHeaderName, header)
				call, err := exchange.NewSocketServerCall(httptest.NewRecorder(), request)
				if err != nil {
					t.Fatal(err)
				}
				var destination bytes.Buffer
				observation, err := receiver.Receive(WebhookReceiveRequest{ScratchParent: parent, Call: call, Destination: &destination, ObservedAt: temporal.InstantFromNanoseconds(seconds * int64(temporal.NanosecondsPerSecond)), Tolerance: tolerance})
				if !errors.Is(err, wantErr) {
					t.Fatalf("variant %d: %v", variant, err)
				}
				if wantErr != nil {
					if observation != (InboundObservation{}) || destination.Len() != 0 {
						t.Fatal("unauthenticated bytes escaped")
					}
				} else if observation.Bytes.Uint64() != uint64(len(body)) || !bytes.Equal(destination.Bytes(), body) {
					t.Fatal("authenticated native bytes changed")
				}
				entries, err := os.ReadDir(parent.String())
				if err != nil || len(entries) != 0 {
					t.Fatalf("scratch residue: %d/%v", len(entries), err)
				}
			}
		})
	}
}

func FuzzStripeSignatureDigestMatchesOfficialSDK(f *testing.F) {
	seedBody := []byte("raw body")
	seed := stripesdk.GenerateTestSignedPayload(&stripesdk.UnsignedPayload{Payload: seedBody, Secret: "whsec_test_fuzz", Timestamp: time.Unix(1_800_000_000, 0)})
	f.Add(seedBody, seed.Header)
	f.Add([]byte{}, "")
	f.Fuzz(func(t *testing.T, body []byte, header string) {
		seconds, nativeErr := signatureSeconds(header)
		wantErr := stripesdk.ValidatePayload(body, header, "whsec_test_fuzz", stripesdk.WithIgnoreTolerance())
		if nativeErr != nil {
			if !errors.Is(nativeErr, core.ErrStripeVerification) {
				t.Fatalf("header refusal lost typed identity: %v", nativeErr)
			}
			// Primitive deliberately refuses ambiguous timestamps even where the
			// SDK would choose the last timestamp. Other refusals must agree.
			timestamps := 0
			for member := range strings.SplitSeq(header, ",") {
				if strings.HasPrefix(member, core.StripeWebhookTimestampField+"=") {
					timestamps++
				}
			}
			if wantErr == nil && timestamps <= 1 {
				t.Fatal("native parser refused an unambiguous authenticated SDK header")
			}
			return
		}
		expected := stripesdk.ComputeSignature(time.Unix(seconds, 0), body, "whsec_test_fuzz")
		gotErr := verifySignatureDigest(header, expected)
		if (gotErr == nil) != (wantErr == nil) {
			t.Fatalf("native signature disagreement: %v/%v", gotErr, wantErr)
		}
		if gotErr != nil && !errors.Is(gotErr, core.ErrStripeVerification) {
			t.Fatalf("digest refusal lost typed identity: %v", gotErr)
		}
	})
}

func BenchmarkStreamedStripeWebhookBodyMemory(b *testing.B) {
	for _, size := range [...]int{4096, 8 << 20, 64 << 20} {
		b.Run(strconv.Itoa(size), func(b *testing.B) {
			body := bytes.Repeat([]byte{0x5a}, size)
			secret, err := ParseWebhookSecret([]byte("whsec_native_benchmark"))
			if err != nil {
				b.Fatal(err)
			}
			defer func() {
				if err := secret.Close(); err != nil {
					b.Error(err)
				}
			}()
			receiver, err := NewWebhookReceiver(secret)
			if err != nil {
				b.Fatal(err)
			}
			defer func() {
				if err := receiver.Close(); err != nil {
					b.Error(err)
				}
			}()
			signed := stripesdk.GenerateTestSignedPayload(&stripesdk.UnsignedPayload{Payload: body, Secret: "whsec_native_benchmark", Timestamp: time.Unix(1_800_000_000, 0)})
			parent := stripeScratchParent(b, b.TempDir())
			tolerance, err := temporal.DurationFromSeconds(1)
			if err != nil {
				b.Fatal(err)
			}
			b.SetBytes(int64(size))
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				request := httptest.NewRequest(http.MethodPost, "https://example.test/stripe", bytes.NewReader(body))
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set(core.StripeWebhookSignatureHeaderName, signed.Header)
				call, err := exchange.NewSocketServerCall(httptest.NewRecorder(), request)
				if err != nil {
					b.Fatal(err)
				}
				observation, err := receiver.Receive(WebhookReceiveRequest{ScratchParent: parent, Call: call, Destination: io.Discard, ObservedAt: temporal.InstantFromNanoseconds(1_800_000_000 * int64(temporal.NanosecondsPerSecond)), Tolerance: tolerance})
				if err != nil || observation.Bytes.Uint64() != uint64(size) {
					b.Fatalf("native streamed benchmark: %v", err)
				}
			}
		})
	}
}
