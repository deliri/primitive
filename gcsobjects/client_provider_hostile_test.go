package gcsobjects

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"errors"
	"hash/crc32"
	"io"
	"io/fs"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"cloud.google.com/go/iam"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/objectstore"
	"github.com/deliri/primitive/v2026/temporal"
	"github.com/deliri/primitive/v2026/testserial"
	"google.golang.org/api/googleapi"
	storageapi "google.golang.org/api/storage/v1"
)

const (
	gcsProviderBucketText       = "primitive-object-tests"
	gcsProviderObjectText       = "users/01/evidence/result.bin"
	gcsProviderMediaTypeText    = "application/octet-stream"
	gcsProviderCacheControlText = "private, no-store"
	gcsProviderGeneration       = int64(1_786_000_000_000_001)
	gcsProviderInstantNanos     = int64(1_786_183_200_000_000_000)
)

var gcsProviderPayload = []byte("authenticated provider evidence\n")

type gcsUploadKind uint8

const (
	gcsUploadMedia gcsUploadKind = iota + 1
	gcsUploadFile
)

type gcsProviderDisposition uint8

const (
	gcsProviderAccept gcsProviderDisposition = iota + 1
	gcsProviderReject
	gcsProviderConflict
	gcsProviderRejectIntegrity
	gcsProviderSourceAbort
)

type gcsUploadCase struct {
	name        string
	source      []byte
	wantBytes   []byte
	crcSource   []byte
	wantErr     core.ErrorIdentity
	kind        gcsUploadKind
	disposition gcsProviderDisposition
}

type gcsUploadProvider struct {
	t             testing.TB
	wantIntegrity objectstore.Integrity
	disposition   gcsProviderDisposition
	received      atomic.Int64
}

func TestAuthenticatedGCSUploadsExecuteTheOfficialSDKAndRefuseBoundaryCorruption(t *testing.T) {
	t.Parallel()

	cases := []gcsUploadCase{
		{name: "served media exact non-empty source", kind: gcsUploadMedia, source: gcsProviderPayload, wantBytes: gcsProviderPayload, crcSource: gcsProviderPayload, disposition: gcsProviderAccept},
		{name: "stored file exact non-empty source", kind: gcsUploadFile, source: gcsProviderPayload, wantBytes: gcsProviderPayload, crcSource: gcsProviderPayload, disposition: gcsProviderAccept},
		{name: "served media exact empty source", kind: gcsUploadMedia, source: nil, wantBytes: nil, crcSource: nil, disposition: gcsProviderAccept},
		{name: "stored file exact one-byte source", kind: gcsUploadFile, source: []byte{0x7f}, wantBytes: []byte{0x7f}, crcSource: []byte{0x7f}, disposition: gcsProviderAccept},
		{name: "short source refuses before provider commitment", kind: gcsUploadFile, source: gcsProviderPayload[:len(gcsProviderPayload)-1], wantBytes: gcsProviderPayload, crcSource: gcsProviderPayload, disposition: gcsProviderSourceAbort, wantErr: core.ErrObjectStoreSource},
		{name: "long source refuses before provider commitment", kind: gcsUploadFile, source: append(bytes.Clone(gcsProviderPayload), 0x7f), wantBytes: gcsProviderPayload, crcSource: gcsProviderPayload, disposition: gcsProviderSourceAbort, wantErr: core.ErrObjectStoreSource},
		{name: "wrong digest refuses before provider commitment", kind: gcsUploadMedia, source: gcsProviderPayload, wantBytes: append([]byte("x"), gcsProviderPayload[1:]...), crcSource: gcsProviderPayload, disposition: gcsProviderSourceAbort, wantErr: core.ErrObjectStoreSource},
		{name: "wrong declared checksum is rejected by provider", kind: gcsUploadFile, source: gcsProviderPayload, wantBytes: gcsProviderPayload, crcSource: append([]byte("x"), gcsProviderPayload[1:]...), disposition: gcsProviderRejectIntegrity, wantErr: core.ErrObjectStoreDestination},
		{name: "provider conflict retains stable conflict identity", kind: gcsUploadMedia, source: gcsProviderPayload, wantBytes: gcsProviderPayload, crcSource: gcsProviderPayload, disposition: gcsProviderConflict, wantErr: core.ErrObjectStoreConflict},
		{name: "provider rejection retains destination identity", kind: gcsUploadFile, source: gcsProviderPayload, wantBytes: gcsProviderPayload, crcSource: gcsProviderPayload, disposition: gcsProviderReject, wantErr: core.ErrObjectStoreDestination},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			integrity := gcsProviderIntegrity(t, tc.wantBytes, tc.crcSource)
			provider := &gcsUploadProvider{t: t, wantIntegrity: integrity, disposition: tc.disposition}
			client := bucketTestClient(t, provider)
			got, gotErr := executeGCSUpload(gcsUploadExecution{t: t, client: client, testCase: tc, integrity: integrity})
			if !errors.Is(tc.wantErr, core.ErrUnknown) {
				if !errors.Is(gotErr, tc.wantErr) || got != (GCSObjectMetadata{}) {
					t.Fatalf("authenticated upload = (%v, %v), want zero and errors.Is(..., %v)", got, gotErr, tc.wantErr)
				}
				return
			}
			if gotErr != nil || got.Validate() != nil {
				t.Fatalf("authenticated upload = (%v, %v), want validated metadata and nil", got, gotErr)
			}
			if got.Bucket() != parsedGCSBucket(t, gcsProviderBucketText) ||
				got.Name() != parsedGCSObjectName(t, gcsProviderObjectText) ||
				got.Length() != integrity.Length || got.CRC32C() != integrity.CRC32C {
				t.Fatalf("authenticated upload metadata = %v, want exact provider evidence", got)
			}
			if provider.received.Load() != 1 {
				t.Fatalf("provider accepted calls = %d, want exactly one", provider.received.Load())
			}
		})
	}
}

