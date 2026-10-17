package loops

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

const (
	nativeWebhookSeconds = int64(1_800_000_000)
	nativeWebhookID      = "msg_2KWPBgLlAfxdpx2AI54pPJ85f4W"
)

var (
	nativeWebhookKey      = []byte("loops-native-standard-webhooks-key")
	nativeWebhookRotated  = []byte("loops-previous-secret-before-rotation")
	nativeWebhookSecret   = "whsec_" + base64.StdEncoding.EncodeToString(nativeWebhookKey)
	nativeWebhookTSString = strconv.FormatInt(nativeWebhookSeconds, 10)
)

// nativeSignature is the independent stdlib oracle for the Standard Webhooks
// signed content "{id}.{timestamp}.{raw body}".
func nativeSignature(key []byte, id, timestamp string, body []byte) string {
	mac := hmac.New(sha256.New, key)
	_, _ = io.WriteString(mac, id+"."+timestamp+".")
	_, _ = mac.Write(body)
	return core.LoopsWebhookSignatureVersion + "," + base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func nativeHeaders(signature string) loopsWebhookHeaders {
	return loopsWebhookHeaders{id: []string{nativeWebhookID}, timestamp: []string{nativeWebhookTSString}, signature: []string{signature}, contentType: "application/json"}
}

type nativeCountingReader struct {
	reader *bytes.Reader
	reads  int
}

func (r *nativeCountingReader) Read(data []byte) (int, error) {
	r.reads++
	return r.reader.Read(data)
}

type nativeFailingReader struct{ reader *bytes.Reader }

func (r nativeFailingReader) Read(data []byte) (int, error) {
	count, err := r.reader.Read(data)
	if r.reader.Len() == 0 {
		return count, errors.Join(io.EOF, io.ErrUnexpectedEOF)
	}
	return count, err
}

func TestStandardWebhooksSignatureAndToleranceMatrixLayerTriad(t *testing.T) {
	t.Parallel()
	body := []byte(`{"eventName":"contact.created","eventTime":1800000000,"webhookSchemaVersion":"1.0.0"}`)
	valid := nativeSignature(nativeWebhookKey, nativeWebhookID, nativeWebhookTSString, body)
	rotated := nativeSignature(nativeWebhookRotated, nativeWebhookID, nativeWebhookTSString, body)
	foreignKey := nativeSignature([]byte("foreign"), nativeWebhookID, nativeWebhookTSString, body)
	digest := strings.TrimPrefix(valid, "v1,")
	at := func(offset int64) string { return strconv.FormatInt(nativeWebhookSeconds+offset, 10) }
	signedAt := func(offset int64) string {
		return nativeSignature(nativeWebhookKey, nativeWebhookID, at(offset), body)
	}
	for _, tc := range []struct {
		wantErr error
		name    string
		headers loopsWebhookHeaders
		body    []byte
	}{
		{name: "genuine signature", headers: nativeHeaders(valid), body: body},
		{name: "rotation previous secret first", headers: nativeHeaders(rotated + " " + valid), body: body},
		{name: "rotation current secret first", headers: nativeHeaders(valid + " " + rotated), body: body},
		{name: "future version entries are ignored", headers: nativeHeaders("v2,opaque-future " + valid + " v1a,zzz"), body: body},
		{name: "exact past tolerance", headers: loopsWebhookHeaders{id: []string{nativeWebhookID}, timestamp: []string{at(-core.LoopsWebhookToleranceSeconds)}, signature: []string{signedAt(-core.LoopsWebhookToleranceSeconds)}, contentType: "application/json"}, body: body},
		{name: "exact future tolerance", headers: loopsWebhookHeaders{id: []string{nativeWebhookID}, timestamp: []string{at(core.LoopsWebhookToleranceSeconds)}, signature: []string{signedAt(core.LoopsWebhookToleranceSeconds)}, contentType: "application/json"}, body: body},
		{name: "one second beyond past tolerance", headers: loopsWebhookHeaders{id: []string{nativeWebhookID}, timestamp: []string{at(-core.LoopsWebhookToleranceSeconds - 1)}, signature: []string{signedAt(-core.LoopsWebhookToleranceSeconds - 1)}, contentType: "application/json"}, body: body, wantErr: core.ErrLoopsVerification},
		{name: "one second beyond future tolerance", headers: loopsWebhookHeaders{id: []string{nativeWebhookID}, timestamp: []string{at(core.LoopsWebhookToleranceSeconds + 1)}, signature: []string{signedAt(core.LoopsWebhookToleranceSeconds + 1)}, contentType: "application/json"}, body: body, wantErr: core.ErrLoopsVerification},
		{name: "wrong secret", headers: nativeHeaders(foreignKey), body: body, wantErr: core.ErrLoopsVerification},
		{name: "rotation entries both foreign", headers: nativeHeaders(foreignKey + " " + rotated), body: body, wantErr: core.ErrLoopsVerification},
		{name: "tampered body", headers: nativeHeaders(valid), body: append(bytes.Clone(body), ' '), wantErr: core.ErrLoopsVerification},
		{name: "truncated body", headers: nativeHeaders(valid), body: body[:len(body)-1], wantErr: core.ErrLoopsVerification},
		{name: "tampered delivery id", headers: loopsWebhookHeaders{id: []string{nativeWebhookID + "x"}, timestamp: []string{nativeWebhookTSString}, signature: []string{valid}, contentType: "application/json"}, body: body, wantErr: core.ErrLoopsVerification},
		{name: "tampered timestamp inside tolerance", headers: loopsWebhookHeaders{id: []string{nativeWebhookID}, timestamp: []string{at(1)}, signature: []string{valid}, contentType: "application/json"}, body: body, wantErr: core.ErrLoopsVerification},
		{name: "only future versions", headers: nativeHeaders("v2," + digest), body: body, wantErr: core.ErrLoopsVerification},
		{name: "entry without separator", headers: nativeHeaders(valid + " v1"), body: body, wantErr: core.ErrLoopsVerification},
		{name: "empty version", headers: nativeHeaders(valid + " ," + digest), body: body, wantErr: core.ErrLoopsVerification},
		{name: "empty v1 value", headers: nativeHeaders(valid + " v1,"), body: body, wantErr: core.ErrLoopsVerification},
		{name: "double space empty entry", headers: nativeHeaders(rotated + "  " + valid), body: body, wantErr: core.ErrLoopsVerification},
		{name: "trailing space empty entry", headers: nativeHeaders(valid + " "), body: body, wantErr: core.ErrLoopsVerification},
		{name: "short v1 digest beside genuine", headers: nativeHeaders(valid + " v1," + digest[:43]), body: body, wantErr: core.ErrLoopsVerification},
		{name: "unpadded v1 digest", headers: nativeHeaders("v1," + strings.TrimRight(digest, "=")), body: body, wantErr: core.ErrLoopsVerification},
		{name: "URL alphabet v1 digest", headers: nativeHeaders("v1," + strings.NewReplacer("+", "-", "/", "_").Replace(digest)), body: body, wantErr: core.ErrLoopsVerification},
		{name: "hex is not the signature encoding", headers: nativeHeaders("v1," + strings.Repeat("a", 64)), body: body, wantErr: core.ErrLoopsVerification},
		{name: "comma inside v1 value", headers: nativeHeaders("v1," + digest + ",x"), body: body, wantErr: core.ErrLoopsVerification},
		{name: "signed content separator in id", headers: loopsWebhookHeaders{id: []string{"msg.1"}, timestamp: []string{nativeWebhookTSString}, signature: []string{valid}, contentType: "application/json"}, body: body, wantErr: core.ErrLoopsVerification},
		{name: "non-canonical timestamp", headers: loopsWebhookHeaders{id: []string{nativeWebhookID}, timestamp: []string{"0" + nativeWebhookTSString}, signature: []string{nativeSignature(nativeWebhookKey, nativeWebhookID, "0"+nativeWebhookTSString, body)}, contentType: "application/json"}, body: body, wantErr: core.ErrLoopsVerification},
		{name: "fractional timestamp", headers: loopsWebhookHeaders{id: []string{nativeWebhookID}, timestamp: []string{nativeWebhookTSString + ".0"}, signature: []string{valid}, contentType: "application/json"}, body: body, wantErr: core.ErrLoopsVerification},
		{name: "missing id", headers: loopsWebhookHeaders{timestamp: []string{nativeWebhookTSString}, signature: []string{valid}, contentType: "application/json"}, body: body, wantErr: core.ErrLoopsAuthentication},
		{name: "missing timestamp", headers: loopsWebhookHeaders{id: []string{nativeWebhookID}, signature: []string{valid}, contentType: "application/json"}, body: body, wantErr: core.ErrLoopsAuthentication},
		{name: "missing signature", headers: loopsWebhookHeaders{id: []string{nativeWebhookID}, timestamp: []string{nativeWebhookTSString}, contentType: "application/json"}, body: body, wantErr: core.ErrLoopsAuthentication},
		{name: "duplicate signature fields", headers: loopsWebhookHeaders{id: []string{nativeWebhookID}, timestamp: []string{nativeWebhookTSString}, signature: []string{rotated, valid}, contentType: "application/json"}, body: body, wantErr: core.ErrLoopsAuthentication},
		{name: "duplicate id fields", headers: loopsWebhookHeaders{id: []string{nativeWebhookID, nativeWebhookID}, timestamp: []string{nativeWebhookTSString}, signature: []string{valid}, contentType: "application/json"}, body: body, wantErr: core.ErrLoopsAuthentication},
		{name: "oversized id", headers: loopsWebhookHeaders{id: []string{strings.Repeat("i", core.LoopsWebhookIDMaximumBytes+1)}, timestamp: []string{nativeWebhookTSString}, signature: []string{valid}, contentType: "application/json"}, body: body, wantErr: core.ErrLoopsAuthentication},
		{name: "oversized signature header", headers: nativeHeaders(valid + " v2," + strings.Repeat("x", core.LoopsWebhookSignatureMaximumBytes)), body: body, wantErr: core.ErrLoopsAuthentication},
		{name: "foreign media type", headers: loopsWebhookHeaders{id: []string{nativeWebhookID}, timestamp: []string{nativeWebhookTSString}, signature: []string{valid}, contentType: "text/plain"}, body: body, wantErr: core.ErrExchangeContentType},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			receiver := loopsWebhookReceiver(t, nativeWebhookSecret)
			directory := t.TempDir()
			var destination bytes.Buffer
			observation, err := loopsWebhookReceive(t, receiver, tc.headers, bytes.NewReader(tc.body), &destination, loopsSeconds(t, nativeWebhookSeconds), directory)
			requireLoopsScratchEmpty(t, directory)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Receive() error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if destination.Len() != 0 || observation.Validate() == nil {
					t.Fatalf("refused delivery released %d bytes, observation %+v", destination.Len(), observation)
				}
				return
			}
			if observation.Validate() != nil || observation.DeliveryID.String() != tc.headers.id[0] || observation.Bytes.Uint64() != uint64(len(tc.body)) || !bytes.Equal(destination.Bytes(), tc.body) {
				t.Fatalf("accepted delivery = %+v body %q, want exact delivery and body", observation, destination.Bytes())
			}
		})
	}
}

