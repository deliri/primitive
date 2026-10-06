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
		prefix, prefixErr := NewCachePurgePrefix(request.URL)
		if prefixErr != nil {
			t.Fatal(prefixErr)
		}
		prefixReceipt, prefixErr := server.PurgePrefix(t.Context(), CachePrefixPurgeRequest{Prefix: prefix}, testPolicy())
		if err != nil {
			if !errors.Is(err, core.ErrCloudflareResponse) || got != (CachePurgeReceipt{}) || !errors.Is(prefixErr, core.ErrCloudflareResponse) || prefixReceipt != (CachePrefixPurgeReceipt{}) {
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
		if prefixErr != nil || prefixReceipt.Validate() != nil || prefixReceipt.Prefix != prefix || prefixReceipt.Zone != server.zone || prefixReceipt.OperationID != observed.Result.ID {
			t.Fatalf("prefix receipt = %+v/%v, want exact accepted provider facts", prefixReceipt, prefixErr)
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

func FuzzCachePurgeOperationIDClosure(f *testing.F) {
	seed, err := ParseCachePurgeOperationID(strings.Repeat("a", core.CloudflareCachePurgeIDMaximumCharacters))
	if err != nil {
		f.Fatal(err)
	}
	f.Add(string(seed))
	f.Add("")
	f.Add(string([]byte{0xff}))
	f.Add(strings.Repeat("é", core.CloudflareCachePurgeIDMaximumCharacters+1))
	f.Fuzz(func(t *testing.T, source string) {
		got, err := ParseCachePurgeOperationID(source)
		wantValid := utf8.ValidString(source) && utf8.RuneCountInString(source) <= core.CloudflareCachePurgeIDMaximumCharacters
		if !wantValid {
			if !errors.Is(err, core.ErrCloudflareResponse) || got != "" {
				t.Fatalf("operation=(%q,%v), want zero/response refusal", got, err)
			}
			return
		}
		if err != nil || got.Validate() != nil || string(got) != source {
			t.Fatalf("operation=(%q,%v), want exact %q", got, err, source)
		}
		wire, err := core.MarshalCanonicalJSONDocument(got)
		if err != nil {
			t.Fatal(err)
		}
		var decoded CachePurgeOperationID
		if err := json.Unmarshal(wire, &decoded); err != nil || decoded != got {
			t.Fatalf("operation round trip=(%q,%v), want %q", decoded, err, got)
		}
		second, err := core.MarshalCanonicalJSONDocument(decoded)
		if err != nil || !bytes.Equal(wire, second) {
			t.Fatalf("second projection=(%q,%v), want %q", second, err, wire)
		}
	})
}
