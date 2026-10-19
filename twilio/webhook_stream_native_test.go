package twilio

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
	twilioclient "github.com/twilio/twilio-go/client"
)

const nativeTwilioWebhookToken = "12345123451234512345123451234512"

func nativeTwilioSignature(t testing.TB, endpoint string, body []byte, representation WebhookRepresentation) string {
	t.Helper()
	mac := hmac.New(sha1.New, []byte(nativeTwilioWebhookToken))
	_, _ = io.WriteString(mac, endpoint)
	if representation == WebhookRepresentationForm {
		values, err := url.ParseQuery(string(body))
		if err != nil {
			t.Fatal(err)
		}
		names := make([]string, 0, len(values))
		for name := range values {
			names = append(names, name)
		}
		slices.Sort(names)
		for _, name := range names {
			_, _ = io.WriteString(mac, name)
			_, _ = io.WriteString(mac, values[name][0])
		}
	}
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func nativeTwilioReceiver(t testing.TB, endpoint string, representation WebhookRepresentation) (WebhookReceiver, AuthToken) {
	t.Helper()
	token, err := ParseAuthToken([]byte(nativeTwilioWebhookToken))
	if err != nil {
		t.Fatal(err)
	}
	public, err := core.ParseHTTPEndpoint(endpoint)
	if err != nil {
		t.Fatal(err)
	}
	receiver, err := NewWebhookReceiver(WebhookReceiverRequest{Token: token, PublicEndpoint: public, Representation: representation})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = errors.Join(receiver.Close(), token.Close()) })
	return receiver, token
}

func nativeTwilioReceive(t testing.TB, receiver WebhookReceiver, endpoint, signature, directory string, body io.Reader, destination io.Writer, representation WebhookRepresentation) (InboundObservation, error) {
	t.Helper()
	request := httptest.NewRequest(http.MethodPost, endpoint, body)
	media, err := representation.mediaType()
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", media.String())
	request.Header.Set(core.TwilioWebhookSignatureHeaderName, signature)
	call, err := exchange.NewSocketServerCall(httptest.NewRecorder(), request)
	if err != nil {
		t.Fatal(err)
	}
	parent, err := core.ParseAbsolutePath(directory)
	if err != nil {
		t.Fatal(err)
	}
	return receiver.Receive(WebhookReceiveRequest{ScratchParent: parent, Call: call, Destination: destination, ObservedAt: temporal.InstantFromNanoseconds(1)})
}

func assertNativeTwilioScratchEmpty(t testing.TB, directory string) {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 0 {
		t.Fatalf("scratch entries %d, error %v", len(entries), err)
	}
}

func TestStreamedTwilioFormMatchesOfficialSDKAcrossNativeBoundaries(t *testing.T) {
	t.Parallel()
	for index := range 64 {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			t.Parallel()
			var body []byte
			switch index % 8 {
			case 0:
				body = []byte{}
			case 1:
				body = []byte("z=&=empty-name&bare&foo=hello+world")
			case 2:
				body = []byte("%E2%98%83=%00%FF%0A&%3D=%3B%26%25%3D&%2B=+&%25=%25")
			case 3:
				body = []byte("z=line\nvalue&y=\xff\x00\r&x=a=b=c")
			case 4:
				body = []byte("z=" + strings.Repeat("v", 65537) + "&a=first")
			case 5:
				body = []byte("z=" + strings.Repeat("v", 6<<20+1) + "&a=first")
			case 6:
				var text strings.Builder
				for field := 4097; field > 0; field-- {
					fmt.Fprintf(&text, "k%08d=v&", field)
				}
				body = []byte(text.String())
			default:
				body = []byte("&&z=tail&&a=first&")
			}
			endpoint := fmt.Sprintf("https://example.com/twilio/%d?foo=1&bar=2", index)
			if index/8%2 == 1 {
				endpoint = strings.Replace(endpoint, "example.com", "example.com:443", 1)
			}
			signed := endpoint
			if index/8%4 == 2 {
				signed = strings.Replace(endpoint, "example.com", "example.com:443", 1)
			}
			if index/8%4 == 3 {
				signed = strings.Replace(endpoint, "example.com:443", "example.com", 1)
			}
			signature := nativeTwilioSignature(t, signed, body, WebhookRepresentationForm)
			validator := twilioclient.NewRequestValidator(nativeTwilioWebhookToken)
			if !validator.ValidateBody(endpoint, body, signature) {
				t.Fatal("official SDK refuses the genuine fixture")
			}
			receiver, _ := nativeTwilioReceiver(t, endpoint, WebhookRepresentationForm)
			directory := t.TempDir()
			var destination bytes.Buffer
			observation, err := nativeTwilioReceive(t, receiver, endpoint, signature, directory, bytes.NewReader(body), &destination, WebhookRepresentationForm)
			if err != nil || observation.Bytes.Uint64() != uint64(len(body)) || !bytes.Equal(destination.Bytes(), body) {
				t.Fatalf("native form handoff %d/%v, exact bytes %t", observation.Bytes.Uint64(), err, bytes.Equal(destination.Bytes(), body))
			}
			assertNativeTwilioScratchEmpty(t, directory)
			mutated := append(bytes.Clone(body), []byte("&new=unsigned")...)
			if validator.ValidateBody(endpoint, mutated, signature) {
				t.Fatal("official SDK accepted unsigned extra field")
			}
			destination.Reset()
			observation, err = nativeTwilioReceive(t, receiver, endpoint, signature, directory, bytes.NewReader(mutated), &destination, WebhookRepresentationForm)
			if !errors.Is(err, core.ErrTwilioVerification) || destination.Len() != 0 || observation.Validate() == nil {
				t.Fatalf("unsigned field released %d bytes, observation %+v, error %v", destination.Len(), observation, err)
			}
			assertNativeTwilioScratchEmpty(t, directory)
		})
	}
}