func TestStreamedStandardWebhooksAcrossTransferWindows(t *testing.T) {
	t.Parallel()
	for index := range 16 {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			t.Parallel()
			size := [...]int{0, 1, 4095, 4096, 4097, 32768, 1 << 20, (6 << 20) + 1}[index%8]
			body := bytes.Repeat([]byte{byte('a' + index)}, size)
			id := fmt.Sprintf("msg_window_%d", index)
			signature := nativeSignature(nativeWebhookKey, id, nativeWebhookTSString, body)
			receiver := loopsWebhookReceiver(t, nativeWebhookSecret)
			directory := t.TempDir()
			headers := loopsWebhookHeaders{id: []string{id}, timestamp: []string{nativeWebhookTSString}, signature: []string{signature}, contentType: "application/json"}
			var destination bytes.Buffer
			observation, err := loopsWebhookReceive(t, receiver, headers, bytes.NewReader(body), &destination, loopsSeconds(t, nativeWebhookSeconds), directory)
			if err != nil || observation.Bytes.Uint64() != uint64(size) || !bytes.Equal(destination.Bytes(), body) {
				t.Fatalf("streamed delivery %d bytes = %+v/%v, exact %t", size, observation, err, bytes.Equal(destination.Bytes(), body))
			}
			requireLoopsScratchEmpty(t, directory)
			mutated := append(bytes.Clone(body), 'z')
			destination.Reset()
			observation, err = loopsWebhookReceive(t, receiver, headers, bytes.NewReader(mutated), &destination, loopsSeconds(t, nativeWebhookSeconds), directory)
			if !errors.Is(err, core.ErrLoopsVerification) || destination.Len() != 0 || observation.Validate() == nil {
				t.Fatalf("one appended byte released %d bytes: %+v/%v", destination.Len(), observation, err)
			}
			requireLoopsScratchEmpty(t, directory)
		})
	}
}

