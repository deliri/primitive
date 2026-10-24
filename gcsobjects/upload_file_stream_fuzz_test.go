package gcsobjects

import (
	"bytes"
	"errors"
	"net/http"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

// The source is an arbitrary stored blob, not a JSON protocol. Each callback
// sends its actual mutated bytes through the official SDK. The provider folds
// them independently; a forged provider CRC or extent must release no proof.
func FuzzGCSFileStreamProviderIntegritySemanticClosure(f *testing.F) {
	seed := []byte("streamed evidence\x00\xff")
	request := GCSFileStreamUpload{Source: bytes.NewReader(seed), Bucket: parsedGCSBucket(f, gcsProviderBucketText), Name: parsedGCSObjectName(f, gcsProviderObjectText), CustomTime: temporal.InstantFromNanoseconds(gcsProviderInstantNanos)}
	if err := request.Validate(); err != nil {
		f.Fatal(err)
	}
	f.Add(seed, uint8(0))
	f.Add([]byte{}, uint8(0))
	f.Add(seed, uint8(1))
	f.Add(seed, uint8(2))
	f.Fuzz(func(t *testing.T, data []byte, mutation uint8) {
		if len(data) > 64<<10 {
			t.Skip("local provider fuzz campaign bounds per-callback network work to 64 KiB")
		}
		mode := mutation % 3
		client := bucketTestClient(t, http.HandlerFunc(func(out http.ResponseWriter, incoming *http.Request) {
			metadata, received, err := receiveGCSMultipart(incoming)
			if err != nil {
				t.Errorf("provider receive=%v, want complete multipart bytes", err)
				out.WriteHeader(http.StatusBadRequest)
				return
			}
			if metadata.Name != gcsProviderObjectText || metadata.ContentType != gcsProviderMediaTypeText || incoming.URL.Query().Get("ifGenerationMatch") != "0" {
				t.Errorf("upload metadata=%+v at %s, want typed create-only stored object", metadata, incoming.URL)
				out.WriteHeader(http.StatusBadRequest)
				return
			}
			before := received
			switch mode {
			case 0:
			case 1:
				checksum, err := received.CRC32C.Uint32()
				if err != nil {
					t.Error(err)
					out.WriteHeader(http.StatusInternalServerError)
					return
				}
				received.CRC32C = core.NewCRC32C(checksum ^ 1)
			case 2:
				received.Length, err = core.NewByteLength(received.Length.Uint64() + 1)
				if err != nil {
					t.Error(err)
					out.WriteHeader(http.StatusInternalServerError)
					return
				}
			}
			if mode != 0 && received == before {
				t.Errorf("provider mutation=%v, want different integrity from %v", received, before)
				out.WriteHeader(http.StatusInternalServerError)
				return
			}
			writeGCSObjectResponse(gcsObjectResponse{t: t, writer: out, metadata: metadata, integrity: received})
		}))
		request := GCSFileStreamUpload{Source: bytes.NewReader(data), Bucket: parsedGCSBucket(t, gcsProviderBucketText), Name: parsedGCSObjectName(t, gcsProviderObjectText), CustomTime: temporal.InstantFromNanoseconds(gcsProviderInstantNanos)}
		got, err := UploadFileStream(t.Context(), client, request)
		if mode != 0 {
			wantErr := core.ErrObjectStoreIntegrity
			if mode == 1 {
				// The official SDK refuses its computed CRC disagreement at
				// Close, before releasing response metadata to this package.
				wantErr = core.ErrObjectStoreDestination
			}
			if !errors.Is(err, wantErr) || got != (GCSFileStreamResult{}) {
				t.Fatalf("forged provider result=%+v/%v, want zero and %v", got, err, wantErr)
			}
			return
		}
		integrity, integrityErr := got.Integrity()
		metadata, metadataErr := got.Metadata()
		want := gcsProviderIntegrity(t, data, data)
		if err != nil || integrityErr != nil || metadataErr != nil || got.Validate() != nil || integrity != want || metadata.Length() != want.Length || metadata.CRC32C() != want.CRC32C {
			t.Fatalf("accepted stream=%v/%v, metadata=%v/%v, want exactly %v and a valid generation", integrity, err, metadata, metadataErr, want)
		}
	})
}
