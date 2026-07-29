package exchange_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func mustHeaderName(t testing.TB, value string) core.HTTPHeaderName {
	t.Helper()

	got, gotErr := core.ParseHTTPHeaderName(value)
	if gotErr != nil {
		t.Fatalf("core.ParseHTTPHeaderName(%q) setup error = %v, want nil", value, gotErr)
	}
	return got
}

func TestResponseHeaderFramingOwnershipLayerTriad(t *testing.T) {
	t.Parallel()

	custom := mustHeaderName(t, "X-Exchange-Result")
	other := mustHeaderName(t, "X-Exchange-Trace")
	maximumValues := make([]string, exchange.HeaderValueMaximumCount)
	for index := range maximumValues {
		maximumValues[index] = "value"
	}
	maximumFields := make([]exchange.Header, 0, exchange.HeaderMaximumCount)
	for index := range exchange.HeaderMaximumCount {
		maximumFields = append(maximumFields, exchange.Header{
			Name:   mustHeaderName(t, "X-Exchange-Field-"+string(rune('a'+index%26))+string(rune('a'+index/26))),
			Values: []string{"value"},
		})
	}

	cases := []struct {
		wantErr error
		name    string
		headers exchange.ResponseHeaders
	}{
		{
			name: "one caller field is admitted",
			headers: exchange.ResponseHeaders{Values: []exchange.Header{
				{Name: custom, Values: []string{"sealed"}},
			}},
		},
		{
			name: "two distinct caller fields are admitted",
			headers: exchange.ResponseHeaders{Values: []exchange.Header{
				{Name: custom, Values: []string{"sealed"}},
				{Name: other, Values: []string{"trace"}},
			}},
		},
		{
			name: "the exact repeated value bound is admitted",
			headers: exchange.ResponseHeaders{Values: []exchange.Header{
				{Name: custom, Values: maximumValues},
			}},
		},
		{
			name: "the exact value size bound is admitted",
			headers: exchange.ResponseHeaders{Values: []exchange.Header{
				{Name: custom, Values: []string{
					strings.Repeat("v", exchange.HeaderValueMaximumBytes),
				}},
			}},
		},
		{
			name:    "the exact field count bound is admitted",
			headers: exchange.ResponseHeaders{Values: maximumFields},
		},
		{
			name:    "an empty collection is admitted and fabricates nothing",
			headers: exchange.ResponseHeaders{},
		},
		{
			name: "Content-Type cannot be overridden by the caller",
			headers: exchange.ResponseHeaders{Values: []exchange.Header{
				{
					Name:   core.HTTPHeaderContentType(),
					Values: []string{core.HTTPMediaTypeTextPlain().String()},
				},
			}},
			wantErr: core.ErrExchangeContract,
		},
		{
			name: "Content-Length cannot be overridden by the caller",
			headers: exchange.ResponseHeaders{Values: []exchange.Header{
				{Name: core.HTTPHeaderContentLength(), Values: []string{"0"}},
			}},
			wantErr: core.ErrExchangeContract,
		},
		{
			name: "Content-Encoding cannot mislabel identity response bytes",
			headers: exchange.ResponseHeaders{Values: []exchange.Header{
				{Name: core.HTTPHeaderContentEncoding(), Values: []string{"gzip"}},
			}},
			wantErr: core.ErrExchangeContract,
		},
		{
			name: "a duplicate field name is refused",
			headers: exchange.ResponseHeaders{Values: []exchange.Header{
				{Name: custom, Values: []string{"first"}},
				{Name: custom, Values: []string{"second"}},
			}},
			wantErr: core.ErrExchangeContract,
		},
		{
			name: "an unset field name is refused",
			headers: exchange.ResponseHeaders{Values: []exchange.Header{
				{Values: []string{"orphan"}},
			}},
			wantErr: core.ErrExchangeContract,
		},
		{
			name: "a field with no values is refused",
			headers: exchange.ResponseHeaders{Values: []exchange.Header{
				{Name: custom},
			}},
			wantErr: core.ErrExchangeContract,
		},
		{
			name: "carriage return injection is refused",
			headers: exchange.ResponseHeaders{Values: []exchange.Header{
				{Name: custom, Values: []string{"sealed\rX-Injected: true"}},
			}},
			wantErr: core.ErrExchangeContract,
		},
		{
			name: "line feed injection is refused",
			headers: exchange.ResponseHeaders{Values: []exchange.Header{
				{Name: custom, Values: []string{"sealed\nX-Injected: true"}},
			}},
			wantErr: core.ErrExchangeContract,
		},
		{
			name: "one byte above the value size bound is refused",
			headers: exchange.ResponseHeaders{Values: []exchange.Header{
				{Name: custom, Values: []string{
					strings.Repeat("v", exchange.HeaderValueMaximumBytes+1),
				}},
			}},
			wantErr: core.ErrExchangeContract,
		},
		{
			name: "one above the repeated value bound is refused",
			headers: exchange.ResponseHeaders{Values: []exchange.Header{
				{Name: custom, Values: append(
					append([]string(nil), maximumValues...),
					"overflow",
				)},
			}},
			wantErr: core.ErrExchangeContract,
		},
		{
			name: "one above the field count bound is refused",
			headers: exchange.ResponseHeaders{Values: append(
				append([]exchange.Header(nil), maximumFields...),
				exchange.Header{Name: custom, Values: []string{"overflow"}},
			)},
			wantErr: core.ErrExchangeContract,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if gotErr := tc.headers.Validate(); !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("exchange.ResponseHeaders.Validate() = %v, want %v", gotErr, tc.wantErr)
			}
		})
	}
}