func TestStaleOrMalformedEvidenceReadsNoBodyByte(t *testing.T) {
	t.Parallel()
	body := []byte(`{"eventName":"testing.testEvent"}`)
	valid := nativeSignature(nativeWebhookKey, nativeWebhookID, nativeWebhookTSString, body)
	for _, tc := range []struct {
		name     string
		headers  loopsWebhookHeaders
		observed int64
	}{
		{name: "stale timestamp", headers: nativeHeaders(valid), observed: nativeWebhookSeconds + core.LoopsWebhookToleranceSeconds + 1},
		{name: "malformed signature header", headers: nativeHeaders("v1"), observed: nativeWebhookSeconds},
		{name: "missing id", headers: loopsWebhookHeaders{timestamp: []string{nativeWebhookTSString}, signature: []string{valid}, contentType: "application/json"}, observed: nativeWebhookSeconds},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			receiver := loopsWebhookReceiver(t, nativeWebhookSecret)
			reader := &nativeCountingReader{reader: bytes.NewReader(body)}
			directory := t.TempDir()
			var destination bytes.Buffer
			observation, err := loopsWebhookReceive(t, receiver, tc.headers, reader, &destination, loopsSeconds(t, tc.observed), directory)
			if err == nil || reader.reads != 0 || destination.Len() != 0 || observation != (InboundObservation{}) {
				t.Fatalf("pre-body refusal = %+v/%v, reads %d, released %d; want zero evidence and no body read", observation, err, reader.reads, destination.Len())
			}
			requireLoopsScratchEmpty(t, directory)
		})
	}
}

