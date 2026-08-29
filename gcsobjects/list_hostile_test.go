package gcsobjects

import (
	"errors"
	"testing"

	"cloud.google.com/go/storage"
	"github.com/deliri/primitive/v2026/core"
	"google.golang.org/api/iterator"
)

type hostileGCSObjectIterator struct {
	objects []*storage.ObjectAttrs
	index   int
	err     error
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
		t.Fatal("visitor received malformed provider evidence")
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

func parsedGCSObjectPrefix(t testing.TB, value string) GCSObjectPrefix {
	t.Helper()
	prefix, err := ParseGCSObjectPrefix(value)
	if err != nil {
		t.Fatalf("ParseGCSObjectPrefix(%q) error = %v", value, err)
	}
	return prefix
}