func TestResponseStatusBodyOwnershipLayerTriad(t *testing.T) {
	t.Parallel()

	writePolicy := exchange.JSONWritePolicy{
		ResponseBodyLimit: mustByteCount(t, 4*1024),
	}

	t.Run("positive a body-bearing status admits a typed JSON response", func(t *testing.T) {
		t.Parallel()

		for _, status := range []int{
			http.StatusOK,
			http.StatusCreated,
			http.StatusAccepted,
			http.StatusInternalServerError,
		} {
			recorder := httptest.NewRecorder()
			gotErr := exchange.WriteJSON(exchange.JSONWriteCall[transportDocument]{
				Writer: recorder,
				Response: exchange.ServerJSONResponse[transportDocument]{
					Body:   transportDocument{Message: "written"},
					Status: mustHTTPStatus(t, status),
				},
				Policy: writePolicy,
			})
			if gotErr != nil {
				t.Fatalf("exchange.WriteJSON(status %d) error = %v, want nil", status, gotErr)
			}
			if recorder.Code != status ||
				recorder.Header().Get(core.HTTPHeaderContentType().String()) !=
					core.HTTPMediaTypeJSON().String() {
				t.Fatalf(
					"written status/content type = (%d, %q), want (%d, %q)",
					recorder.Code,
					recorder.Header().Get(core.HTTPHeaderContentType().String()),
					status,
					core.HTTPMediaTypeJSON(),
				)
			}
		}
	})

	t.Run("negative a body-forbidding status refuses a body and writes nothing", func(t *testing.T) {
		t.Parallel()

		for _, status := range []int{
			http.StatusContinue,
			http.StatusNoContent,
			http.StatusNotModified,
		} {
			recorder := httptest.NewRecorder()
			gotErr := exchange.WriteJSON(exchange.JSONWriteCall[transportDocument]{
				Writer: recorder,
				Response: exchange.ServerJSONResponse[transportDocument]{
					Body:   transportDocument{Message: "forbidden"},
					Status: mustHTTPStatus(t, status),
				},
				Policy: writePolicy,
			})
			if !errors.Is(gotErr, core.ErrExchangeResponse) {
				t.Fatalf(
					"exchange.WriteJSON(status %d) error = %v, want %v",
					status,
					gotErr,
					core.ErrExchangeResponse,
				)
			}
			if recorder.Body.Len() != 0 || len(recorder.Header()) != 0 {
				t.Fatalf(
					"refused JSON write body/headers = (%d, %d), want (0, 0)",
					recorder.Body.Len(),
					len(recorder.Header()),
				)
			}
		}
	})

	t.Run("neutral a body-forbidding status remains a valid no-body response", func(t *testing.T) {
		t.Parallel()

		recorder := httptest.NewRecorder()
		gotErr := exchange.WriteNoBody(exchange.NoBodyWriteCall{
			Writer: recorder,
			Response: exchange.ServerNoBodyResponse{
				Status: mustHTTPStatus(t, http.StatusNoContent),
			},
		})
		if gotErr != nil {
			t.Fatalf("exchange.WriteNoBody(204) error = %v, want nil", gotErr)
		}
		if recorder.Code != http.StatusNoContent || recorder.Body.Len() != 0 {
			t.Fatalf(
				"no-body 204 status/body = (%d, %d), want (%d, 0)",
				recorder.Code,
				recorder.Body.Len(),
				http.StatusNoContent,
			)
		}
	})

	t.Run("neutral an absent response writer is refused before any effect", func(t *testing.T) {
		t.Parallel()

		jsonErr := exchange.WriteJSON(exchange.JSONWriteCall[transportDocument]{
			Response: exchange.ServerJSONResponse[transportDocument]{
				Body:   transportDocument{Message: "written"},
				Status: mustHTTPStatus(t, http.StatusOK),
			},
			Policy: writePolicy,
		})
		noBodyErr := exchange.WriteNoBody(exchange.NoBodyWriteCall{
			Response: exchange.ServerNoBodyResponse{
				Status: mustHTTPStatus(t, http.StatusNoContent),
			},
		})
		streamErr := exchange.WriteStream(exchange.StreamWriteCall{
			Context: context.Background(),
			Response: exchange.ServerStreamResponse{
				Source:        bytes.NewReader(nil),
				ContentLength: core.NewByteLength(0),
				ContentType:   core.HTTPMediaTypeOctetStream(),
				Status:        mustHTTPStatus(t, http.StatusOK),
			},
		})
		for name, gotErr := range map[string]error{
			"WriteJSON":   jsonErr,
			"WriteNoBody": noBodyErr,
			"WriteStream": streamErr,
		} {
			if !errors.Is(gotErr, core.ErrExchangeResponse) {
				t.Fatalf(
					"exchange.%s(nil writer) error = %v, want %v",
					name,
					gotErr,
					core.ErrExchangeResponse,
				)
			}
		}
	})
}