func TestLoopsCloseZeroesSharedCustodyAndRefusesCopiedReceiver(t *testing.T) {
	t.Parallel()
	for index := range 8 {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			t.Parallel()
			receiver := loopsWebhookReceiver(t, nativeWebhookSecret)
			copied := receiver
			custody := receiver.state.secret.key
			body := []byte(fmt.Sprintf(`{"index":%d}`, index))
			signature := nativeSignature(nativeWebhookKey, nativeWebhookID, nativeWebhookTSString, body)
			if err := receiver.Close(); err != nil {
				t.Fatal(err)
			}
			for position, value := range custody {
				if value != 0 {
					t.Fatalf("owned webhook key byte %d = %#x after Close, want 0", position, value)
				}
			}
			if err := receiver.Close(); !errors.Is(err, core.ErrLoopsContract) {
				t.Fatalf("second Close() = %v, want contract refusal", err)
			}
			reader := &nativeCountingReader{reader: bytes.NewReader(body)}
			directory := t.TempDir()
			var destination bytes.Buffer
			observation, err := loopsWebhookReceive(t, copied, nativeHeaders(signature), reader, &destination, loopsSeconds(t, nativeWebhookSeconds), directory)
			if !errors.Is(err, core.ErrLoopsContract) || observation != (InboundObservation{}) || reader.reads != 0 || destination.Len() != 0 || copied.Validate() == nil {
				t.Fatalf("closed shared copy = %+v/%v reads %d released %d", observation, err, reader.reads, destination.Len())
			}
			requireLoopsScratchEmpty(t, directory)
		})
	}
}

func TestReceiverOwnsIndependentSecretCustody(t *testing.T) {
	t.Parallel()
	secret, err := ParseWebhookSecret([]byte(nativeWebhookSecret))
	if err != nil {
		t.Fatal(err)
	}
	receiver, err := NewWebhookReceiver(secret)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = receiver.Close() }()
	if err := secret.Close(); err != nil {
		t.Fatal(err)
	}
	body := []byte(`{}`)
	directory := t.TempDir()
	var destination bytes.Buffer
	_, err = loopsWebhookReceive(t, receiver, nativeHeaders(nativeSignature(nativeWebhookKey, nativeWebhookID, nativeWebhookTSString, body)), bytes.NewReader(body), &destination, loopsSeconds(t, nativeWebhookSeconds), directory)
	if err != nil || destination.String() != `{}` {
		t.Fatalf("receiver after caller Close = %v/%q, want independent custody", err, destination.String())
	}
	if _, err := NewWebhookReceiver(WebhookSecret{}); !errors.Is(err, core.ErrLoopsContract) {
		t.Fatalf("NewWebhookReceiver(zero) = %v, want contract refusal", err)
	}
}

