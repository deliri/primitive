package objectstore

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	json "encoding/json/v2"
	"errors"
	"hash/crc32"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/temporal"
)

const (
	gcsFuzzProviderFirstHeader            = "x-goog-meta-provider-first"
	gcsFuzzProviderSecondHeader           = "x-goog-meta-provider-second"
	gcsFuzzProviderFirstEncodedValue      = "provider-first-encoded"
	gcsFuzzProviderSecondEncodedValue     = "provider-second-encoded"
	gcsFuzzInvalidUTF8CarrierMaximumBytes = 1024
)

func FuzzGoogleCloudStorageDownloadCRC32CProviderValuesSemanticClosure(f *testing.F) {
	payload := []byte("provider response semantic closure")
	provider := newGCSDownloadFuzzProvider(f, payload)
	payloadCRC := core.NewCRC32C(crc32.Checksum(payload, crc32.MakeTable(crc32.Castagnoli)))
	payloadEncoded, gotPayloadErr := payloadCRC.Base64()
	if gotPayloadErr != nil {
		f.Fatalf("CRC32C.Base64() payload seed error = %v, want nil", gotPayloadErr)
	}
	for _, checksum := range []core.CRC32C{
		core.NewCRC32C(0),
		core.NewCRC32C(0x01020304),
		core.NewCRC32C(^uint32(0)),
	} {
		encoded, gotErr := checksum.Base64()
		if gotErr != nil {
			f.Fatalf("CRC32C.Base64() seed error = %v, want nil", gotErr)
		}
		f.Add(headerGCSChecksumPrefix+encoded, "", true, false)
		f.Add(
			"md5=1B2M2Y8AsgTpgAmY7PhCfg==",
			headerGCSChecksumPrefix+encoded,
			true, true,
		)
	}
	f.Add(headerGCSChecksumPrefix+payloadEncoded, "", true, false)
	f.Add("", "", false, false)
	f.Add("", "", true, false)
	f.Add(headerGCSChecksumPrefix, "", true, false)
	f.Add(headerGCSChecksumPrefix+"not-base64!!", "", true, false)
	f.Add(
		headerGCSChecksumPrefix+"SUYRpg==",
		headerGCSChecksumPrefix+"SUYRpg==",
		true, true,
	)
	f.Add(strings.Repeat("x", exchange.HeaderValueMaximumBytes), "", true, false)
	f.Add(strings.Repeat("x", exchange.HeaderValueMaximumBytes+1), "", true, false)

	f.Fuzz(func(t *testing.T, first string, second string, includeFirst bool, includeSecond bool) {
		if len(first) > exchange.HeaderValueMaximumBytes+1 {
			first = first[:exchange.HeaderValueMaximumBytes+1]
		}
		if len(second) > exchange.HeaderValueMaximumBytes+1 {
			second = second[:exchange.HeaderValueMaximumBytes+1]
		}
		first = boundGCSFuzzInvalidUTF8Carrier(first)
		second = boundGCSFuzzInvalidUTF8Carrier(second)
		values := make([]string, 0, 2)
		if includeFirst {
			values = append(values, first)
		}
		if includeSecond {
			values = append(values, second)
		}
		for _, value := range values {
			admitted, gotErr := exchange.NewHeaderValue(value)
			if gotErr != nil {
				if !errors.Is(gotErr, core.ErrExchangeContract) ||
					admitted != (exchange.HeaderValue{}) {
					t.Fatalf(
						"exchange.NewHeaderValue(%d bytes) = (%v, %v), want (zero, %v)",
						len(value),
						admitted,
						gotErr,
						core.ErrExchangeContract,
					)
				}
				return
			}
			if admitted == (exchange.HeaderValue{}) {
				t.Fatalf("exchange.NewHeaderValue(%d bytes) = zero, want admitted nonzero value", len(value))
			}
		}

		wantCRC, wantPresent, wantErr := independentGCSCRC32CProviderValues(values)
		if wantErr == nil && wantPresent && wantCRC != payloadCRC {
			wantErr = core.ErrObjectStoreIntegrity
		}
		request := provider.request(t, first, second, includeFirst, includeSecond)
		got, gotErr := Download(context.Background(), provider.client, request)
		destination, gotDestination := request.Destination.(*bytes.Buffer)
		if !gotDestination {
			t.Fatalf("objectstore.Download() destination type = %T, want *bytes.Buffer", request.Destination)
		}
		if !bytes.Equal(destination.Bytes(), payload) {
			t.Fatalf("objectstore.Download() destination = %d bytes, want exact %d-byte provider body", destination.Len(), len(payload))
		}
		if wantErr != nil {
			if !errors.Is(gotErr, wantErr) || got.Commitment() != CommitmentRejected || got.Validate() == nil {
				t.Fatalf(
					"objectstore.Download(rejected provider checksum) = (commitment %v, validation %v, error %v), want rejected, invalid, and errors.Is(..., %v)",
					got.Commitment(), got.Validate(), gotErr, wantErr,
				)
			}
			if got.SHA256() != (core.SHA256Digest{}) || got.CRC32C() != (core.CRC32C{}) {
				t.Fatalf("objectstore.Download(rejected provider checksum) proof = (%v, %v), want zero SHA-256 and CRC32C", got.SHA256(), got.CRC32C())
			}
			return
		}
		if gotErr != nil || got.Validate() != nil || got.Commitment() != CommitmentConfirmed ||
			got.Provider() != ProviderGoogleCloudStorage || got.Direction() != DirectionDownload ||
			got.CRC32C() != payloadCRC || got.SHA256() != core.SHA256Of(payload) {
			t.Fatalf(
				"objectstore.Download(accepted provider checksum) = (provider %v, direction %v, commitment %v, CRC32C %v, SHA-256 %v, validation %v, error %v), want exact confirmed GCS download",
				got.Provider(), got.Direction(), got.Commitment(), got.CRC32C(), got.SHA256(), got.Validate(), gotErr,
			)
		}
	})
}

