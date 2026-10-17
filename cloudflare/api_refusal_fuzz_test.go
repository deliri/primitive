package cloudflare

import (
	"bytes"
	"encoding/json/v2"
	"errors"
	"io"
	"net/http"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

// This explicit transport seam tests terminal body causes and fuzzed provider
// documents. api_refusal_status_test.go separately proves real local TLS.
type refusalTestBody struct {
	*bytes.Reader
	readErr  error
	closeErr error
}

func (b refusalTestBody) Read(p []byte) (int, error) {
	n, err := b.Reader.Read(p)
	if errors.Is(err, io.EOF) && b.readErr != nil {
		return n, b.readErr
	}
	return n, err
}
func (b refusalTestBody) Close() error { return b.closeErr }

type refusalTestTransport struct {
	body     []byte
	readErr  error
	closeErr error
	status   int
}

func (r refusalTestTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	return &http.Response{StatusCode: r.status, Request: request, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: refusalTestBody{Reader: bytes.NewReader(r.body), readErr: r.readErr, closeErr: r.closeErr}}, nil
}

func TestCloudflareRefusalRequiresCompleteBodyLayerTriad(t *testing.T) {
	t.Parallel()
	body, err := core.MarshalCanonicalJSONDocument(apiEnvelope[imageDeleteWire]{Success: new(false), Errors: []APIIssue{{Code: 5404, Message: "synthetic missing image"}}, Result: imageDeleteWire{body: []byte(`null`)}})
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name     string
		readErr  error
		closeErr error
	}{
		{name: "complete body preserves provider refusal"},
		{name: "read failure after complete JSON cannot issue a receipt", readErr: io.ErrUnexpectedEOF},
		{name: "close failure after complete JSON cannot issue a receipt", closeErr: io.ErrClosedPipe},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			client, err := exchange.NewClient(&http.Client{Transport: refusalTestTransport{body: body, status: http.StatusNotFound, readErr: tc.readErr, closeErr: tc.closeErr}})
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
			id, err := ParseImageID("codex-absence-probe")
			if err != nil {
				t.Fatal(err)
			}
			err = server.Delete(t.Context(), id, testPolicy())
			_, statusPresent := errors.AsType[exchange.StatusError](err)
			_, refusalPresent := errors.AsType[APIRefusal](err)
			wantRefusal := tc.readErr == nil && tc.closeErr == nil
			if !statusPresent || refusalPresent != wantRefusal || tc.readErr != nil && !errors.Is(err, tc.readErr) || tc.closeErr != nil && !errors.Is(err, tc.closeErr) {
				t.Fatalf("refusal complete=%t,status=%t,error=%v,want refusal %t and preserved native causes", refusalPresent, statusPresent, err, wantRefusal)
			}
		})
	}
}

func sameRefusalIssue(a, b APIIssue) bool {
	if a.Code != b.Code || a.Message != b.Message || a.DocumentationURL != b.DocumentationURL {
		return false
	}
	if a.Source == nil || b.Source == nil {
		return a.Source == nil && b.Source == nil
	}
	return *a.Source == *b.Source
}

func FuzzCloudflareFailureEnvelopeStatusConservation(f *testing.F) {
	canonical, err := core.MarshalCanonicalJSONDocument(apiEnvelope[imageDeleteWire]{Success: new(false), Errors: []APIIssue{{Code: 5404, Message: "synthetic missing image"}}, Result: imageDeleteWire{body: []byte(`null`)}})
	if err != nil {
		f.Fatal(err)
	}
	for _, door := range []uint8{0, 1, 2} {
		f.Add(canonical, uint16(4), door)
	}
	f.Add([]byte(`{"success":true,"result":{}}`), uint16(4), uint8(1))
	f.Add([]byte(`{"success":false`), uint16(3), uint8(0))
	f.Fuzz(func(t *testing.T, body []byte, statusBits uint16, door uint8) {
		status := 400 + int(statusBits)%200
		client, err := exchange.NewClient(&http.Client{Transport: refusalTestTransport{body: body, status: status}})
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
		id, err := ParseImageID("codex-absence-probe")
		if err != nil {
			t.Fatal(err)
		}
		switch door % 3 {
		case 0:
			details, cause := server.Details(t.Context(), id, testPolicy())
			err = cause
			if details.ID != (ImageID{}) || len(details.Variants) != 0 {
				t.Fatalf("refused details=%+v,want no metadata", details)
			}
		case 1:
			err = server.Delete(t.Context(), id, testPolicy())
		case 2:
			upload, cause := server.CreateDirectUpload(t.Context(), ImageDirectUploadRequest{}, testPolicy())
			err = cause
			if upload != (ImageUpload{}) {
				t.Fatalf("refused upload=%+v,want no capability", upload)
			}
		}
		native, present := errors.AsType[exchange.StatusError](err)
		if !present {
			t.Fatalf("refused status error=%v,want retained native status", err)
		}
		observed, cause := native.Status().Int()
		if cause != nil || observed != status || native.Expected() != core.HTTPStatusOK() {
			t.Fatalf("native status=%d/%v,want exact %d and expected200", observed, cause, status)
		}
		refusal, present := errors.AsType[APIRefusal](err)
		if bytes.Equal(body, canonical) && !present {
			t.Fatalf("canonical provider refusal=%v,want typed issue conservation", err)
		}
		if !present {
			return
		}
		var oracle apiEnvelope[imageDeleteWire]
		if cause := json.Unmarshal(body, &oracle); cause != nil || oracle.Success == nil || *oracle.Success || !slices.EqualFunc(refusal.Issues, oracle.Errors, sameRefusalIssue) || !errors.Is(err, core.ErrCloudflareResponse) {
			t.Fatalf("retained issues=%+v,decode=%v,error=%v,want exact false-envelope facts and typed refusal", refusal.Issues, cause, err)
		}
	})
}