func TestLoopsCompleteBytesWithNativeReadFailureReleaseNothing(t *testing.T) {
	t.Parallel()
	for index := range 8 {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			t.Parallel()
			body := bytes.Repeat([]byte{'v'}, [...]int{1, 4095, 4096, 4097, 32767, 32768, 32769, 65537}[index])
			signature := nativeSignature(nativeWebhookKey, nativeWebhookID, nativeWebhookTSString, body)
			receiver := loopsWebhookReceiver(t, nativeWebhookSecret)
			directory := t.TempDir()
			var destination bytes.Buffer
			observation, err := loopsWebhookReceive(t, receiver, nativeHeaders(signature), nativeFailingReader{reader: bytes.NewReader(body)}, &destination, loopsSeconds(t, nativeWebhookSeconds), directory)
			if !errors.Is(err, io.ErrUnexpectedEOF) || destination.Len() != 0 || observation.Validate() == nil {
				t.Fatalf("native read failure = %+v/%v, released %d", observation, err, destination.Len())
			}
			requireLoopsScratchEmpty(t, directory)
		})
	}
}

func FuzzStreamedStandardWebhooksMatchesIndependentHMAC(f *testing.F) {
	for _, body := range [][]byte{nil, []byte(officialWebhookBody), {0, 255, '\n'}, []byte(strings.Repeat("x", 4097))} {
		f.Add(body, []byte{}, true, int64(0))
		f.Add(body, []byte("changed"), false, int64(core.LoopsWebhookToleranceSeconds))
		f.Add(body, []byte{}, true, int64(-core.LoopsWebhookToleranceSeconds-1))
	}
	f.Fuzz(func(t *testing.T, signedBody, candidate []byte, genuine bool, skew int64) {
		if skew > 1_000_000 || skew < -1_000_000 {
			skew %= 1_000_000
		}
		signature := nativeSignature(nativeWebhookKey, nativeWebhookID, nativeWebhookTSString, signedBody)
		body := candidate
		if genuine {
			body = signedBody
		}
		want := bytes.Equal(body, signedBody) && skew >= -core.LoopsWebhookToleranceSeconds && skew <= core.LoopsWebhookToleranceSeconds
		receiver := loopsWebhookReceiver(t, nativeWebhookSecret)
		directory := t.TempDir()
		var destination bytes.Buffer
		observation, err := loopsWebhookReceive(t, receiver, nativeHeaders(signature), bytes.NewReader(body), &destination, loopsSeconds(t, nativeWebhookSeconds+skew), directory)
		if want {
			if err != nil || observation.Bytes.Uint64() != uint64(len(body)) || !bytes.Equal(destination.Bytes(), body) {
				t.Fatalf("independent HMAC accepted, native handoff %+v/%v", observation, err)
			}
		} else if !errors.Is(err, core.ErrLoopsVerification) || observation.Validate() == nil || destination.Len() != 0 {
			t.Fatalf("independent oracle refused, native handoff %+v/%v released %d", observation, err, destination.Len())
		}
		requireLoopsScratchEmpty(t, directory)
	})
}

func BenchmarkStreamedStandardWebhooksBodyMemory(b *testing.B) {
	b.ReportAllocs()
	for _, extent := range [...]int{4096, 8 << 20, 64 << 20} {
		b.Run(fmt.Sprint(extent), func(b *testing.B) {
			body := bytes.Repeat([]byte{'v'}, extent)
			signature := nativeSignature(nativeWebhookKey, nativeWebhookID, nativeWebhookTSString, body)
			receiver := loopsWebhookReceiver(b, nativeWebhookSecret)
			directory := b.TempDir()
			fixture := filepath.Join(b.TempDir(), "native-body")
			if err := os.WriteFile(fixture, body, 0600); err != nil {
				b.Fatal(err)
			}
			total := int64(len(body))
			body = nil
			observedAt := loopsSeconds(b, nativeWebhookSeconds)
			b.ReportAllocs()
			b.SetBytes(total)
			b.ResetTimer()
			for range b.N {
				source, err := os.Open(fixture)
				if err != nil {
					b.Fatal(err)
				}
				// Exchange owns and closes the HTTP request body, including native files.
				observation, err := loopsWebhookReceive(b, receiver, nativeHeaders(signature), source, io.Discard, observedAt, directory)
				if err != nil || observation.Bytes.Uint64() != uint64(total) {
					b.Fatalf("native streamed benchmark %+v/%v", observation, err)
				}
			}
		})
	}
}