func TestStreamedTwilioJSONPreservesRawDigestAndQueryBinding(t *testing.T) {
	t.Parallel()
	for index := range 64 {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			t.Parallel()
			body := []byte(fmt.Sprintf(" {\n \"event\": %d, \"url\": \"https:\\/\\/example.com\", \"data\": \"%s\" }\n", index, strings.Repeat("v", [...]int{0, 1, 4095, 4096, 4097, 65537, 1 << 20, 6<<20 + 1}[index%8])))
			digest := sha256.Sum256(body)
			configured := fmt.Sprintf("https://example.com/json/%d?x=first&x=second&space=hello+world", index)
			observed := strings.Split(configured, "?")[0] + "?space=hello%20world&x=first&bodySHA256=" + hex.EncodeToString(digest[:]) + "&x=second"
			signature := nativeTwilioSignature(t, observed, body, WebhookRepresentationJSON)
			validator := twilioclient.NewRequestValidator(nativeTwilioWebhookToken)
			if !validator.ValidateBody(observed, body, signature) {
				t.Fatal("official SDK refuses genuine JSON fixture")
			}
			receiver, _ := nativeTwilioReceiver(t, configured, WebhookRepresentationJSON)
			directory := t.TempDir()
			var destination bytes.Buffer
			observation, err := nativeTwilioReceive(t, receiver, observed, signature, directory, bytes.NewReader(body), &destination, WebhookRepresentationJSON)
			if err != nil || !bytes.Equal(destination.Bytes(), body) || observation.Bytes.Uint64() != uint64(len(body)) {
				t.Fatalf("native JSON %d/%v, exact bytes %t", observation.Bytes.Uint64(), err, bytes.Equal(destination.Bytes(), body))
			}
			assertNativeTwilioScratchEmpty(t, directory)
			destination.Reset()
			mutated := append(bytes.Clone(body), ' ')
			observation, err = nativeTwilioReceive(t, receiver, observed, signature, directory, bytes.NewReader(mutated), &destination, WebhookRepresentationJSON)
			if !errors.Is(err, core.ErrTwilioVerification) || destination.Len() != 0 || observation.Validate() == nil {
				t.Fatalf("raw digest mutation released %d bytes: %+v/%v", destination.Len(), observation, err)
			}
			assertNativeTwilioScratchEmpty(t, directory)
			foreign := strings.Replace(observed, "x=first", "x=foreign", 1)
			foreignSignature := nativeTwilioSignature(t, foreign, body, WebhookRepresentationJSON)
			_, err = nativeTwilioReceive(t, receiver, foreign, foreignSignature, directory, bytes.NewReader(body), &destination, WebhookRepresentationJSON)
			if !errors.Is(err, core.ErrTwilioBinding) || destination.Len() != 0 {
				t.Fatalf("validly signed foreign query released %d bytes: %v", destination.Len(), err)
			}
		})
	}
}

