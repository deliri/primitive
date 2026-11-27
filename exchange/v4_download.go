package exchange

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsv4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// V4DownloadSigning borrows credentials for a single server-side HTTPS GET.
type V4DownloadSigning struct {
	Region    string
	Service   string
	AccessKey []byte
	SecretKey []byte
	SignedAt  temporal.Instant
}

func (V4DownloadSigning) Format(state fmt.State, _ rune) {
	_, _ = io.WriteString(state, core.RedactedValueText)
}

// DownloadV4 signs the actual native GET, with an empty payload and the
// exact escaped path, immediately before Exchange sends it. Redirects are
// refused so the authorization can only reach the signed endpoint.
func DownloadV4(call DownloadCall, signing V4DownloadSigning) (StreamResponse, error) {
	if call.Policy.Redirect.Mode != RedirectReject || call.Request.Semantics.Method != MethodGet ||
		len(call.Request.SecretHeaders) != 0 || len(call.Request.Headers.Values) != 0 {
		return StreamResponse{}, core.ErrExchangeRequest
	}
	target, err := validatedTarget(call.Request.Target)
	if err != nil {
		return StreamResponse{}, err
	}
	digest := sha256.Sum256(nil)
	payload := hex.EncodeToString(digest[:])
	validated := V4PresignRequest{
		Target: target, Method: MethodGet, Region: signing.Region, Service: signing.Service,
		AccessKey: signing.AccessKey, SecretKey: signing.SecretKey, SignedAt: signing.SignedAt, PayloadHash: payload,
	}
	if err := validated.Validate(); err != nil {
		return StreamResponse{}, err
	}
	stamp, err := signing.SignedAt.Time()
	if err != nil {
		return StreamResponse{}, requestError(err)
	}
	call.Request.Target = target
	return download(call, func(request *http.Request) error {
		request.Header.Set("X-Amz-Content-Sha256", payload)
		credentials := aws.Credentials{AccessKeyID: string(signing.AccessKey), SecretAccessKey: string(signing.SecretKey)}
		return awsv4.NewSigner().SignHTTP(request.Context(), credentials, request, payload, signing.Service, signing.Region, stamp,
			func(options *awsv4.SignerOptions) { options.DisableURIPathEscaping = true })
	})
}
