package gcsobjects

import (
	"bytes"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	storageapi "google.golang.org/api/storage/v1"
)

func FuzzGCSListRangeNativeClosure(f *testing.F) {
	low := parsedGCSObjectName(f, "brand/_usage_2026_10_28_20")
	high := parsedGCSObjectName(f, "brand/_usage_2026_10_28_21")
	name := parsedGCSObjectName(f, "brand/_usage_2026_10_28_20_30_00_fixture_v0")
	for _, bounds := range []GCSListRange{{}, {Start: low}, {End: high}, {Start: low, End: high}, {Start: low, End: low}} {
		request := GCSListRequest{Bucket: parsedGCSBucket(f, gcsProviderBucketText), Prefix: parsedGCSObjectPrefix(f, "brand/"), Range: bounds}
		if err := request.Validate(); err != nil {
			f.Fatal(err)
		}
		f.Add(bounds.Start.String(), bounds.End.String(), name.String())
	}
	// Hostile source values, including a valid-nominal but reversed interval.
	f.Add(high.String(), low.String(), name.String())
	f.Add("\n", high.String(), name.String())
	f.Add(low.String(), "\xff", name.String())
	f.Add(low.String(), high.String(), "\n")
	f.Fuzz(func(t *testing.T, startText, endText, nameText string) {
		bounds := GCSListRange{}
		if startText != "" {
			start, err := ParseGCSObjectName(startText)
			if err != nil {
				if !errors.Is(err, core.ErrObjectStoreContract) || start != (GCSObjectName{}) {
					t.Fatalf("lower nominal=%v/%v,want zero/contract refusal", start, err)
				}
				return
			}
			bounds.Start = start
		}
		if endText != "" {
			end, err := ParseGCSObjectName(endText)
			if err != nil {
				if !errors.Is(err, core.ErrObjectStoreContract) || end != (GCSObjectName{}) {
					t.Fatalf("upper nominal=%v/%v,want zero/contract refusal", end, err)
				}
				return
			}
			bounds.End = end
		}
		objectName, err := ParseGCSObjectName(nameText)
		if err != nil {
			if !errors.Is(err, core.ErrObjectStoreContract) || objectName != (GCSObjectName{}) {
				t.Fatalf("provider nominal=%v/%v,want zero/contract refusal", objectName, err)
			}
			return
		}
		// Independent byte ordering defines native lexical bounds. Neither Validate
		// nor contains supplies the expected admission or visit count.
		reversed := startText != "" && endText != "" && bytes.Compare([]byte(startText), []byte(endText)) > 0
		wantVisit := strings.HasPrefix(nameText, "brand/") && (startText == "" || bytes.Compare([]byte(nameText), []byte(startText)) >= 0) && (endText == "" || bytes.Compare([]byte(nameText), []byte(endText)) < 0)
		calls := &atomic.Int32{}
		client := bucketTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			calls.Add(1)
			query := r.URL.Query()
			if query.Get("prefix") != "brand/" || query.Get("startOffset") != startText || query.Get("endOffset") != endText {
				t.Errorf("native query=%v,want exact admitted prefix/bounds", query)
			}
			result := storageapi.Objects{Items: []*storageapi.Object{}}
			if strings.HasPrefix(nameText, query.Get("prefix")) && (query.Get("startOffset") == "" || nameText >= query.Get("startOffset")) && (query.Get("endOffset") == "" || nameText < query.Get("endOffset")) {
				object := (&gcsListReadProvider{t: t, payload: []byte("x")}).object()
				object.Name = objectName.String()
				result.Items = append(result.Items, &object)
			}
			writeProviderJSON(t, w, result)
		}))
		request := GCSListRequest{Bucket: parsedGCSBucket(t, gcsProviderBucketText), Prefix: parsedGCSObjectPrefix(t, "brand/"), Range: bounds}
		visits := 0
		gotErr := ListGCSObjects(t.Context(), client, request, func(metadata GCSObjectMetadata) error {
			visits++
			if metadata.Name() != objectName {
				t.Fatalf("visited name=%v,want exact admitted provider name %v", metadata.Name(), objectName)
			}
			return nil
		})
		if reversed {
			if !errors.Is(gotErr, core.ErrObjectStoreContract) || calls.Load() != 0 || visits != 0 {
				t.Fatalf("reversed listing=(%v,%d calls,%d visits),want contract refusal/no effects", gotErr, calls.Load(), visits)
			}
			return
		}
		wantCount := 0
		if wantVisit {
			wantCount = 1
		}
		if gotErr != nil || calls.Load() != 1 || visits != wantCount {
			t.Fatalf("native listing=(%v,%d calls,%d visits),want nil/1/%d", gotErr, calls.Load(), visits, wantCount)
		}
	})
}
