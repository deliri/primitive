package exchange

import (
	"bytes"
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type wholeJSONReviewDocument struct {
	Text string `json:"text"`
}

func (wholeJSONReviewDocument) Validate() error { return nil }
func (d wholeJSONReviewDocument) MarshalJSON() ([]byte, error) {
	type wire wholeJSONReviewDocument
	return core.MarshalCanonicalJSONDocument(wire(d))
}
func TestJSONWholeValueWithoutTransportQuotaLayerTriad(t *testing.T) {
	t.Parallel()
	document := wholeJSONReviewDocument{Text: strings.Repeat("x", (1<<20)+1)}
	wire, err := document.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name    string
		wire    []byte
		wantErr error
	}{
		{name: "complete value beyond former default", wire: wire},
		{name: "unknown field after large value", wire: append(bytes.Clone(wire[:len(wire)-1]), []byte(",\"unknown\":true}")...), wantErr: core.ErrJSONContract},
		{name: "case folded duplicate after large value", wire: append(bytes.Clone(wire[:len(wire)-1]), []byte(",\"TEXT\":\"changed\"}")...), wantErr: core.ErrJSONContract},
		{name: "second document after large value", wire: append(bytes.Clone(wire), []byte("{}")...), wantErr: core.ErrJSONContract},
		{name: "truncated final delimiter", wire: wire[:len(wire)-1], wantErr: core.ErrJSONContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request := httptest.NewRequest(http.MethodPost, "/", io.NopCloser(bytes.NewReader(tc.wire)))
			request.Header.Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeJSON().String())
			call, err := NewSocketServerCall(httptest.NewRecorder(), request)
			if err != nil {
				t.Fatal(err)
			}
			got, err := ReceiveJSON[wholeJSONReviewDocument, *wholeJSONReviewDocument](JSONReceiveCall{Call: call, Route: RouteSemantics{Method: MethodPost, Replay: ReplaySingleAttempt}})
			if !errors.Is(err, tc.wantErr) || errors.Is(err, core.ErrExchangeBodyLimit) {
				t.Fatalf("receive = %v, want %v without transport quota", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got.Body != nil {
					t.Fatalf("refused document escaped: %d text bytes", len(got.Body.Text))
				}
				return
			}
			if got.Body == nil || *got.Body != document {
				t.Fatalf("received document = %v, want all %d text bytes", got.Body, len(document.Text))
			}
			output := httptest.NewRecorder()
			outCall, err := NewSocketServerCall(output, httptest.NewRequest(http.MethodGet, "/", nil))
			if err != nil {
				t.Fatal(err)
			}
			err = WriteJSON(JSONWriteCall[wholeJSONReviewDocument]{Call: outCall, Response: ServerJSONResponse[wholeJSONReviewDocument]{Body: *got.Body, Status: core.HTTPStatusOK()}})
			if err != nil || !bytes.Equal(output.Body.Bytes(), wire) || output.Code != http.StatusOK {
				t.Fatalf("write = %d bytes/status %d/%v, want exact %d-byte JSON and OK", output.Body.Len(), output.Code, err, len(wire))
			}
		})
	}
}