type gcsUploadExecution struct {
	t         testing.TB
	client    *GCSClient
	testCase  gcsUploadCase
	integrity objectstore.Integrity
}

func executeGCSUpload(execution gcsUploadExecution) (GCSObjectMetadata, error) {
	execution.t.Helper()
	t := execution.t
	client := execution.client
	tc := execution.testCase
	integrity := execution.integrity
	customTime := temporal.InstantFromNanoseconds(gcsProviderInstantNanos)
	if tc.kind == gcsUploadFile {
		return UploadFile(context.Background(), client, GCSFileUpload{
			Source: bytes.NewReader(tc.source), Bucket: parsedGCSBucket(t, gcsProviderBucketText),
			Name: parsedGCSObjectName(t, gcsProviderObjectText), Integrity: integrity, CustomTime: customTime,
		})
	}
	mediaType, mediaErr := core.ParseHTTPMediaType(gcsProviderMediaTypeText)
	cacheControl, cacheErr := ParseGCSCacheControl(gcsProviderCacheControlText)
	if mediaErr != nil || cacheErr != nil {
		t.Fatalf("upload media fixture errors = (%v, %v), want nil", mediaErr, cacheErr)
	}
	return UploadMedia(context.Background(), client, GCSMediaUpload{
		Source: bytes.NewReader(tc.source), Bucket: parsedGCSBucket(t, gcsProviderBucketText),
		Name: parsedGCSObjectName(t, gcsProviderObjectText), Integrity: integrity,
		ContentType: mediaType, CacheControl: cacheControl, CustomTime: customTime,
	})
}

