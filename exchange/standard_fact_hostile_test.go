package exchange_test

import (
	"errors"
	"math"
	"mime"
	"net/http"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

func TestStandardHeaderExhaustsCompleteByteDomain(t *testing.T) {
	t.Parallel()

	valid := []exchange.StandardHeader{
		exchange.StandardHeaderAuthorization,
		exchange.StandardHeaderCacheControl,
		exchange.StandardHeaderForwardedFor,
		exchange.StandardHeaderRetryAfter,
	}
	seen := make([]string, 0, len(valid))
	for value := range math.MaxUint8 + 1 {
		header := exchange.StandardHeader(value)
		wantValid := slices.Contains(valid, header)
		name, admitted := requireStandardHeaderDomainValue(t, header, wantValid)
		if admitted {
			requireDistinctStandardFact(t, value, name, seen)
			seen = append(seen, name)
		}
	}
	if len(seen) != len(valid) {
		t.Fatalf("distinct valid standard headers = %d, want %d", len(seen), len(valid))
	}
}

func requireStandardHeaderDomainValue(t *testing.T, header exchange.StandardHeader, wantValid bool) (string, bool) {
	t.Helper()

	if got := header.IsValid(); got != wantValid {
		t.Fatalf("StandardHeader(%d).IsValid() = %t, want %t", header, got, wantValid)
	}
	name, gotErr := header.Name()
	if !wantValid {
		if !errors.Is(gotErr, core.ErrExchangeContract) || name != (core.HTTPHeaderName{}) || header.String() != "" {
			t.Fatalf("invalid StandardHeader(%d) projection = (%q, %v), want zero and %v", header, name.String(), gotErr, core.ErrExchangeContract)
		}
		return "", false
	}
	if gotErr != nil || name.String() != header.String() {
		t.Fatalf("StandardHeader(%d).Name() = (%q, %v), want (%q, nil)", header, name.String(), gotErr, header.String())
	}
	var offWire core.OffWireEnum = header
	offWire.OffWireEnum()
	return name.String(), true
}

func TestStandardMediaTypeExhaustsCompleteByteDomain(t *testing.T) {
	t.Parallel()

	valid := []exchange.StandardMediaType{
		exchange.StandardMediaTypeJSON,
		exchange.StandardMediaTypePlainText,
	}
	seen := make([]string, 0, len(valid))
	for value := range math.MaxUint8 + 1 {
		media := exchange.StandardMediaType(value)
		wantValid := slices.Contains(valid, media)
		name, admitted := requireStandardMediaDomainValue(t, media, wantValid)
		if admitted {
			requireDistinctStandardFact(t, value, name, seen)
			seen = append(seen, name)
		}
	}
	if len(seen) != len(valid) {
		t.Fatalf("distinct valid standard media types = %d, want %d", len(seen), len(valid))
	}
}

func requireStandardMediaDomainValue(t *testing.T, media exchange.StandardMediaType, wantValid bool) (string, bool) {
	t.Helper()

	if got := media.IsValid(); got != wantValid {
		t.Fatalf("StandardMediaType(%d).IsValid() = %t, want %t", media, got, wantValid)
	}
	projected, gotErr := media.HTTPMediaType()
	if !wantValid {
		if !errors.Is(gotErr, core.ErrExchangeContract) || projected != (core.HTTPMediaType{}) || media.String() != "" {
			t.Fatalf("invalid StandardMediaType(%d) projection = (%q, %v), want zero and %v", media, projected.String(), gotErr, core.ErrExchangeContract)
		}
		return "", false
	}
	if gotErr != nil || projected.String() != media.String() {
		t.Fatalf("StandardMediaType(%d).HTTPMediaType() = (%q, %v), want (%q, nil)", media, projected.String(), gotErr, media.String())
	}
	var offWire core.OffWireEnum = media
	offWire.OffWireEnum()
	return projected.String(), true
}

func requireDistinctStandardFact(t *testing.T, value int, name string, seen []string) {
	t.Helper()

	if slices.Contains(seen, name) {
		t.Fatalf("standard fact %d duplicates canonical projection %q", value, name)
	}
}

func TestStandardHTTPFactsReachRealStandardLibraryHandoffs(t *testing.T) {
	t.Parallel()

	headerCases := []exchange.StandardHeader{
		exchange.StandardHeaderAuthorization,
		exchange.StandardHeaderCacheControl,
		exchange.StandardHeaderForwardedFor,
		exchange.StandardHeaderRetryAfter,
	}
	for _, header := range headerCases {
		t.Run(header.String(), func(t *testing.T) {
			t.Parallel()

			name, gotErr := header.Name()
			if gotErr != nil {
				t.Fatalf("StandardHeader.Name() error = %v, want nil", gotErr)
			}
			fields := make(http.Header)
			fields.Set(name.String(), "exact-value")
			if got := fields.Get(header.String()); got != "exact-value" {
				t.Fatalf("http.Header.Get(%q) = %q, want %q", header.String(), got, "exact-value")
			}
		})
	}

	mediaCases := []exchange.StandardMediaType{
		exchange.StandardMediaTypeJSON,
		exchange.StandardMediaTypePlainText,
	}
	for _, media := range mediaCases {
		t.Run(media.String(), func(t *testing.T) {
			t.Parallel()

			projected, gotErr := media.HTTPMediaType()
			if gotErr != nil {
				t.Fatalf("StandardMediaType.HTTPMediaType() error = %v, want nil", gotErr)
			}
			base, parameters, gotParseErr := mime.ParseMediaType(projected.String())
			if gotParseErr != nil || base != media.String() || len(parameters) != 0 {
				t.Fatalf("mime.ParseMediaType(%q) = (%q, %v, %v), want (%q, empty, nil)", projected.String(), base, parameters, gotParseErr, media.String())
			}
		})
	}
}
