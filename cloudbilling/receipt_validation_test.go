package cloudbilling

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestListSKUsRequestValidatePreservesOpaqueTokenGrammar(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		token   PageToken
		proof   io.Writer
		wantErr error
	}{
		{name: "empty cursor starts first page", proof: io.Discard},
		{name: "provider punctuation remains opaque", token: "/+=?&", proof: io.Discard},
		{name: "unicode cursor remains opaque", token: "東京", proof: io.Discard},
		{name: "NUL remains existing valid UTF8 token", token: "a\x00b", proof: io.Discard},
		{name: "SDK adds no Kernel4096 byte quota", token: PageToken(strings.Repeat("x", 4097)), proof: io.Discard},
		{name: "missing proof refuses", wantErr: core.ErrCloudBillingContract},
		{name: "typed nil proof refuses", proof: (*bytes.Buffer)(nil), wantErr: core.ErrCloudBillingContract},
		{name: "invalid UTF8 refuses", token: PageToken(string([]byte{0xff})), proof: io.Discard, wantErr: core.ErrCloudBillingContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := (ListSKUsRequest{Token: tc.token, Proof: tc.proof}).Validate()
			if !errors.Is(got, tc.wantErr) {
				t.Fatalf("ListSKUsRequest.Validate()=%v,want %v", got, tc.wantErr)
			}
		})
	}
}

func TestPageReceiptValidateSourceLayerTriad(t *testing.T) {
	t.Parallel()
	endpoint, err := core.ParseHTTPEndpoint("https://cloudbilling.googleapis.com/v1/services/service/skus?currencyCode=USD&pageSize=5&pageToken=%2F%2B")
	if err != nil {
		t.Fatal(err)
	}
	extent, err := core.NewByteLength(2)
	if err != nil {
		t.Fatal(err)
	}
	source := SourceReceipt{URL: endpoint, SHA256: core.SHA256Of([]byte("{}")), Bytes: extent}
	for _, tc := range []struct {
		name          string
		change        func(*PageReceipt)
		wantSourceErr error
		wantPageErr   error
		wantCause     error
	}{
		{name: "empty provider page retains actual queried source", change: func(*PageReceipt) {}},
		{name: "provider continuation needs no SKU eligibility", change: func(r *PageReceipt) { r.Page.Next = "next /+" }},
		{name: "missing URL cannot certify source", change: func(r *PageReceipt) { r.Source.URL = core.HTTPEndpoint{} }, wantSourceErr: core.ErrCloudBillingContract, wantPageErr: core.ErrCloudBillingContract},
		{name: "missing digest cannot certify source", change: func(r *PageReceipt) { r.Source.SHA256 = core.SHA256Digest{} }, wantSourceErr: core.ErrCloudBillingContract, wantPageErr: core.ErrCloudBillingContract},
		{name: "zero original bytes cannot contain a JSON object", change: func(r *PageReceipt) { r.Source.Bytes = core.ByteLength{} }, wantSourceErr: core.ErrCloudBillingContract, wantPageErr: core.ErrCloudBillingContract},
		{name: "invalid page preserves response cause", change: func(r *PageReceipt) { r.Page.Next = PageToken(string([]byte{0xff})) }, wantPageErr: core.ErrCloudBillingContract, wantCause: core.ErrCloudBillingResponse},
		{name: "zero receipt cannot invent provider evidence", change: func(r *PageReceipt) { *r = PageReceipt{} }, wantSourceErr: core.ErrCloudBillingContract, wantPageErr: core.ErrCloudBillingContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			receipt := PageReceipt{Source: source}
			tc.change(&receipt)
			if got := receipt.Source.Validate(); !errors.Is(got, tc.wantSourceErr) {
				t.Fatalf("SourceReceipt.Validate()=%v,want %v", got, tc.wantSourceErr)
			}
			got := receipt.Validate()
			if !errors.Is(got, tc.wantPageErr) || tc.wantCause != nil && !errors.Is(got, tc.wantCause) {
				t.Fatalf("PageReceipt.Validate()=%v,want %v preserving %v", got, tc.wantPageErr, tc.wantCause)
			}
		})
	}
}