func (p *gcsUploadProvider) ServeHTTP(writer http.ResponseWriter, incoming *http.Request) {
	if incoming.Method == exchange.MethodGet.String() {
		writeGCSPolicy(p.t, writer, gcsPolicy("upload-public-read", gcsPublicReadBinding(iam.AllUsers)))
		return
	}
	if incoming.Method != exchange.MethodPost.String() {
		p.t.Errorf("upload provider method = %q, want %q", incoming.Method, exchange.MethodPost.String())
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	metadata, payloadIntegrity, err := receiveGCSMultipart(incoming)
	if err != nil {
		if p.disposition == gcsProviderSourceAbort {
			return
		}
		p.t.Errorf("upload provider request error = %v, want nil", err)
		writer.WriteHeader(http.StatusBadRequest)
		return
	}
	if err := p.verifyUpload(metadata, payloadIntegrity); err != nil {
		if p.disposition == gcsProviderRejectIntegrity {
			writer.WriteHeader(http.StatusBadRequest)
			return
		}
		p.t.Errorf("upload provider semantic error = %v, want nil", err)
		writer.WriteHeader(http.StatusBadRequest)
		return
	}
	switch p.disposition {
	case gcsProviderConflict:
		writeGoogleAPIError(writer, http.StatusPreconditionFailed)
		return
	case gcsProviderReject:
		writeGoogleAPIError(writer, http.StatusBadRequest)
		return
	}
	p.received.Add(1)
	writeGCSObjectResponse(gcsObjectResponse{t: p.t, writer: writer, metadata: metadata, integrity: p.wantIntegrity})
}

func receiveGCSMultipart(incoming *http.Request) (storageapi.Object, objectstore.Integrity, error) {
	_, parameters, err := mime.ParseMediaType(incoming.Header.Get(core.HTTPHeaderContentType().String()))
	if err != nil {
		return storageapi.Object{}, objectstore.Integrity{}, err
	}
	boundary := parameters["boundary"]
	if boundary == "" {
		return storageapi.Object{}, objectstore.Integrity{}, core.ErrObjectStoreContract
	}
	reader := multipart.NewReader(incoming.Body, boundary)
	metadataPart, err := reader.NextPart()
	if err != nil {
		return storageapi.Object{}, objectstore.Integrity{}, err
	}
	var metadata storageapi.Object
	if err := json.UnmarshalRead(metadataPart, &metadata); err != nil {
		return storageapi.Object{}, objectstore.Integrity{}, err
	}
	payloadPart, err := reader.NextPart()
	if err != nil {
		return storageapi.Object{}, objectstore.Integrity{}, err
	}
	integrity, err := gcsProviderStreamIntegrity(payloadPart)
	if err != nil {
		return storageapi.Object{}, objectstore.Integrity{}, err
	}
	if _, err := reader.NextPart(); !errors.Is(err, io.EOF) {
		return storageapi.Object{}, objectstore.Integrity{}, core.ErrObjectStoreContract
	}
	return metadata, integrity, nil
}

func (p *gcsUploadProvider) verifyUpload(metadata storageapi.Object, payload objectstore.Integrity) error {
	wantCRC, err := p.wantIntegrity.CRC32C.Base64()
	if err != nil {
		return err
	}
	if metadata.Name != gcsProviderObjectText || metadata.Crc32c != wantCRC ||
		payload.Length != p.wantIntegrity.Length || payload.SHA256 != p.wantIntegrity.SHA256 ||
		payload.CRC32C != p.wantIntegrity.CRC32C {
		return core.ErrObjectStoreIntegrity
	}
	return nil
}

type gcsReadDisposition uint8

const (
	gcsReadAvailable gcsReadDisposition = iota + 1
	gcsReadMediaAbsent
	gcsReadMetadataAbsent
)

type gcsReadCase struct {
	name          string
	payload       []byte
	metadataBytes []byte
	wantBytes     []byte
	maximum       uint64
	wantErr       core.ErrorIdentity
	disposition   gcsReadDisposition
}

type gcsReadProvider struct {
	t             testing.TB
	payload       []byte
	metadataBytes []byte
	disposition   gcsReadDisposition
}

func TestAuthenticatedGCSReadsExecuteTheOfficialSDKAndProveEveryByte(t *testing.T) {
	t.Parallel()

	short := gcsProviderPayload[:len(gcsProviderPayload)-1]
	long := append(bytes.Clone(gcsProviderPayload), 0x7f)
	cases := []gcsReadCase{
		{name: "exact non-empty object", payload: gcsProviderPayload, metadataBytes: gcsProviderPayload, wantBytes: gcsProviderPayload, maximum: uint64(len(gcsProviderPayload)), disposition: gcsReadAvailable},
		{name: "exact empty object", payload: nil, metadataBytes: nil, wantBytes: nil, disposition: gcsReadAvailable},
		{name: "exact one-byte object", payload: []byte{0x7f}, metadataBytes: []byte{0x7f}, wantBytes: []byte{0x7f}, maximum: 1, disposition: gcsReadAvailable},
		{name: "metadata extent exceeds caller ceiling", payload: gcsProviderPayload, metadataBytes: gcsProviderPayload, wantBytes: gcsProviderPayload, maximum: uint64(len(gcsProviderPayload) - 1), disposition: gcsReadAvailable, wantErr: core.ErrObjectStoreSize},
		{name: "caller digest differs from provider bytes", payload: gcsProviderPayload, metadataBytes: gcsProviderPayload, wantBytes: append([]byte("x"), gcsProviderPayload[1:]...), maximum: uint64(len(gcsProviderPayload)), disposition: gcsReadAvailable, wantErr: core.ErrObjectStoreIntegrity},
		{name: "provider body is shorter than metadata", payload: short, metadataBytes: gcsProviderPayload, wantBytes: gcsProviderPayload, maximum: uint64(len(gcsProviderPayload)), disposition: gcsReadAvailable, wantErr: core.ErrObjectStoreIntegrity},
		{name: "provider body is longer than metadata", payload: long, metadataBytes: gcsProviderPayload, wantBytes: gcsProviderPayload, maximum: uint64(len(gcsProviderPayload)), disposition: gcsReadAvailable, wantErr: core.ErrObjectStoreIntegrity},
		{name: "missing media retains source and absence identities", wantBytes: gcsProviderPayload, maximum: uint64(len(gcsProviderPayload)), disposition: gcsReadMediaAbsent, wantErr: core.ErrObjectStoreAbsent},
		{name: "missing generation metadata retains absence identity", payload: gcsProviderPayload, metadataBytes: gcsProviderPayload, wantBytes: gcsProviderPayload, maximum: uint64(len(gcsProviderPayload)), disposition: gcsReadMetadataAbsent, wantErr: core.ErrObjectStoreAbsent},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			provider := &gcsReadProvider{t: t, payload: tc.payload, metadataBytes: tc.metadataBytes, disposition: tc.disposition}
			client := bucketTestClient(t, provider)
			destination, root := gcsReadStageDestination(t, tc.maximum)
			got, gotErr := ReadGCSObject(context.Background(), client, GCSReadRequest{
				Destination: destination, Bucket: parsedGCSBucket(t, gcsProviderBucketText),
				Name:      parsedGCSObjectName(t, gcsProviderObjectText),
				Integrity: gcsExpectedReadIntegrity(t, tc.wantBytes, tc.metadataBytes, tc.maximum),
			})
			if !errors.Is(tc.wantErr, core.ErrUnknown) {
				if !errors.Is(gotErr, tc.wantErr) || got != (GCSReadResult{}) {
					t.Fatalf("ReadGCSObject() = (%v, %v), want zero and errors.Is(..., %v)", got, gotErr, tc.wantErr)
				}
				var leaked bytes.Buffer
				_, leakErr := filestore.Read(t.Context(), filestore.ReadRequest{
					Destination:  &leaked,
					Location:     filestore.Location{Root: root, Path: destination.Temporary.Path},
					MaximumBytes: gcsProviderMaximum(t, tc.maximum+1),
				})
				if !errors.Is(leakErr, fs.ErrNotExist) || leaked.Len() != 0 {
					t.Fatalf("rejected GCS read local stage = (%d bytes, %v), want absent and zero", leaked.Len(), leakErr)
				}
				return
			}
			staged, stagedErr := got.Staged()
			var content bytes.Buffer
			_, readErr := filestore.Read(t.Context(), filestore.ReadRequest{
				Destination:  &content,
				Location:     filestore.Location{Root: root, Path: destination.Temporary.Path},
				MaximumBytes: gcsProviderMaximum(t, tc.maximum+1),
			})
			if gotErr != nil || got.Validate() != nil || stagedErr != nil || readErr != nil ||
				!bytes.Equal(content.Bytes(), tc.wantBytes) {
				t.Fatalf("ReadGCSObject() = (%v, %v, staged %v, read %v, %q), want validated result and exact %q", got, gotErr, stagedErr, readErr, content.Bytes(), tc.wantBytes)
			}
			if err := filestore.Discard(t.Context(), staged); err != nil {
				t.Fatalf("filestore.Discard(verified read) error = %v, want nil", err)
			}
		})
	}
}