func FuzzStreamedTwilioFormMatchesOfficialSDK(f *testing.F) {
	endpoint := "https://example.com/twilio?configured=1"
	for _, body := range [][]byte{nil, []byte("z=last&a=first"), []byte("%E2%98%83=%0A%00%FF"), []byte("a=first&a=second"), []byte("invalid=%"), []byte("x=raw\nvalue")} {
		f.Add(body, []byte{}, true)
		f.Add(body, []byte("invalid signature"), false)
	}
	f.Fuzz(func(t *testing.T, body, candidate []byte, genuine bool) {
		values, parseErr := url.ParseQuery(string(body))
		signature := string(candidate)
		if genuine && parseErr == nil {
			signature = nativeTwilioSignature(t, endpoint, body, WebhookRepresentationForm)
		}
		validator := twilioclient.NewRequestValidator(nativeTwilioWebhookToken)
		want := validator.ValidateBody(endpoint, body, signature)
		for _, entries := range values {
			// The nominal contract rejects duplicate decoded names rather than
			// authenticating the SDK's first-value-only projection.
			if len(entries) != 1 {
				want = false
			}
		}
		receiver, _ := nativeTwilioReceiver(t, endpoint, WebhookRepresentationForm)
		directory := t.TempDir()
		var destination bytes.Buffer
		observation, err := nativeTwilioReceive(t, receiver, endpoint, signature, directory, bytes.NewReader(body), &destination, WebhookRepresentationForm)
		if want {
			if err != nil || !bytes.Equal(destination.Bytes(), body) || observation.Bytes.Uint64() != uint64(len(body)) {
				t.Fatalf("official SDK accepted, native handoff %+v/%v, exact %t", observation, err, bytes.Equal(destination.Bytes(), body))
			}
		} else if err == nil || destination.Len() != 0 || observation.Validate() == nil {
			t.Fatalf("official SDK/nominal coverage refused, native handoff %+v/%v, released %d", observation, err, destination.Len())
		}
		assertNativeTwilioScratchEmpty(t, directory)
	})
}

type nativeTwilioCountingReader struct {
	Reader *bytes.Reader
	Reads  int
}

func (r *nativeTwilioCountingReader) Read(destination []byte) (int, error) {
	r.Reads++
	return r.Reader.Read(destination)
}

func (r *nativeTwilioCountingReader) Len() int { return r.Reader.Len() }

type nativeTwilioFailingReader struct{ Reader *bytes.Reader }

func (r nativeTwilioFailingReader) Read(destination []byte) (int, error) {
	count, err := r.Reader.Read(destination)
	if r.Reader.Len() == 0 {
		return count, errors.Join(io.EOF, io.ErrUnexpectedEOF)
	}
	return count, err
}

func (r nativeTwilioFailingReader) Len() int { return r.Reader.Len() }

func TestTwilioCloseZeroesSharedCustodyAndRefusesCopiedReceiverBeforeIngress(t *testing.T) {
	t.Parallel()
	for index := range 64 {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			t.Parallel()
			endpoint := fmt.Sprintf("https://example.com/closed/%d", index)
			receiver, _ := nativeTwilioReceiver(t, endpoint, WebhookRepresentationForm)
			copied := receiver
			custody := receiver.state.token.value
			body := []byte(fmt.Sprintf("index=%d", index))
			signature := nativeTwilioSignature(t, endpoint, body, WebhookRepresentationForm)
			if err := receiver.Close(); err != nil {
				t.Fatal(err)
			}
			for _, value := range custody {
				if value != 0 {
					t.Fatal("owned native token custody survives Close")
				}
			}
			directory := t.TempDir()
			reader := &nativeTwilioCountingReader{Reader: bytes.NewReader(body)}
			var destination bytes.Buffer
			observation, err := nativeTwilioReceive(t, copied, endpoint, signature, directory, reader, &destination, WebhookRepresentationForm)
			if err == nil || observation.Validate() == nil || reader.Reads != 0 || destination.Len() != 0 || copied.Validate() == nil {
				t.Fatalf("closed shared copy: %+v/%v reads%d released%d", observation, err, reader.Reads, destination.Len())
			}
			assertNativeTwilioScratchEmpty(t, directory)
		})
	}
}

func TestTwilioCompleteBytesWithNativeReadFailureReleaseNothing(t *testing.T) {
	t.Parallel()
	for index := range 64 {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			t.Parallel()
			endpoint := fmt.Sprintf("https://example.com/read-error/%d", index)
			body := []byte("index=" + fmt.Sprint(index) + "&value=" + strings.Repeat("v", [...]int{0, 1, 4095, 4096, 4097, 32767, 32768, 32769}[index%8]))
			signature := nativeTwilioSignature(t, endpoint, body, WebhookRepresentationForm)
			receiver, _ := nativeTwilioReceiver(t, endpoint, WebhookRepresentationForm)
			directory := t.TempDir()
			var destination bytes.Buffer
			observation, err := nativeTwilioReceive(t, receiver, endpoint, signature, directory, nativeTwilioFailingReader{Reader: bytes.NewReader(body)}, &destination, WebhookRepresentationForm)
			if !errors.Is(err, io.ErrUnexpectedEOF) || destination.Len() != 0 || observation.Validate() == nil {
				t.Fatalf("native complete-byte read error: %+v/%v, released%d", observation, err, destination.Len())
			}
			assertNativeTwilioScratchEmpty(t, directory)
		})
	}
}