// streamEgressCase declares one exact-extent server response and the wire the
// caller must observe for it.
type streamEgressCase struct {
	wantErr           error
	name              string
	sourceBytes       int
	declaredLength    uint64
	wantWireBytes     int
	wantWireTruncated bool
}

func TestStreamEgressExactExtentLayerTriad(t *testing.T) {
	t.Parallel()

	const declared = 4 * exchange.TransferBufferBytes
	cases := []streamEgressCase{
		{
			name:           "positive an exact source fills the declared extent",
			sourceBytes:    declared,
			declaredLength: declared,
			wantWireBytes:  declared,
		},
		{
			name:              "negative a truncated source cannot satisfy the declared extent",
			sourceBytes:       declared - 1,
			declaredLength:    declared,
			wantWireBytes:     declared - 1,
			wantErr:           io.ErrUnexpectedEOF,
			wantWireTruncated: true,
		},
		{
			name:           "negative an oversized source is cut at the declared extent",
			sourceBytes:    declared + 1,
			declaredLength: declared,
			wantWireBytes:  declared,
			wantErr:        core.ErrExchangeBodyLimit,
		},
		{
			name:           "neutral a zero extent refuses a source that still has bytes",
			sourceBytes:    1,
			declaredLength: 0,
			wantWireBytes:  0,
			wantErr:        core.ErrExchangeBodyLimit,
		},
		{
			name:           "neutral a zero extent with an exhausted source writes nothing",
			sourceBytes:    0,
			declaredLength: 0,
			wantWireBytes:  0,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			ok := mustHTTPStatus(t, http.StatusOK)
			payload := bytes.Repeat([]byte{0x3c}, tc.sourceBytes)
			observed := make(chan error, 1)
			server := httptest.NewServer(http.HandlerFunc(func(
				writer http.ResponseWriter,
				request *http.Request,
			) {
				observed <- exchange.WriteStream(exchange.StreamWriteCall{
					Context: request.Context(),
					Writer:  writer,
					Response: exchange.ServerStreamResponse{
						Source:        bytes.NewReader(payload),
						ContentLength: core.NewByteLength(tc.declaredLength),
						ContentType:   core.HTTPMediaTypeOctetStream(),
						Status:        ok,
					},
				})
			}))
			defer server.Close()

			response, gotSendErr := server.Client().Get(server.URL)
			if gotSendErr != nil || response == nil || response.Body == nil {
				t.Fatalf(
					"server.Client().Get() = (%v, %v), want a real response and nil",
					response,
					gotSendErr,
				)
				return
			}
			wire, gotReadErr := io.ReadAll(response.Body)
			gotCloseErr := response.Body.Close()
			if gotCloseErr != nil {
				t.Fatalf("wire body close error = %v, want nil", gotCloseErr)
			}

			if (gotReadErr != nil) != tc.wantWireTruncated {
				t.Fatalf(
					"caller wire read error = %v, want truncation observed = %t",
					gotReadErr,
					tc.wantWireTruncated,
				)
			}
			gotWriteErr := <-observed
			if tc.wantErr == nil {
				if gotWriteErr != nil {
					t.Fatalf("exchange.WriteStream() error = %v, want nil", gotWriteErr)
				}
			} else {
				if !errors.Is(gotWriteErr, tc.wantErr) ||
					!errors.Is(gotWriteErr, core.ErrExchangeWrite) {
					t.Fatalf(
						"exchange.WriteStream() error = %v, want %v and %v",
						gotWriteErr,
						tc.wantErr,
						core.ErrExchangeWrite,
					)
				}
			}
			if len(wire) != tc.wantWireBytes ||
				!bytes.Equal(wire, payload[:tc.wantWireBytes]) {
				t.Fatalf(
					"wire bytes/prefix = (%d, %t), want (%d, true)",
					len(wire),
					bytes.Equal(wire, payload[:tc.wantWireBytes]),
					tc.wantWireBytes,
				)
			}
			if gotLength := response.Header.Get(
				core.HTTPHeaderContentLength().String(),
			); gotLength != "" && tc.declaredLength == 0 && gotLength != "0" {
				t.Fatalf("declared Content-Length = %q, want %q", gotLength, "0")
			}
		})
	}
}