type gcsDownloadFuzzProvider struct {
	client     Client
	firstName  core.HTTPHeaderName
	secondName core.HTTPHeaderName
	payload    []byte
}

func newGCSDownloadFuzzProvider(
	t testing.TB,
	payload []byte,
) gcsDownloadFuzzProvider {
	t.Helper()

	firstName, gotFirstNameErr := core.ParseHTTPHeaderName(gcsFuzzProviderFirstHeader)
	if gotFirstNameErr != nil {
		t.Fatalf("core.ParseHTTPHeaderName(first provider field) error = %v, want nil", gotFirstNameErr)
	}
	secondName, gotSecondNameErr := core.ParseHTTPHeaderName(gcsFuzzProviderSecondHeader)
	if gotSecondNameErr != nil {
		t.Fatalf("core.ParseHTTPHeaderName(second provider field) error = %v, want nil", gotSecondNameErr)
	}
	provider := gcsDownloadFuzzProvider{
		payload: bytes.Clone(payload), firstName: firstName, secondName: secondName,
	}
	server := httptest.NewTLSServer(http.HandlerFunc(provider.serveHTTP))
	t.Cleanup(server.Close)
	serverAddress := strings.TrimPrefix(server.URL, core.SchemeHTTPS+"://")
	transport := server.Client().Transport.(*http.Transport).Clone()
	transport.TLSClientConfig = transport.TLSClientConfig.Clone()
	transport.TLSClientConfig.ServerName = "example.com"
	dialer := &net.Dialer{}
	transport.DialContext = func(ctx context.Context, network string, _ string) (net.Conn, error) {
		return dialer.DialContext(ctx, network, serverAddress)
	}
	t.Cleanup(transport.CloseIdleConnections)
	exchangeClient, gotExchangeErr := exchange.NewClient(&http.Client{Transport: transport})
	if gotExchangeErr != nil {
		t.Fatalf("exchange.NewClient() error = %v, want nil", gotExchangeErr)
	}
	client, gotClientErr := NewClient(exchangeClient)
	if gotClientErr != nil {
		t.Fatalf("objectstore.NewClient() error = %v, want nil", gotClientErr)
	}
	provider.client = client
	return provider
}

func (p gcsDownloadFuzzProvider) serveHTTP(
	writer http.ResponseWriter,
	request *http.Request,
) {
	query := request.URL.Query()
	for _, key := range []string{
		gcsFuzzProviderFirstEncodedValue,
		gcsFuzzProviderSecondEncodedValue,
	} {
		if !query.Has(key) {
			continue
		}
		value, gotErr := base64.RawURLEncoding.DecodeString(query.Get(key))
		if gotErr != nil {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		writer.Header().Add(headerGCSHash, string(value))
	}
	for _, name := range []core.HTTPHeaderName{p.firstName, p.secondName} {
		for _, value := range request.Header.Values(name.String()) {
			writer.Header().Add(headerGCSHash, value)
		}
	}
	writer.Header().Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeOctetStream().String())
	writer.WriteHeader(http.StatusOK)
	_, _ = writer.Write(p.payload)
}