func gcsReadStageDestination(t testing.TB, expected uint64) (filestore.StageDestinationRequest, *os.Root) {
	t.Helper()
	absolute, err := core.ParseAbsolutePath(t.TempDir())
	if err != nil {
		t.Fatalf("core.ParseAbsolutePath(stage root) error = %v, want nil", err)
	}
	root, err := filestore.OpenRoot(t.Context(), absolute)
	if err != nil {
		t.Fatalf("filestore.OpenRoot(stage root) error = %v, want nil", err)
	}
	t.Cleanup(func() { _ = root.Close() })
	path, err := core.ParseRelativePath("download.stage")
	if err != nil {
		t.Fatalf("core.ParseRelativePath(download.stage) error = %v, want nil", err)
	}
	length, err := core.NewByteLength(expected)
	if err != nil {
		t.Fatalf("core.NewByteLength(%d) error = %v, want nil", expected, err)
	}
	return filestore.StageDestinationRequest{
		Temporary: filestore.Location{Root: root, Path: path}, ExpectedBytes: length, Mode: 0o600,
	}, root
}

func gcsExpectedReadIntegrity(t testing.TB, digest, checksum []byte, expected uint64) objectstore.Integrity {
	t.Helper()
	length, err := core.NewByteLength(expected)
	if err != nil {
		t.Fatalf("core.NewByteLength(%d) error = %v, want nil", expected, err)
	}
	return objectstore.Integrity{
		SHA256: core.SHA256Of(digest), Length: length,
		CRC32C: core.NewCRC32C(crc32.Checksum(checksum, crc32.MakeTable(crc32.Castagnoli))),
	}
}

