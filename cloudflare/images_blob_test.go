package cloudflare

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func TestImagesOriginalRealTLSBoundedTransfer(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                                                                           string
		body                                                                           string
		maximum                                                                        uint64
		media                                                                          string
		status                                                                         int
		invalidID, nilDestination, typedNilDestination, zeroMaximum, wrongRequestMedia bool
		wantErr                                                                        error
		wantCalls                                                                      int64
	}{
		{name: "ordinary original preserves exact bytes", body: "original", maximum: 8, media: "image/png", status: 200, wantCalls: 1},
		{name: "one below export ceiling preserves exact bytes", body: "seven77", maximum: 8, media: "image/png", status: 200, wantCalls: 1},
		{name: "one above export ceiling is refused", body: "nine99999", maximum: 8, media: "image/png", status: 200, wantCalls: 1, wantErr: core.ErrCloudflareResponse},
		{name: "empty provider body remains empty", maximum: 8, media: "image/png", status: 200, wantCalls: 1},
		{name: "minimum positive extent admits one byte", body: "x", maximum: 1, media: "image/png", status: 200, wantCalls: 1},
		{name: "minimum positive extent refuses second byte", body: "xy", maximum: 1, media: "image/png", status: 200, wantCalls: 1, wantErr: core.ErrCloudflareResponse},
		{name: "foreign representation never reaches destination", body: "json", maximum: 8, media: "application/json", status: 200, wantCalls: 1, wantErr: core.ErrExchangeResponse},
		{name: "missing representation never reaches destination", body: "x", maximum: 8, status: 200, wantCalls: 1, wantErr: core.ErrExchangeResponse},
		{name: "provider refusal never reaches destination", body: "denied", maximum: 8, media: "image/png", status: 403, wantCalls: 1, wantErr: core.ErrExchangeResponse},
		{name: "redirect cannot export another authority", body: "x", maximum: 8, media: "image/png", status: 302, wantCalls: 1, wantErr: core.ErrExchangeResponse},
		{name: "absent image identity performs no request", maximum: 8, invalidID: true, wantErr: core.ErrCloudflareContract},
		{name: "absent destination performs no request", maximum: 8, nilDestination: true, wantErr: core.ErrCloudflareContract},
		{name: "typed nil destination performs no request", maximum: 8, typedNilDestination: true, wantErr: core.ErrCloudflareContract},
		{name: "zero ceiling performs no request", zeroMaximum: true, wantErr: core.ErrCloudflareContract},
		{name: "nonimage request representation performs no request", maximum: 8, wrongRequestMedia: true, wantErr: core.ErrCloudflareContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var calls atomic.Int64
			client := testExchange(t, func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				wantPath := "/client/v4/accounts/" + strings.Repeat("a", core.CloudflareIdentityCharacters) + "/images/v1/tenant/photo/blob"
				if r.Method != http.MethodGet || r.URL.Path != wantPath || r.Header.Get("Authorization") != "Bearer cloudflare-test-token" {
					t.Errorf("request=(%s,%s,authenticated=%t), want authenticated GET %s", r.Method, r.URL.Path, r.Header.Get("Authorization") != "", wantPath)
				}
				if tc.media != "" {
					w.Header().Set("Content-Type", tc.media)
				}
				status := tc.status
				if status == 0 {
					status = http.StatusOK
				}
				w.WriteHeader(status)
				if _, err := io.WriteString(w, tc.body); err != nil {
					t.Error(err)
				}
			})
			server, err := NewImagesServer(client, testOptions(t))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := server.Close(); err != nil {
					t.Error(err)
				}
			})
			id, err := ParseImageID("tenant/photo")
			if err != nil {
				t.Fatal(err)
			}
			media, err := core.ParseHTTPMediaType("image/png")
			if err != nil {
				t.Fatal(err)
			}
			var destination bytes.Buffer
			request := ImageBlobDownloadRequest{ID: id, Destination: &destination, ContentType: media}
			if !tc.zeroMaximum {
				request.Maximum, err = core.NewByteCount(tc.maximum)
				if err != nil {
					t.Fatal(err)
				}
			}
			if tc.invalidID {
				request.ID = ImageID{}
			}
			if tc.nilDestination {
				request.Destination = nil
			}
			if tc.typedNilDestination {
				var absent *bytes.Buffer
				request.Destination = absent
			}
			if tc.wrongRequestMedia {
				request.ContentType = core.HTTPMediaTypeJSON()
			}
			_, gotErr := server.DownloadOriginal(t.Context(), request, testPolicy())
			if !errors.Is(gotErr, tc.wantErr) || calls.Load() != tc.wantCalls {
				t.Fatalf("DownloadOriginal=(%v,%d calls), want (%v,%d calls)", gotErr, calls.Load(), tc.wantErr, tc.wantCalls)
			}
			if gotErr == nil && destination.String() != tc.body {
				t.Fatalf("destination=%q, want %q", destination.String(), tc.body)
			}
			if uint64(destination.Len()) > tc.maximum {
				t.Fatalf("destination=%d bytes, want <= %d", destination.Len(), tc.maximum)
			}
			if tc.wantErr != nil && tc.wantErr != core.ErrCloudflareResponse && destination.Len() != 0 {
				t.Fatalf("refused destination=%d bytes, want 0", destination.Len())
			}
		})
	}
}

