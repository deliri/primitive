package distributionauth

import (
	"bytes"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/distribution"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/permit"
)

// MarshalJSON alone skipped the exchange encoder's receive-only projection
// check. This regression drives the HTTP writer used by the deployed API.
func TestUpgradeDeliveryHTTPWriterLayerTriad(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		wantErr        error
		name           string
		removeDownload bool
		removeTransfer bool
	}{
		{name: "signed download and unchanged permission reach receiving contract"},
		{name: "missing download emits no partial permission", removeDownload: true, wantErr: core.ErrDistributionContract},
		{name: "missing permission emits no partial download", removeTransfer: true, wantErr: core.ErrPermitContract},
		{name: "zero delivery emits no response", removeDownload: true, removeTransfer: true, wantErr: core.ErrDistributionContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			projection, download, transfer := upgradeDeliveryFixture(t)
			if tc.removeDownload {
				projection.Download = distribution.UpgradeGrantProjection{}
			}
			if tc.removeTransfer {
				projection.Transfer = permit.BuildTransfer{}
			}
			writer := httptest.NewRecorder()
			call, err := exchange.NewSocketServerCall(writer, httptest.NewRequest(http.MethodPost, "/", nil))
			if err != nil {
				t.Fatalf("NewSocketServerCall() error = %v, want nil", err)
			}
			err = exchange.WriteJSON(exchange.JSONWriteCall[UpgradeDeliveryProjection]{Call: call, Response: exchange.ServerJSONResponse[UpgradeDeliveryProjection]{Body: projection, Status: core.HTTPStatusOK()}})
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) || writer.Body.Len() != 0 || len(writer.Header()) != 0 {
					t.Fatalf("refused writer = (%v, %d bytes, %v), want typed %v and no response", err, writer.Body.Len(), writer.Header(), tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("WriteJSON() error = %v, want nil and usable delivery", err)
			}
			canonical, err := projection.MarshalJSON()
			if err != nil || !bytes.Equal(writer.Body.Bytes(), canonical) || writer.Code != http.StatusOK || writer.Header().Get("Content-Type") != "application/json" {
				t.Fatalf("HTTP delivery = (%d bytes, status %d, type %q, %v), want exact canonical JSON and 200", writer.Body.Len(), writer.Code, writer.Header().Get("Content-Type"), err)
			}
			var got UpgradeDeliveryDocument
			if err := got.UnmarshalJSON(writer.Body.Bytes()); err != nil {
				t.Fatalf("client decode = %v, want nil", err)
			}
			download.Document, transfer.Document = got.Download, got.Transfer
			if _, err := distribution.VerifyUpgradeGrant(download); err != nil {
				t.Fatalf("client download verification = %v, want nil", err)
			}
			if _, err := permit.VerifyBuildTransfer(transfer); err != nil {
				t.Fatalf("client permission verification = %v, want nil", err)
			}
		})
	}
}