func (p *gcsReadProvider) ServeHTTP(writer http.ResponseWriter, incoming *http.Request) {
	if incoming.Method != exchange.MethodGet.String() {
		p.t.Errorf("read provider method = %q, want %q", incoming.Method, exchange.MethodGet.String())
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if incoming.URL.Query().Get("alt") == "json" {
		if p.disposition == gcsReadMetadataAbsent {
			writeGoogleAPIError(writer, http.StatusNotFound)
			return
		}
		writeGCSAttrsResponse(p.t, writer, p.metadataBytes)
		return
	}
	if p.disposition == gcsReadMediaAbsent {
		writeGoogleAPIError(writer, http.StatusNotFound)
		return
	}
	writeGCSMediaResponse(p.t, writer, p.payload)
}

func writeGCSMediaResponse(t testing.TB, writer http.ResponseWriter, payload []byte) {
	t.Helper()
	integrity := gcsProviderIntegrity(t, payload, payload)
	checksum, err := integrity.CRC32C.Base64()
	if err != nil {
		t.Errorf("CRC32C.Base64() error = %v, want nil", err)
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}
	writer.Header().Set("Content-Type", gcsProviderMediaTypeText)
	writer.Header().Set("Cache-Control", gcsProviderCacheControlText)
	writer.Header().Set("Last-Modified", time.Unix(0, gcsProviderInstantNanos).UTC().Format(http.TimeFormat))
	writer.Header().Set("X-Goog-Generation", "1786000000000001")
	writer.Header().Set("X-Goog-Metageneration", "1")
	writer.Header().Set("X-Goog-Hash", "crc32c="+checksum)
	writer.WriteHeader(http.StatusOK)
	if _, err := writer.Write(payload); err != nil {
		t.Errorf("provider media response error = %v, want nil", err)
	}
}

func writeGCSAttrsResponse(t testing.TB, writer http.ResponseWriter, payload []byte) {
	t.Helper()
	integrity := gcsProviderIntegrity(t, payload, payload)
	writeGCSObjectResponse(gcsObjectResponse{
		t: t, writer: writer, integrity: integrity,
		metadata: storageapi.Object{Name: gcsProviderObjectText, ContentType: gcsProviderMediaTypeText,
			CacheControl: gcsProviderCacheControlText},
	})
}

type gcsDeleteDisposition uint8

const (
	gcsDeleteConfirmed gcsDeleteDisposition = iota + 1
	gcsDeleteSoftRetention
	gcsDeleteBucketAbsent
	gcsDeleteObjectAbsent
	gcsDeleteGenerationZero
	gcsDeleteRejected
	gcsDeleteAlreadyAbsent
	gcsDeleteReappears
	gcsDeleteConfirmationFails
)

type gcsDeleteCase struct {
	name        string
	disposition gcsDeleteDisposition
	wantErr     core.ErrorIdentity
}

type gcsDeleteProvider struct {
	t           testing.TB
	disposition gcsDeleteDisposition
	deleted     atomic.Bool
}

func TestAuthenticatedGCSExactDeletionBindsGenerationAndProvesAbsence(t *testing.T) {
	t.Parallel()

	cases := []gcsDeleteCase{
		{name: "current generation is deleted and confirmed absent", disposition: gcsDeleteConfirmed},
		{name: "soft-delete retention refuses destructive ambiguity", disposition: gcsDeleteSoftRetention, wantErr: core.ErrObjectStoreConflict},
		{name: "missing bucket retains absence identity", disposition: gcsDeleteBucketAbsent, wantErr: core.ErrObjectStoreAbsent},
		{name: "missing object retains absence identity", disposition: gcsDeleteObjectAbsent, wantErr: core.ErrObjectStoreAbsent},
		{name: "zero provider generation refuses contract", disposition: gcsDeleteGenerationZero, wantErr: core.ErrObjectStoreContract},
		{name: "provider delete rejection retains destination identity", disposition: gcsDeleteRejected, wantErr: core.ErrObjectStoreDestination},
		{name: "already absent generation delete remains confirmed", disposition: gcsDeleteAlreadyAbsent},
		{name: "object reappearing after delete is a conflict", disposition: gcsDeleteReappears, wantErr: core.ErrObjectStoreConflict},
		{name: "confirmation failure retains destination identity", disposition: gcsDeleteConfirmationFails, wantErr: core.ErrObjectStoreDestination},
		{name: "canceled call never reaches provider", disposition: gcsDeleteConfirmed, wantErr: core.ErrObjectStoreContract},
	}

	for index, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			provider := &gcsDeleteProvider{t: t, disposition: tc.disposition}
			client := bucketTestClient(t, provider)
			ctx := context.Background()
			if index == len(cases)-1 {
				canceled, cancel := context.WithCancel(ctx)
				cancel()
				ctx = canceled
			}
			got, gotErr := DeleteGCSObject(ctx, client, GCSDeleteObjectRequest{
				Bucket: parsedGCSBucket(t, gcsProviderBucketText), Name: parsedGCSObjectName(t, gcsProviderObjectText),
			})
			if !errors.Is(tc.wantErr, core.ErrUnknown) {
				if !errors.Is(gotErr, tc.wantErr) || got != (GCSDeleteObjectResult{}) {
					t.Fatalf("DeleteGCSObject() = (%v, %v), want zero and errors.Is(..., %v)", got, gotErr, tc.wantErr)
				}
				return
			}
			if gotErr != nil || got.Validate() != nil || got.Name() != parsedGCSObjectName(t, gcsProviderObjectText) {
				t.Fatalf("DeleteGCSObject() = (%v, %v), want validated exact deletion and nil", got, gotErr)
			}
		})
	}
}