type imageBlobResponseTransport struct {
	data  []byte
	calls *int
}

func (r imageBlobResponseTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	*r.calls++
	return &http.Response{StatusCode: http.StatusOK, Header: http.Header{"Content-Type": []string{"image/png"}}, Body: io.NopCloser(bytes.NewReader(r.data)), Request: request}, nil
}

// The blob is opaque provider bytes, with no JSON/binary document marshaler.
// The canonical seed request is constructed and validated through native types.
func FuzzImagesOriginalPublicTransferSemanticBound(f *testing.F) {
	f.Add([]byte("opaque-original"), uint16(15))
	f.Add([]byte{}, uint16(1))
	f.Fuzz(func(t *testing.T, data []byte, ceiling uint16) {
		if len(data) > 65536 {
			return
		}
		calls := 0
		client, err := exchange.NewClient(&http.Client{Transport: imageBlobResponseTransport{data: data, calls: &calls}})
		if err != nil {
			t.Fatal(err)
		}
		server, err := NewImagesServer(client, testOptions(t))
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() {
			if err := server.Close(); err != nil {
				t.Error(err)
			}
		})
		id, err := ParseImageID("tenant/photo")
		if err != nil {
			t.Fatal(err)
		}
		media, err := core.ParseHTTPMediaType("image/png")
		if err != nil {
			t.Fatal(err)
		}
		var destination bytes.Buffer
		request := ImageBlobDownloadRequest{ID: id, ContentType: media, Destination: &destination}
		if ceiling != 0 {
			request.Maximum, err = core.NewByteCount(uint64(ceiling))
			if err != nil {
				t.Fatal(err)
			}
			if err := request.Validate(); err != nil {
				t.Fatal(err)
			}
		}
		_, gotErr := server.DownloadOriginal(t.Context(), request, testPolicy())
		if ceiling == 0 {
			if !errors.Is(gotErr, core.ErrCloudflareContract) || calls != 0 || destination.Len() != 0 {
				t.Fatalf("zero-ceiling=(%v,%d calls,%d bytes), want typed refusal and no effects", gotErr, calls, destination.Len())
			}
			return
		}
		if calls != 1 || destination.Len() > int(ceiling) {
			t.Fatalf("transfer=(%d calls,%d bytes), want one call and <=%d bytes", calls, destination.Len(), ceiling)
		}
		if len(data) <= int(ceiling) {
			if gotErr != nil || !bytes.Equal(destination.Bytes(), data) {
				t.Fatalf("admitted original=(%v,%d bytes), want exact %d source bytes", gotErr, destination.Len(), len(data))
			}
		} else if !errors.Is(gotErr, core.ErrCloudflareResponse) {
			t.Fatalf("oversized original error=%v, want typed extent refusal", gotErr)
		}
	})
}

func FuzzImageBlobDestinationSemanticBound(f *testing.F) {
	f.Add([]byte("original"), uint16(8), uint16(3))
	f.Add([]byte{}, uint16(1), uint16(1))
	f.Fuzz(func(t *testing.T, data []byte, ceiling uint16, split uint16) {
		if len(data) > 65536 {
			return
		}
		var destination bytes.Buffer
		writer := imageBlobDestination{writer: &destination, remaining: uint64(ceiling)}
		position := int(split)
		if position > len(data) {
			position = len(data)
		}
		for _, part := range [][]byte{data[:position], data[position:]} {
			before := destination.Len()
			n, err := writer.Write(part)
			if len(part) > int(ceiling)-before {
				if n != 0 || !errors.Is(err, core.ErrCloudflareResponse) || destination.Len() != before {
					t.Fatalf("oversized write=(%d,%v,%d bytes), want unchanged %d bytes", n, err, destination.Len(), before)
				}
				continue
			}
			if err != nil || n != len(part) || !bytes.Equal(destination.Bytes()[before:], part) {
				t.Fatalf("admitted write=(%d,%v), want exact %d bytes", n, err, len(part))
			}
		}
		if uint64(destination.Len())+writer.remaining != uint64(ceiling) {
			t.Fatalf("written+remaining=%d, want %d", uint64(destination.Len())+writer.remaining, ceiling)
		}
	})
}
