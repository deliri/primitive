package cloudflare

import (
	"bytes"
	"encoding/json/v2"
	"errors"
	"io"
	"math"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestStreamServerProviderLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		wantErr   error
		name      string
		success   bool
		empty     bool
		watermark bool
	}{
		{name: "JSON reservation returns issued upload authority", success: true},
		{name: "documented watermark does not break strict response decoding", success: true, watermark: true},
		{name: "provider refusal cannot emit upload authority", wantErr: core.ErrCloudflareResponse},
		{name: "empty successful result cannot invent video ID", success: true, empty: true, wantErr: core.ErrCloudflareResponse},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int64
			client := testExchange(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				if r.Method != http.MethodPost || r.URL.Path != "/client/v4/accounts/"+strings.Repeat("a", core.CloudflareIdentityCharacters)+"/stream/direct_upload" || r.Header.Get("Authorization") != "Bearer cloudflare-test-token" || r.Header.Get("Content-Type") != core.HTTPMediaTypeJSON().String() {
					t.Errorf("Stream request=(%s,%s,%v), want account-bound JSON POST", r.Method, r.URL.Path, r.Header)
				}
				var got streamDirectUploadRequestWire
				if err := json.UnmarshalRead(r.Body, &got, json.RejectUnknownMembers(true)); err != nil {
					t.Error(err)
				}
				if got.MaximumDurationSeconds != 123 || got.Creator != "owner-42" || !got.RequireSignedURLs {
					t.Errorf("Stream reservation=%+v, want exact duration, creator and signed-delivery policy", got)
				}
				result := streamDirectUploadWire{UID: strings.Repeat("b", core.CloudflareIdentityCharacters), UploadURL: "https://" + core.CloudflareStreamUploadHost + "/upload"}
				if tc.empty {
					result = streamDirectUploadWire{}
				}
				if tc.watermark {
					opacity := 0.75
					result.Watermark = &streamWatermarkWire{Position: core.CloudflareWatermarkCenter, Opacity: &opacity}
				}
				w.Header().Set("Content-Type", core.HTTPMediaTypeJSON().String())
				if err := json.MarshalWrite(w, apiEnvelope[streamDirectUploadWire]{Success: &tc.success, Result: result}); err != nil {
					t.Error(err)
				}
			})
			server, err := NewStreamServer(client, testOptions(t))
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := server.Close(); err != nil {
					t.Error(err)
				}
			}()
			got, gotErr := server.CreateDirectUpload(t.Context(), StreamDirectUploadRequest{Creator: "owner-42", MaximumDurationSeconds: 123, RequireSignedURLs: true}, testPolicy())
			if calls.Load() != 1 || !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("Stream calls/error=%d/%v, want 1/%v", calls.Load(), gotErr, tc.wantErr)
			}
			if gotErr != nil {
				if got != (StreamUpload{}) {
					t.Fatalf("refused upload=%v, want zero", got)
				}
				return
			}
			if got.Validate() != nil || got.ID.value != strings.Repeat("b", core.CloudflareIdentityCharacters) || got.endpoint.String() != "https://"+core.CloudflareStreamUploadHost+"/upload" {
				t.Fatalf("issued Stream authority=%v, want exact provider identity", got)
			}
		})
	}
}

func TestStreamBasicUploadRealMultipart(t *testing.T) {
	t.Parallel()
	const body = "exact video payload"
	observed := make(chan string, 1)
	client := testExchange(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.Host != core.CloudflareStreamUploadHost || r.Header.Get("Authorization") != "" {
			t.Errorf("upload method/host/auth=%s/%s/%q, want uncredentialed Stream POST", r.Method, r.Host, r.Header.Get("Authorization"))
		}
		reader, err := r.MultipartReader()
		if err != nil {
			t.Error(err)
			return
		}
		part, err := reader.NextPart()
		if err != nil {
			t.Error(err)
			return
		}
		if part.FormName() != core.CloudflareMultipartFileField || part.FileName() != "video.mp4" {
			t.Errorf("multipart identity=%s/%s, want file/video.mp4", part.FormName(), part.FileName())
		}
		var data bytes.Buffer
		if _, err := io.Copy(&data, part); err != nil {
			t.Error(err)
		}
		if err := part.Close(); err != nil {
			t.Error(err)
		}
		if _, err := reader.NextPart(); !errors.Is(err, io.EOF) {
			t.Errorf("extra multipart part error=%v, want EOF", err)
		}
		observed <- data.String()
		w.Header().Set("Content-Type", core.HTTPMediaTypeJSON().String())
		if _, err := io.WriteString(w, "{}"); err != nil {
			t.Error(err)
		}
	})
	video, err := ParseStreamVideoID(strings.Repeat("b", core.CloudflareIdentityCharacters))
	if err != nil {
		t.Fatal(err)
	}
	authority, err := ParseStreamUpload(video, "https://"+core.CloudflareStreamUploadHost+"/upload")
	if err != nil {
		t.Fatal(err)
	}
	uploader, err := NewStreamClient(client)
	if err != nil {
		t.Fatal(err)
	}
	length, err := core.NewByteLength(uint64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	got, err := uploader.UploadBasic(t.Context(), authority, MediaUpload{Source: strings.NewReader(body), Response: io.Discard, Filename: "video.mp4", Bytes: length}, testPolicy())
	if err != nil || got.Metadata.Bytes.Uint64() != 2 {
		t.Fatalf("basic upload=(%+v,%v), want exact response receipt", got, err)
	}
	select {
	case got := <-observed:
		if got != body {
			t.Fatalf("provider body=%q, want %q", got, body)
		}
	default:
		t.Fatalf("provider observations = %d, want exact multipart body", len(observed))
	}
}