func FuzzStreamedTwilioJSONMatchesOfficialSDK(f *testing.F) {
	for _, body := range [][]byte{nil, []byte(" {\n\"url\":\"https:\\/\\/example.com\"}\n"), {0, 255, '\n'}, []byte("{} ")} {
		f.Add(body, []byte("changed"), true)
		f.Add(body, []byte("changed"), false)
	}
	f.Fuzz(func(t *testing.T, signedBody, candidate []byte, genuine bool) {
		configured := "https://example.com/json-fuzz?x=first&x=second"
		digest := sha256.Sum256(signedBody)
		observed := configured + "&bodySHA256=" + hex.EncodeToString(digest[:])
		signature := nativeTwilioSignature(t, observed, signedBody, WebhookRepresentationJSON)
		body := candidate
		if genuine {
			body = signedBody
		}
		validator := twilioclient.NewRequestValidator(nativeTwilioWebhookToken)
		want := validator.ValidateBody(observed, body, signature)
		receiver, _ := nativeTwilioReceiver(t, configured, WebhookRepresentationJSON)
		directory := t.TempDir()
		var destination bytes.Buffer
		observation, err := nativeTwilioReceive(t, receiver, observed, signature, directory, bytes.NewReader(body), &destination, WebhookRepresentationJSON)
		if want {
			if err != nil || observation.Bytes.Uint64() != uint64(len(body)) || !bytes.Equal(destination.Bytes(), body) {
				t.Fatalf("native JSON SDK agreement %+v/%v", observation, err)
			}
		} else if !errors.Is(err, core.ErrTwilioVerification) || observation.Validate() == nil || destination.Len() != 0 {
			t.Fatalf("invalid raw JSON digest released %+v/%v bytes%d", observation, err, destination.Len())
		}
		assertNativeTwilioScratchEmpty(t, directory)
	})
}

func FuzzStreamedTwilioQueryBindingMatchesNativeURLSemantics(f *testing.F) {
	for _, query := range []string{"", "x=first&x=second&space=hello+world", "%00=%FF%0A&%3D=%26", "x=%", "x=a;b", "&&bare&=empty&&"} {
		f.Add(query, query, true)
		f.Add(query, "x=foreign", false)
	}
	f.Fuzz(func(t *testing.T, configured, candidate string, equivalent bool) {
		left, leftErr := url.ParseQuery(configured)
		if equivalent && leftErr == nil {
			candidate = left.Encode()
		}
		right, rightErr := url.ParseQuery(candidate)
		_, leftReserved := left[core.TwilioWebhookBodySHA256QueryName]
		_, rightReserved := right[core.TwilioWebhookBodySHA256QueryName]
		leftValid := leftErr == nil && !leftReserved
		rightValid := rightErr == nil && !rightReserved
		_, err := validateWebhookQuery(context.Background(), configured, false)
		if (err == nil) != leftValid {
			t.Fatalf("configured query admission %v, native validity %t", err, leftValid)
		}
		_, err = validateWebhookQuery(context.Background(), candidate, false)
		if (err == nil) != rightValid {
			t.Fatalf("observed query admission %v, native validity %t", err, rightValid)
		}
		if !leftValid || !rightValid {
			return // Both full native grammars were checked; comparison requires admitted fields.
		}
		observed := candidate + "&" + core.TwilioWebhookBodySHA256QueryName + "=" + strings.Repeat("a", 64)
		if _, err := validateWebhookQuery(context.Background(), observed, true); err != nil {
			t.Fatal(err)
		}
		got, err := sameWebhookQueries(context.Background(), configured, observed)
		// Empty segments have no values; Native ParseQuery normalizes both to non-nil maps.
		want := reflect.DeepEqual(left, right)
		if err != nil || got != want {
			t.Fatalf("streamed binding %t/%v, native query equality %t", got, err, want)
		}
	})
}

