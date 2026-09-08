package awsidentity

import (
	"bytes"
	"encoding/xml"
	"errors"
	"net/http"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestAWSAcquireRequiresOneCompleteXMLDocument(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name           string
		prefix, suffix []byte
		wantErr        error
	}{
		{name: "one complete document"},
		{name: "leading XML whitespace", prefix: []byte(" \t\r\n")},
		{name: "trailing XML whitespace", suffix: []byte(" \t\r\n")},
		{name: "trailing comment", suffix: []byte("<!-- provider comment -->")},
		{name: "leading XML declaration", prefix: []byte(xml.Header)},
		{name: "leading processing instruction", prefix: []byte("<?provider receipt?>")},
		{name: "trailing processing instruction", suffix: []byte("<?provider receipt?>")},
		{name: "trailing XML declaration", suffix: []byte(xml.Header), wantErr: core.ErrAWSIdentityContract},
		{name: "non XML whitespace after root", suffix: []byte("\u00a0"), wantErr: core.ErrAWSIdentityContract},

		{name: "second root", suffix: []byte("<Future/>"), wantErr: core.ErrAWSIdentityContract},
		{name: "trailing truncated tag", suffix: []byte("<"), wantErr: core.ErrAWSIdentityContract},
		{name: "trailing unmatched close", suffix: []byte("</Future>"), wantErr: core.ErrAWSIdentityContract},
		{name: "trailing text", suffix: []byte("not an XML document"), wantErr: core.ErrAWSIdentityContract},
		{name: "leading text", prefix: []byte("not an XML document"), wantErr: core.ErrAWSIdentityContract},
		{name: "trailing invalid UTF8", suffix: []byte{0xff}, wantErr: core.ErrAWSIdentityContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			canonical := awsProviderBytes(t, awsProviderDocument(awsTestBearer))
			data := append(bytes.Clone(tc.prefix), canonical...)
			data = append(data, tc.suffix...)
			body := &awsObservedBody{reader: bytes.NewReader(data)}
			transport := &awsResponseTransport{body: body, status: http.StatusOK, length: int64(len(data))}
			got, err := Acquire(t.Context(), awsClient(t, transport), awsRequest(t))
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("Acquire XML framing error = %v, want %v", err, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got != (Token{}) {
					t.Fatalf("XML framing refusal = %v, want zero", got)
				}
			} else {
				value, err := got.BearerValue()
				if err != nil || value != bearerPrefix+awsTestBearer {
					t.Fatalf("XML framing disclosure = (%q,%v), want exact token", value, err)
				}
			}
			if transport.calls != 1 || body.closes != 1 || body.bytes != len(data) {
				t.Fatalf("XML framing effect = %d/%d/%d, want 1/1/%d", transport.calls, body.closes, body.bytes, len(data))
			}
		})
	}
}

func TestAWSAcquireReorderedEnvelopePreservesExactToken(t *testing.T) {
	t.Parallel()
	document := awsProviderDocument(awsTestBearer)
	var buffer bytes.Buffer
	encoder := xml.NewEncoder(&buffer)
	root := xml.StartElement{Name: document.XMLName}
	if err := encoder.EncodeToken(root); err != nil {
		t.Fatalf("root fixture error = %v, want nil", err)
	}
	if err := encoder.EncodeElement(document.Metadata[0], xml.StartElement{Name: document.Metadata[0].XMLName}); err != nil {
		t.Fatalf("metadata fixture error = %v, want nil", err)
	}
	if err := encoder.EncodeElement(document.Results[0], xml.StartElement{Name: document.Results[0].XMLName}); err != nil {
		t.Fatalf("result fixture error = %v, want nil", err)
	}
	if err := encoder.EncodeToken(root.End()); err != nil {
		t.Fatalf("root closing fixture error = %v, want nil", err)
	}
	if err := encoder.Close(); err != nil {
		t.Fatalf("fixture close error = %v, want nil", err)
	}
	data := buffer.Bytes()
	if bytes.Equal(data, awsProviderBytes(t, document)) {
		t.Fatalf("reordered document=%x, want metadata before result", data)
	}
	body := &awsObservedBody{reader: bytes.NewReader(data)}
	transport := &awsResponseTransport{body: body, status: http.StatusOK, length: int64(len(data))}
	got, err := Acquire(t.Context(), awsClient(t, transport), awsRequest(t))
	value, discloseErr := got.BearerValue()
	if err != nil || discloseErr != nil || value != bearerPrefix+awsTestBearer || transport.calls != 1 || body.closes != 1 || body.bytes != len(data) {
		t.Fatalf("reordered Acquire = (%q,%v,%v,%d/%d/%d), want exact token and 1/1/%d", value, err, discloseErr, transport.calls, body.closes, body.bytes, len(data))
	}
}