func (p *gcsDeleteProvider) ServeHTTP(writer http.ResponseWriter, incoming *http.Request) {
	if incoming.Method == exchange.MethodDelete.String() {
		p.delete(writer)
		return
	}
	if incoming.Method != exchange.MethodGet.String() {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	if strings.Contains(incoming.URL.Path, "/o/") {
		p.objectAttrs(writer)
		return
	}
	p.bucketAttrs(writer)
}

func (p *gcsDeleteProvider) bucketAttrs(writer http.ResponseWriter) {
	if p.disposition == gcsDeleteBucketAbsent {
		writeGoogleAPIError(writer, http.StatusNotFound)
		return
	}
	response := storageapi.Bucket{Name: gcsProviderBucketText}
	if p.disposition == gcsDeleteSoftRetention {
		response.SoftDeletePolicy = &storageapi.BucketSoftDeletePolicy{RetentionDurationSeconds: 604800}
	}
	writeProviderJSON(p.t, writer, response)
}

func (p *gcsDeleteProvider) objectAttrs(writer http.ResponseWriter) {
	if !p.deleted.Load() {
		if p.disposition == gcsDeleteObjectAbsent {
			writeGoogleAPIError(writer, http.StatusNotFound)
			return
		}
		generation := gcsProviderGeneration
		if p.disposition == gcsDeleteGenerationZero {
			generation = 0
		}
		writeProviderJSON(p.t, writer, storageapi.Object{Name: gcsProviderObjectText, Generation: generation})
		return
	}
	switch p.disposition {
	case gcsDeleteReappears:
		writeProviderJSON(p.t, writer, storageapi.Object{Name: gcsProviderObjectText, Generation: gcsProviderGeneration + 1})
	case gcsDeleteConfirmationFails:
		writeGoogleAPIError(writer, http.StatusBadRequest)
	default:
		writeGoogleAPIError(writer, http.StatusNotFound)
	}
}

func (p *gcsDeleteProvider) delete(writer http.ResponseWriter) {
	if p.disposition == gcsDeleteRejected {
		writeGoogleAPIError(writer, http.StatusBadRequest)
		return
	}
	p.deleted.Store(true)
	if p.disposition == gcsDeleteAlreadyAbsent {
		writeGoogleAPIError(writer, http.StatusNotFound)
		return
	}
	writer.WriteHeader(http.StatusNoContent)
}

func TestGCSClientConstructorAndCloseOwnTheSDKLifecycle(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{
		Hazard: core.TestIsolationHazardProcessEnvironment,
		Scope:  core.TestIsolationScopePackageProcess,
	})

	server := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(server.Close)
	t.Setenv("STORAGE_EMULATOR_HOST", server.URL)

	client, gotErr := NewGCSClient(context.Background(), GCSClientConfig{
		Authentication: GCSAuthenticationApplicationDefault,
	})
	if gotErr != nil || client.Validate() != nil {
		t.Fatalf("NewGCSClient(emulator) = (%v, %v), want validated client and nil", client, gotErr)
	}
	if gotCloseErr := client.Close(); gotCloseErr != nil || client.Validate() == nil {
		t.Fatalf("GCSClient.Close() = %v and Validate() = %v, want nil then refused closed client", gotCloseErr, client.Validate())
	}
	if gotSecondCloseErr := client.Close(); !errors.Is(gotSecondCloseErr, core.ErrObjectStoreContract) {
		t.Fatalf("second GCSClient.Close() error = %v, want errors.Is(..., %v)", gotSecondCloseErr, core.ErrObjectStoreContract)
	}

	canceled, cancel := context.WithCancel(context.Background())
	cancel()
	gotCanceled, gotCanceledErr := NewGCSClient(canceled, GCSClientConfig{
		Authentication: GCSAuthenticationApplicationDefault,
	})
	if gotCanceled != nil || !errors.Is(gotCanceledErr, core.ErrObjectStoreContract) || !errors.Is(gotCanceledErr, context.Canceled) {
		t.Fatalf("NewGCSClient(canceled) = (%v, %v), want nil plus contract and cancellation identities", gotCanceled, gotCanceledErr)
	}
}