func TestTwilioIPv6NativeURLPortSpellingAndSignatureBinding(t *testing.T) {
	t.Parallel()
	for index := range 64 {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			t.Parallel()
			host := fmt.Sprintf("[2001:db8::%x]", index+1)
			endpoint := "https://" + host + "/callback"
			port := [...]string{"443", "8443"}[index%2]
			if index%4 >= 2 {
				endpoint = "https://" + host + ":" + port + "/callback"
			}
			signed := endpoint
			if index%8 >= 4 {
				if index%4 >= 2 {
					signed = "https://" + host + "/callback"
				} else {
					signed = "https://" + host + ":443/callback"
				}
			}
			body := []byte(fmt.Sprintf("index=%d&name=hello+world", index))
			signature := nativeTwilioSignature(t, signed, body, WebhookRepresentationForm)
			receiver, _ := nativeTwilioReceiver(t, endpoint, WebhookRepresentationForm)
			directory := t.TempDir()
			var destination bytes.Buffer
			observation, err := nativeTwilioReceive(t, receiver, endpoint, signature, directory, bytes.NewReader(body), &destination, WebhookRepresentationForm)
			if err != nil || observation.Bytes.Uint64() != uint64(len(body)) || !bytes.Equal(destination.Bytes(), body) {
				t.Fatalf("native IPv6 port binding %+v/%v", observation, err)
			}
			foreign := nativeTwilioSignature(t, strings.Replace(signed, host, "[2001:db8::ffff]", 1), body, WebhookRepresentationForm)
			destination.Reset()
			observation, err = nativeTwilioReceive(t, receiver, endpoint, foreign, directory, bytes.NewReader(body), &destination, WebhookRepresentationForm)
			if !errors.Is(err, core.ErrTwilioVerification) || destination.Len() != 0 || observation.Validate() == nil {
				t.Fatalf("foreign IPv6 host signature released bytes %+v/%v", observation, err)
			}
			assertNativeTwilioScratchEmpty(t, directory)
		})
	}
}

func BenchmarkStreamedTwilioWebhookBodyMemory(b *testing.B) {
	for _, extent := range [...]int{4096, 8 << 20, 64 << 20} {
		b.Run(fmt.Sprint(extent), func(b *testing.B) {
			endpoint := "https://example.com/benchmark"
			body := append([]byte("value="), bytes.Repeat([]byte{'v'}, extent)...)
			signature := nativeTwilioSignature(b, endpoint, body, WebhookRepresentationForm)
			receiver, _ := nativeTwilioReceiver(b, endpoint, WebhookRepresentationForm)
			directory := b.TempDir()
			fixture := filepath.Join(b.TempDir(), "native-body")
			if err := os.WriteFile(fixture, body, 0600); err != nil {
				b.Fatal(err)
			}
			bytesTotal := int64(len(body))
			body = nil
			b.ReportAllocs()
			b.SetBytes(bytesTotal)
			b.ResetTimer()
			for range b.N {
				source, err := os.Open(fixture)
				if err != nil {
					b.Fatal(err)
				}
				observation, err := nativeTwilioReceive(b, receiver, endpoint, signature, directory, source, io.Discard, WebhookRepresentationForm)
				// Exchange owns and closes the HTTP request body, including native files.
				if err != nil || observation.Bytes.Uint64() != uint64(bytesTotal) {
					b.Fatalf("native streamed benchmark %+v/%v", observation, err)
				}
			}
		})
	}
}

// BenchmarkStreamedTwilioWebhookTypicalForm receives the shape Twilio actually
// posts: a few dozen short percent-encoded fields rather than one large value.
func BenchmarkStreamedTwilioWebhookTypicalForm(b *testing.B) {
	endpoint := "https://example.com/benchmark"
	var form strings.Builder
	for index := range 24 {
		if index > 0 {
			form.WriteByte('&')
		}
		fmt.Fprintf(&form, "Field%02d=value+%d%%2Fpart", index, index)
	}
	body := []byte(form.String())
	signature := nativeTwilioSignature(b, endpoint, body, WebhookRepresentationForm)
	receiver, _ := nativeTwilioReceiver(b, endpoint, WebhookRepresentationForm)
	directory := b.TempDir()
	b.ReportAllocs()
	b.SetBytes(int64(len(body)))
	var destination bytes.Buffer
	for b.Loop() {
		destination.Reset()
		observation, err := nativeTwilioReceive(b, receiver, endpoint, signature, directory, bytes.NewReader(body), &destination, WebhookRepresentationForm)
		if err != nil || observation.Bytes.Uint64() != uint64(len(body)) {
			b.Fatalf("typical form receive %+v/%v", observation, err)
		}
	}
	if !bytes.Equal(destination.Bytes(), body) {
		b.Fatalf("released body = %q, want %q", destination.Bytes(), body)
	}
}
