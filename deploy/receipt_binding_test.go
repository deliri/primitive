package deploy

import (
	"bytes"
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/release"
	"github.com/deliri/primitive/v2026/temporal"
	"hash/crc32"
	"io"
	"net/http"
	"testing"
)

type receiptBoundaryTransport struct {
	fail  bool
	calls int
}

func (r *receiptBoundaryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r.calls++
	if r.fail {
		return nil, errors.Join(io.ErrClosedPipe, req.Body.Close())
	}
	_, readErr := io.Copy(io.Discard, req.Body)
	if err := errors.Join(readErr, req.Body.Close()); err != nil {
		return nil, err
	}
	h := make(http.Header)
	h.Set("X-Goog-Generation", "1")
	return &http.Response{StatusCode: http.StatusOK, Header: h, Body: http.NoBody, ContentLength: 0, Request: req}, nil
}
func receiptBoundaryCapability(t testing.TB, object string) objectstore.UploadCapability {
	t.Helper()
	u, err := objectstore.ParseSignedURL("https://storage.googleapis.com/bucket/" + object + "?X-Goog-Signature=signature&X-Goog-SignedHeaders=host%3Bx-goog-hash%3Bx-goog-if-generation-match")
	if err != nil {
		t.Fatalf("ParseSignedURL()=%v, want nil", err)
	}
	headers, err := objectstore.NewSignedHeaders(nil)
	if err != nil {
		t.Fatalf("NewSignedHeaders()=%v, want nil", err)
	}
	p, err := objectstore.NewUploadCapabilityProjection(objectstore.ProviderGoogleCloudStorage, objectstore.UploadTarget{URL: u, Headers: headers, ExpiresAt: temporal.InstantFromNanoseconds(2_051_222_400_000_000_000)})
	if err != nil {
		t.Fatalf("NewUploadCapabilityProjection()=%v, want nil", err)
	}
	wire, err := p.MarshalJSON()
	if err != nil {
		t.Fatalf("capability MarshalJSON()=%v, want nil", err)
	}
	var c objectstore.UploadCapability
	if err := c.UnmarshalJSON(wire); err != nil {
		t.Fatalf("capability UnmarshalJSON()=%v, want nil", err)
	}
	return c
}
func TestReceiptCapabilityBindingLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                       string
		foreign, raw, fail, absent bool
		wantErr                    error
	}{
		{name: "confirmed capability matches receipt"},
		{name: "confirmed transfer under another granted destination", foreign: true, wantErr: core.ErrDeployContract},
		{name: "confirmed raw provider transfer has no grant identity", raw: true, wantErr: core.ErrDeployContract},
		{name: "attempt identity cannot turn failed upload into receipt", fail: true, wantErr: core.ErrDeployContract},
		{name: "no transfer cannot create receipt", absent: true, wantErr: core.ErrDeployContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := receiptBoundaryCapability(t, "original")
			commitment, err := c.Commitment()
			if err != nil {
				t.Fatalf("Commitment()=%v, want nil", err)
			}
			transport := &receiptBoundaryTransport{fail: tc.fail}
			ec, err := exchange.NewClient(&http.Client{Transport: transport})
			if err != nil {
				t.Fatalf("Exchange.NewClient()=%v, want nil", err)
			}
			client, err := objectstore.NewClient(ec)
			if err != nil {
				t.Fatalf("Objectstore.NewClient()=%v, want nil", err)
			}
			payload := []byte("one exact receipt")
			length, err := core.NewByteLength(uint64(len(payload)))
			if err != nil {
				t.Fatalf("NewByteLength()=%v, want nil", err)
			}
			integrity := objectstore.Integrity{Length: length, SHA256: core.SHA256Of(payload), CRC32C: core.NewCRC32C(crc32.Checksum(payload, crc32.MakeTable(crc32.Castagnoli)))}
			operation, err := temporal.DurationFromSeconds(10)
			if err != nil {
				t.Fatalf("operation duration=%v, want nil", err)
			}
			attempt, err := temporal.DurationFromSeconds(5)
			if err != nil {
				t.Fatalf("attempt duration=%v, want nil", err)
			}
			limit, err := core.NewByteCount(4096)
			if err != nil {
				t.Fatalf("error-body bound=%v, want nil", err)
			}
			policy := objectstore.Policy{OperationTimeout: operation, AttemptTimeout: attempt, ErrorBodyLimit: limit}
			var transfer objectstore.Transfer
			if !tc.absent {
				if tc.raw {
					target, targetErr := c.Target()
					if targetErr != nil {
						t.Fatalf("Target()=%v, want nil", targetErr)
					}
					transfer, err = objectstore.UploadGCS(t.Context(), client, objectstore.UploadRequest{Source: bytes.NewReader(payload), Target: target, ContentType: core.HTTPMediaTypeOctetStream(), Integrity: integrity, Policy: policy})
				} else {
					transfer, err = objectstore.Upload(t.Context(), client, objectstore.UploadCapabilityRequest{Source: bytes.NewReader(payload), Capability: c, ContentType: core.HTTPMediaTypeOctetStream(), Integrity: integrity, Policy: policy})
				}
				if tc.fail {
					if !errors.Is(err, io.ErrClosedPipe) || transfer.Commitment() == objectstore.CommitmentConfirmed {
						t.Fatalf("failed transfer=(%v,%v), want native refusal and no confirmation", transfer.Commitment(), err)
					}
					evidence, evidenceErr := transfer.Evidence()
					wire, wireErr := evidence.MarshalJSON()
					if !errors.Is(evidenceErr, core.ErrObjectStoreContract) || wire != nil || !errors.Is(wireErr, core.ErrJSONContract) {
						t.Fatalf("failed evidence=(%v,%q,%v), want refusal and no wire", evidenceErr, wire, wireErr)
					}
					got, present := transfer.UploadCapability()
					if !present || got != commitment {
						t.Fatalf("attempt capability=(%v,%t), want (%v,true)", got, present, commitment)
					}
				} else if err != nil || transfer.Validate() != nil {
					t.Fatalf("upload=(%v,%v), want confirmed transfer", transfer, err)
				}
			}
			wantCalls := 1
			if tc.absent {
				wantCalls = 0
			}
			if transport.calls != wantCalls {
				t.Fatalf("transport calls=%d, want %d", transport.calls, wantCalls)
			}
			if tc.foreign {
				foreign, foreignErr := receiptBoundaryCapability(t, "foreign").Commitment()
				if foreignErr != nil || foreign == commitment {
					t.Fatalf("foreign commitment=(%v,%v), want distinct valid commitment", foreign, foreignErr)
				}
				commitment = foreign
			}
			r := Receipt{transfer: transfer, commitment: commitment, role: release.PublicationRoleManifest, valid: true}
			if err := r.Validate(); !errors.Is(err, tc.wantErr) {
				t.Fatalf("Receipt.Validate()=%v, want %v", err, tc.wantErr)
			}
		})
	}
}