func TestGCSClientDisablesOfficialSDKRetryForOneProviderExecutionPolicy(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{
		Hazard: core.TestIsolationHazardProcessEnvironment,
		Scope:  core.TestIsolationScopePackageProcess,
	})

	var gotCalls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		gotCalls.Add(1)
		writeGoogleAPIError(writer, http.StatusServiceUnavailable)
	}))
	t.Cleanup(server.Close)
	t.Setenv("STORAGE_EMULATOR_HOST", server.URL)

	client, gotClientErr := NewGCSClient(context.Background(), GCSClientConfig{
		Authentication: GCSAuthenticationApplicationDefault,
	})
	if gotClientErr != nil || client.Validate() != nil {
		t.Fatalf("NewGCSClient(emulator) = (%v, %v), want validated client and nil", client, gotClientErr)
	}
	t.Cleanup(func() {
		if gotCloseErr := client.Close(); gotCloseErr != nil {
			t.Errorf("GCSClient.Close() error = %v, want nil", gotCloseErr)
		}
	})

	got, gotErr := CreateBucket(
		context.Background(), client, bucketCreateFixture(t, GCSNamespaceFlat),
	)
	providerCause, gotProviderCause := errors.AsType[*googleapi.Error](gotErr)
	if got != (GCSBucketProvisioning{}) || !errors.Is(gotErr, core.ErrObjectStoreDestination) ||
		!gotProviderCause || providerCause.Code != http.StatusServiceUnavailable {
		t.Fatalf("CreateBucket(provider unavailable) = (%v, %v, provider=%v/%t), want zero, destination identity, and provider 503",
			got, gotErr, providerCause, gotProviderCause)
	}
	if got := gotCalls.Load(); got != 1 {
		t.Fatalf("official SDK provider calls after retriable 503 = %d, want exactly 1", got)
	}
}

