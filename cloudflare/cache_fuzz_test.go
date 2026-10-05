package cloudflare

import (
	"bytes"
	"encoding/json/v2"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzCloudflareZoneIdentityClosure(f *testing.F) {
	seed, err := ParseZoneID(strings.Repeat("a", core.CloudflareIdentityCharacters))
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seed.String())
	f.Add("")
	f.Add(strings.Repeat("a", core.CloudflareIdentityCharacters+1))
	grammar := regexp.MustCompile(`^[0-9a-f]+$`)
	f.Fuzz(func(t *testing.T, source string) {
		got, err := ParseZoneID(source)
		if len(source) == core.CloudflareIdentityCharacters && grammar.MatchString(source) {
			if err != nil || got.Validate() != nil || got.String() != source {
				t.Fatalf("zone=(%v,%v), want exact valid identity", got, err)
			}
			return
		}
		if !errors.Is(err, core.ErrCloudflareBinding) || got != (ZoneID{}) {
			t.Fatalf("zone=(%v,%v), want zero and binding refusal", got, err)
		}
	})
}

func FuzzCachePurgeResponseClosure(f *testing.F) {
	seed, err := core.MarshalCanonicalJSONDocument(apiEnvelope[cachePurgeWire]{Success: new(true), Result: cachePurgeWire{ID: CachePurgeOperationID("operation")}})
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seed)
	f.Add([]byte{})
	f.Add([]byte(`{"success":true,"success":false}`))
	f.Fuzz(func(t *testing.T, document []byte) {
		// The SDK owns the aggregate response bound. The oracle's input is capped
		// before the local provider fixture allocates or reflects arbitrary data.
		if len(document) > 65536 {
			return
		}
		server, request := cacheFixture(t, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set(core.HTTPHeaderContentType().String(), core.HTTPMediaTypeJSON().String())
			if _, err := w.Write(document); err != nil {
				t.Error(err)
			}
		})
		got, err := server.PurgeFile(t.Context(), request, testPolicy())
		if err != nil {
			if !errors.Is(err, core.ErrCloudflareResponse) || got != (CachePurgeReceipt{}) {
				t.Fatalf("refused=(%+v,%v), want zero and response refusal", got, err)
			}
			return
		}
		var observed apiEnvelope[cachePurgeWire]
		if err := json.Unmarshal(document, &observed, json.RejectUnknownMembers(true)); err != nil {
			t.Fatal(err)
		}
		id := string(observed.Result.ID)
		if observed.Success == nil || !*observed.Success || len(observed.Errors) != 0 || !utf8.ValidString(id) || utf8.RuneCountInString(id) > core.CloudflareCachePurgeIDMaximumCharacters || got.OperationID != observed.Result.ID || got.Zone != server.zone || got.URL != request.URL || got.Validate() != nil {
			t.Fatalf("receipt=%+v, want exact accepted provider facts", got)
		}
		canonical, err := core.MarshalCanonicalJSONDocument(observed)
		if err != nil {
			t.Fatal(err)
		}
		var again apiEnvelope[cachePurgeWire]
		if err := json.Unmarshal(canonical, &again); err != nil {
			t.Fatal(err)
		}
		second, err := core.MarshalCanonicalJSONDocument(again)
		if err != nil || !bytes.Equal(canonical, second) {
			t.Fatalf("second projection=(%q,%v), want stable %q", second, err, canonical)
		}
	})
}
