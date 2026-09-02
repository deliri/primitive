package gcsobjects

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/storage"
	"github.com/deliri/primitive/v2026/core"
	"google.golang.org/api/iterator"
	storageapi "google.golang.org/api/storage/v1"
)

type hostileGCSObjectIterator struct {
	err     error
	objects []*storage.ObjectAttrs
	index   int
}

func (i *hostileGCSObjectIterator) Next() (*storage.ObjectAttrs, error) {
	if i.index < len(i.objects) {
		object := i.objects[i.index]
		i.index++
		return object, nil
	}
	if i.err != nil {
		return nil, i.err
	}
	return nil, iterator.Done
}

func TestVisitGCSObjectsStreamsInProviderOrderAndEnforcesBound(t *testing.T) {
	t.Parallel()

	first := validMediaGCSAttrs(t)
	first.Name = "forge-recovery/v1/project/0001.json"
	second := *first
	second.Name = "forge-recovery/v1/project/0002.json"
	maximum, err := core.NewByteCount(2)
	if err != nil {
		t.Fatalf("core.NewByteCount() error = %v", err)
	}
	request := GCSListRequest{Bucket: parsedGCSBucket(t, first.Bucket), Prefix: parsedGCSObjectPrefix(t, "forge-recovery/v1/project/"), MaxObjects: maximum}
	if err := request.Validate(); err != nil {
		t.Fatalf("GCSListRequest.Validate() error = %v, want nil", err)
	}
	var names []GCSObjectName
	objects := &hostileGCSObjectIterator{objects: []*storage.ObjectAttrs{first, &second}}
	err = visitGCSObjects(objects, 2, func(metadata GCSObjectMetadata) error {
		names = append(names, metadata.Name())
		return nil
	})
	if err != nil {
		t.Fatalf("visitGCSObjects(exact bound) error = %v, want nil", err)
	}
	if len(names) != 2 || names[0].String() != first.Name || names[1].String() != second.Name {
		t.Fatalf("visitGCSObjects order = %q, want [%q %q]", names, first.Name, second.Name)
	}

	objects = &hostileGCSObjectIterator{objects: []*storage.ObjectAttrs{first, &second}}
	err = visitGCSObjects(objects, 1, func(GCSObjectMetadata) error { return nil })
	if !errors.Is(err, core.ErrObjectStoreSize) {
		t.Fatalf("visitGCSObjects(over bound) error = %v, want %v", err, core.ErrObjectStoreSize)
	}
}

func TestVisitGCSObjectsRefusesMalformedProviderEvidenceAndVisitorFailure(t *testing.T) {
	t.Parallel()

	valid := validMediaGCSAttrs(t)
	malformed := *valid
	malformed.Generation = 0
	err := visitGCSObjects(&hostileGCSObjectIterator{objects: []*storage.ObjectAttrs{&malformed}}, 1, func(GCSObjectMetadata) error {
		t.Fatalf("visitor calls for generation = %d, want 0 calls for malformed provider evidence", malformed.Generation)
		return nil
	})
	if !errors.Is(err, core.ErrObjectStoreContract) {
		t.Fatalf("visitGCSObjects(malformed metadata) error = %v, want %v", err, core.ErrObjectStoreContract)
	}

	want := errors.New("visitor refused object")
	err = visitGCSObjects(&hostileGCSObjectIterator{objects: []*storage.ObjectAttrs{valid}}, 1, func(GCSObjectMetadata) error { return want })
	if !errors.Is(err, want) {
		t.Fatalf("visitGCSObjects(visitor failure) error = %v, want %v", err, want)
	}
}