func gcsProviderIntegrity(t testing.TB, digestBytes, checksumBytes []byte) objectstore.Integrity {
	t.Helper()
	length, err := core.NewByteLength(uint64(len(digestBytes)))
	if err != nil {
		t.Fatalf("NewByteLength(%d) error = %v, want nil", len(digestBytes), err)
	}
	return objectstore.Integrity{
		SHA256: core.SHA256Of(digestBytes), Length: length,
		CRC32C: core.NewCRC32C(crc32.Checksum(checksumBytes, crc32.MakeTable(crc32.Castagnoli))),
	}
}

func gcsProviderStreamIntegrity(reader io.Reader) (objectstore.Integrity, error) {
	digest := core.NewDigestWriter()
	checksum := crc32.New(crc32.MakeTable(crc32.Castagnoli))
	if _, err := io.Copy(io.MultiWriter(digest, checksum), reader); err != nil {
		return objectstore.Integrity{}, err
	}
	sha256, length, err := digest.Seal()
	if err != nil {
		return objectstore.Integrity{}, err
	}
	return objectstore.Integrity{SHA256: sha256, Length: length, CRC32C: core.NewCRC32C(checksum.Sum32())}, nil
}

func gcsProviderMaximum(t testing.TB, value uint64) core.ByteCount {
	t.Helper()
	maximum, err := core.NewByteCount(value)
	if err != nil {
		t.Fatalf("NewByteCount(%d) error = %v, want nil", value, err)
	}
	return maximum
}

func parsedGCSObjectName(t testing.TB, value string) GCSObjectName {
	t.Helper()
	name, err := ParseGCSObjectName(value)
	if err != nil {
		t.Fatalf("ParseGCSObjectName(%q) error = %v, want nil", value, err)
	}
	return name
}

type gcsObjectResponse struct {
	t         testing.TB
	writer    http.ResponseWriter
	metadata  storageapi.Object
	integrity objectstore.Integrity
}

func writeGCSObjectResponse(response gcsObjectResponse) {
	response.t.Helper()
	checksum, err := response.integrity.CRC32C.Base64()
	if err != nil {
		response.t.Errorf("CRC32C.Base64() error = %v, want nil", err)
		response.writer.WriteHeader(http.StatusInternalServerError)
		return
	}
	instant := time.Unix(0, gcsProviderInstantNanos).UTC().Format(time.RFC3339Nano)
	metadata := response.metadata
	metadata.Bucket = gcsProviderBucketText
	metadata.Name = gcsProviderObjectText
	metadata.Generation = gcsProviderGeneration
	metadata.Size = response.integrity.Length.Uint64()
	metadata.Crc32c = checksum
	metadata.TimeCreated = instant
	metadata.Updated = instant
	metadata.CustomTime = instant
	if metadata.ContentType == "" {
		metadata.ContentType = gcsProviderMediaTypeText
	}
	writeProviderJSON(response.t, response.writer, metadata)
}

func writeProviderJSON(t testing.TB, writer http.ResponseWriter, value any) {
	t.Helper()
	writer.Header().Set("Content-Type", "application/json")
	if err := json.MarshalWrite(writer, value); err != nil {
		t.Errorf("provider JSON response error = %v, want nil", err)
	}
}

func writeGoogleAPIError(writer http.ResponseWriter, status int) {
	writer.WriteHeader(status)
}

var _ http.Handler = (*gcsUploadProvider)(nil)
var _ http.Handler = (*gcsReadProvider)(nil)
var _ http.Handler = (*gcsDeleteProvider)(nil)