func (p gcsDownloadFuzzProvider) request(
	t testing.TB,
	first string,
	second string,
	includeFirst bool,
	includeSecond bool,
) DownloadCapabilityRequest {
	t.Helper()

	query := url.Values{
		queryGCSSignature: []string{"signature"},
	}
	signedFields := []string{strings.ToLower(core.HTTPHeaderHost().String())}
	callerHeaders := make([]SignedHeader, 0, 2)
	if includeFirst {
		if utf8.ValidString(first) {
			firstHeader, gotFirstErr := NewSignedHeader(p.firstName, first)
			if gotFirstErr != nil {
				t.Fatalf("NewSignedHeader(first provider value) error = %v, want nil", gotFirstErr)
			}
			callerHeaders = append(callerHeaders, firstHeader)
			signedFields = append(signedFields, strings.ToLower(p.firstName.String()))
		} else {
			query.Set(
				gcsFuzzProviderFirstEncodedValue,
				base64.RawURLEncoding.EncodeToString([]byte(first)),
			)
		}
	}
	if includeSecond {
		if utf8.ValidString(second) {
			secondHeader, gotSecondErr := NewSignedHeader(p.secondName, second)
			if gotSecondErr != nil {
				t.Fatalf("NewSignedHeader(second provider value) error = %v, want nil", gotSecondErr)
			}
			callerHeaders = append(callerHeaders, secondHeader)
			signedFields = append(signedFields, strings.ToLower(p.secondName.String()))
		} else {
			query.Set(
				gcsFuzzProviderSecondEncodedValue,
				base64.RawURLEncoding.EncodeToString([]byte(second)),
			)
		}
	}
	query.Set(queryGCSSignedHeaders, strings.Join(signedFields, signedHeaderTokenSeparator))
	signed, gotSignedErr := ParseSignedURL(capabilityGCSObject + "?" + query.Encode())
	if gotSignedErr != nil {
		t.Fatalf("ParseSignedURL() error = %v, want nil", gotSignedErr)
	}
	headers, gotHeadersErr := NewSignedHeaders(callerHeaders)
	if gotHeadersErr != nil {
		t.Fatalf("NewSignedHeaders() error = %v, want nil", gotHeadersErr)
	}
	expiresAt, gotExpiryErr := temporal.NewInstant(time.Date(2099, time.January, 1, 0, 0, 0, 0, time.UTC))
	if gotExpiryErr != nil {
		t.Fatalf("temporal.NewInstant() error = %v, want nil", gotExpiryErr)
	}
	projection, gotProjectionErr := NewDownloadCapabilityProjection(
		ProviderGoogleCloudStorage,
		DownloadTarget{URL: signed, Headers: headers, ExpiresAt: expiresAt},
	)
	if gotProjectionErr != nil {
		t.Fatalf("NewDownloadCapabilityProjection() error = %v, want nil", gotProjectionErr)
	}
	encoded, gotMarshalErr := projection.MarshalJSON()
	if gotMarshalErr != nil {
		t.Fatalf("DownloadCapabilityProjection.MarshalJSON() error = %v, want nil", gotMarshalErr)
	}
	var capability DownloadCapability
	if gotDecodeErr := json.Unmarshal(encoded, &capability); gotDecodeErr != nil {
		t.Fatalf("DownloadCapability.UnmarshalJSON() error = %v, want nil", gotDecodeErr)
	}
	length, gotLengthErr := core.NewByteLength(uint64(len(p.payload)))
	if gotLengthErr != nil {
		t.Fatalf("core.NewByteLength(%d) error = %v, want nil", len(p.payload), gotLengthErr)
	}
	errorBodyLimit, gotLimitErr := core.NewByteCount(4096)
	if gotLimitErr != nil {
		t.Fatalf("core.NewByteCount() error = %v, want nil", gotLimitErr)
	}
	operationTimeout, gotOperationErr := temporal.DurationFromSeconds(10)
	if gotOperationErr != nil {
		t.Fatalf("temporal.DurationFromSeconds(10) error = %v, want nil", gotOperationErr)
	}
	attemptTimeout, gotAttemptErr := temporal.DurationFromSeconds(5)
	if gotAttemptErr != nil {
		t.Fatalf("temporal.DurationFromSeconds(5) error = %v, want nil", gotAttemptErr)
	}
	destination := &bytes.Buffer{}
	return DownloadCapabilityRequest{
		Destination: destination,
		ContentType: core.HTTPMediaTypeOctetStream(),
		Capability:  capability,
		Integrity: Integrity{
			SHA256: core.SHA256Of(p.payload),
			Length: length,
			CRC32C: core.NewCRC32C(crc32.Checksum(p.payload, crc32.MakeTable(crc32.Castagnoli))),
		},
		Policy: Policy{
			OperationTimeout: operationTimeout,
			AttemptTimeout:   attemptTimeout,
			ErrorBodyLimit:   errorBodyLimit,
		},
	}
}

func boundGCSFuzzInvalidUTF8Carrier(value string) string {
	if utf8.ValidString(value) || len(value) <= gcsFuzzInvalidUTF8CarrierMaximumBytes {
		return value
	}
	return value[:gcsFuzzInvalidUTF8CarrierMaximumBytes]
}

func independentGCSCRC32CProviderValues(
	values []string,
) (core.CRC32C, bool, error) {
	encoded := ""
	count := 0
	for _, value := range values {
		for component := range strings.SplitSeq(value, gcsHashComponentSeparator) {
			trimmed := strings.TrimSpace(component)
			if !strings.HasPrefix(trimmed, headerGCSChecksumPrefix) {
				continue
			}
			encoded = strings.TrimPrefix(trimmed, headerGCSChecksumPrefix)
			count++
		}
	}
	if count == 0 {
		return core.CRC32C{}, false, nil
	}
	if count != 1 {
		return core.CRC32C{}, false, core.ErrObjectStoreIntegrity
	}
	raw, gotErr := base64.StdEncoding.DecodeString(encoded)
	if gotErr != nil || len(raw) != crc32.Size ||
		base64.StdEncoding.EncodeToString(raw) != encoded {
		return core.CRC32C{}, false, core.ErrObjectStoreIntegrity
	}
	return core.NewCRC32C(binary.BigEndian.Uint32(raw)), true, nil
}