func FuzzStreamDirectUploadResponseSemanticClosure(f *testing.F) {
	accepted := true
	wire := streamDirectUploadWire{UID: strings.Repeat("b", core.CloudflareIdentityCharacters), UploadURL: "https://" + core.CloudflareStreamUploadHost + "/upload"}
	if err := wire.Validate(); err != nil {
		f.Fatal(err)
	}
	seed, err := core.MarshalCanonicalJSONDocument(apiEnvelope[streamDirectUploadWire]{Success: &accepted, Result: wire})
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seed)
	f.Add([]byte{})
	f.Add([]byte(`{"success":true,"result":null}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		calls := 0
		client, err := exchange.NewClient(&http.Client{Transport: responseTransport{data: data, calls: &calls}})
		if err != nil {
			t.Fatal(err)
		}
		server, err := NewStreamServer(client, testOptions(t))
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := server.Close(); err != nil {
				t.Error(err)
			}
		}()
		got, gotErr := server.CreateDirectUpload(t.Context(), StreamDirectUploadRequest{MaximumDurationSeconds: 60}, testPolicy())
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrCloudflareContract) || got != (StreamUpload{}) {
				t.Fatalf("Stream refusal=(%v,%v), want zero typed refusal", got, gotErr)
			}
			return
		}
		var independent apiEnvelope[streamDirectUploadWire]
		if err := json.Unmarshal(data, &independent); err != nil || independent.Success == nil || !*independent.Success || len(independent.Errors) != 0 || independent.Result.UID != got.ID.value || independent.Result.UploadURL != got.endpoint.String() {
			t.Fatalf("admitted Stream source agreement error=%v, want exact provider facts", err)
		}
		if got.Validate() != nil || calls != 1 {
			t.Fatalf("authority validation/calls=%v/%d, want nil/1", got.Validate(), calls)
		}
		canonical, err := core.MarshalCanonicalJSONDocument(independent)
		if err != nil {
			t.Fatal(err)
		}
		again, err := core.DecodeStrictJSONStructure[apiEnvelope[streamDirectUploadWire]](canonical, core.DefaultStrictJSONLimits())
		if err != nil {
			t.Fatal(err)
		}
		second, err := core.MarshalCanonicalJSONDocument(again)
		if err != nil || !bytes.Equal(canonical, second) {
			t.Fatalf("canonical closure error=%v, want byte-identical round trip", err)
		}
	})
}

func TestStreamDurationRetainsWholeTemporalSeconds(t *testing.T) {
	t.Parallel()
	const second = int64(temporal.NanosecondsPerSecond)
	const maximum = core.CloudflareStreamDurationMaximumSeconds * second
	for _, tc := range []struct {
		wantErr error
		name    string
		nanos   int64
		want    StreamDurationSeconds
	}{
		{name: "zero reserves no usable duration", wantErr: core.ErrCloudflareContract},
		{name: "one nanosecond below minimum refuses truncation", nanos: second - 1, wantErr: core.ErrCloudflareContract},
		{name: "exact minimum retains one second", nanos: second, want: 1},
		{name: "one nanosecond above minimum refuses rounding", nanos: second + 1, wantErr: core.ErrCloudflareContract},
		{name: "ordinary whole duration preserves sixty seconds", nanos: 60 * second, want: 60},
		{name: "one second below maximum retains duration", nanos: maximum - second, want: core.CloudflareStreamDurationMaximumSeconds - 1},
		{name: "exact provider ceiling remains accepted", nanos: maximum, want: core.CloudflareStreamDurationMaximumSeconds},
		{name: "one nanosecond above ceiling is refused", nanos: maximum + 1, wantErr: core.ErrCloudflareContract},
		{name: "one second above ceiling is refused", nanos: maximum + second, wantErr: core.ErrCloudflareContract},
		{name: "extreme native duration cannot wrap into admission", nanos: math.MaxInt64, wantErr: core.ErrCloudflareContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			duration, err := temporal.DurationFromNanoseconds(tc.nanos)
			if err != nil {
				t.Fatal(err)
			}
			got, gotErr := StreamDurationFromTemporal(duration)
			if got != tc.want || !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("StreamDurationFromTemporal(%d)=(%d,%v), want (%d,%v)", tc.nanos, got, gotErr, tc.want, tc.wantErr)
			}
			if gotErr == nil {
				restored, err := got.Duration()
				if err != nil || restored != duration {
					t.Fatalf("Temporal round trip=(%v,%v), want (%v,nil)", restored, err, duration)
				}
			}
		})
	}
}