func TestGCSListToExactReadProductionHandoffLayerTriad(t *testing.T) {
	t.Parallel()

	provider := &gcsListReadProvider{t: t, payload: bytes.Clone(gcsProviderPayload)}
	client := bucketTestClient(t, provider)
	maximum, err := core.NewByteCount(2)
	if err != nil {
		t.Fatalf("core.NewByteCount() error = %v, want nil", err)
	}
	request := GCSListRequest{
		Bucket: parsedGCSBucket(t, gcsProviderBucketText), Prefix: parsedGCSObjectPrefix(t, "users/01/evidence/"), MaxObjects: maximum,
	}
	var listed []GCSObjectMetadata
	if gotErr := ListGCSObjects(t.Context(), client, request, func(object GCSObjectMetadata) error {
		listed = append(listed, object)
		return nil
	}); gotErr != nil {
		t.Fatalf("ListGCSObjects() error = %v, want nil", gotErr)
	}
	if len(listed) != 1 || listed[0].Name() != parsedGCSObjectName(t, gcsProviderObjectText) {
		t.Fatalf("ListGCSObjects() = %v, want one exact provider object", listed)
	}
	readMaximum, err := core.NewByteCount(uint64(len(gcsProviderPayload)))
	if err != nil {
		t.Fatalf("core.NewByteCount(read maximum) error = %v, want nil", err)
	}
	var destination bytes.Buffer
	readRequest := GCSListedReadRequest{Destination: &destination, Object: listed[0], Maximum: readMaximum}
	if gotErr := readRequest.Validate(); gotErr != nil {
		t.Fatalf("GCSListedReadRequest.Validate() error = %v, want nil", gotErr)
	}
	read, gotErr := ReadListedGCSObject(t.Context(), client, readRequest)
	if gotErr != nil || read != listed[0] || !bytes.Equal(destination.Bytes(), gcsProviderPayload) {
		t.Fatalf("ReadListedGCSObject() = (%v, %q, %v), want listed metadata, exact bytes, nil", read, destination.Bytes(), gotErr)
	}
	if provider.listCalls != 1 || provider.metadataCalls != 1 || provider.mediaCalls != 1 {
		t.Fatalf("provider list/metadata/media calls = (%d, %d, %d), want (1, 1, 1)", provider.listCalls, provider.metadataCalls, provider.mediaCalls)
	}

	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	before := provider.listCalls
	gotErr = ListGCSObjects(cancelled, client, request, func(GCSObjectMetadata) error { return nil })
	if !errors.Is(gotErr, context.Canceled) || provider.listCalls != before {
		t.Fatalf("ListGCSObjects(cancelled) = (%v, calls:%d), want context.Canceled and no provider call", gotErr, provider.listCalls-before)
	}
	if gotErr := (GCSListedReadRequest{}).Validate(); !errors.Is(gotErr, core.ErrObjectStoreDestination) {
		t.Fatalf("GCSListedReadRequest{}.Validate() error = %v, want errors.Is(..., %v)", gotErr, core.ErrObjectStoreDestination)
	}
}

type gcsListReadProvider struct {
	t             testing.TB
	payload       []byte
	listCalls     uint64
	metadataCalls uint64
	mediaCalls    uint64
}

func (p *gcsListReadProvider) ServeHTTP(writer http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		writer.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	object := p.object()
	if strings.HasSuffix(request.URL.Path, "/o") {
		p.listCalls++
		writeProviderJSON(p.t, writer, storageapi.Objects{Items: []*storageapi.Object{&object}})
		return
	}
	if request.URL.Query().Get("alt") == "json" {
		p.metadataCalls++
		writeProviderJSON(p.t, writer, object)
		return
	}
	p.mediaCalls++
	writeGCSMediaResponse(p.t, writer, p.payload)
}

func (p *gcsListReadProvider) object() storageapi.Object {
	integrity := gcsProviderIntegrity(p.t, p.payload, p.payload)
	checksum, err := integrity.CRC32C.Base64()
	if err != nil {
		p.t.Fatalf("CRC32C.Base64() error = %v, want nil", err)
	}
	instant := time.Unix(0, gcsProviderInstantNanos).UTC().Format(time.RFC3339Nano)
	return storageapi.Object{
		Bucket: gcsProviderBucketText, Name: gcsProviderObjectText, Generation: gcsProviderGeneration, Size: integrity.Length.Uint64(), Crc32c: checksum,
		ContentType: gcsProviderMediaTypeText, CacheControl: gcsProviderCacheControlText, TimeCreated: instant, Updated: instant, CustomTime: instant,
	}
}

func parsedGCSObjectPrefix(t testing.TB, value string) GCSObjectPrefix {
	t.Helper()
	prefix, err := ParseGCSObjectPrefix(value)
	if err != nil {
		t.Fatalf("ParseGCSObjectPrefix(%q) error = %v", value, err)
	}
	return prefix
}
