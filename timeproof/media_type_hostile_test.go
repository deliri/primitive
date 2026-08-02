package timeproof_test

import (
	"errors"
	"math"
	"mime"
	"slices"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/timeproof"
)

func TestRFC3161MediaTypeExhaustsCompleteByteDomain(t *testing.T) {
	t.Parallel()

	valid := []timeproof.MediaType{
		timeproof.MediaTypeRequest,
		timeproof.MediaTypeResponse,
	}
	seen := make([]string, 0, len(valid))
	for value := range math.MaxUint8 + 1 {
		media := timeproof.MediaType(value)
		wantValid := slices.Contains(valid, media)
		name, admitted := requireMediaTypeDomainValue(t, media, wantValid)
		if admitted {
			if slices.Contains(seen, name) {
				t.Fatalf("MediaType(%d) duplicates canonical representation %q", value, name)
			}
			seen = append(seen, name)
		}
	}
	if len(seen) != len(valid) {
		t.Fatalf("distinct valid RFC 3161 media types = %d, want %d", len(seen), len(valid))
	}
}

func requireMediaTypeDomainValue(t *testing.T, media timeproof.MediaType, wantValid bool) (string, bool) {
	t.Helper()

	if got := media.IsValid(); got != wantValid {
		t.Fatalf("MediaType(%d).IsValid() = %t, want %t", media, got, wantValid)
	}
	projected, gotErr := media.HTTPMediaType()
	if !wantValid {
		if !errors.Is(gotErr, core.ErrTimeProofContract) || projected != (core.HTTPMediaType{}) || media.String() != "" {
			t.Fatalf("invalid MediaType(%d) projection = (%q, %v), want zero and %v", media, projected.String(), gotErr, core.ErrTimeProofContract)
		}
		return "", false
	}
	if gotErr != nil || projected.String() != media.String() {
		t.Fatalf("MediaType(%d).HTTPMediaType() = (%q, %v), want (%q, nil)", media, projected.String(), gotErr, media.String())
	}
	var offWire core.OffWireEnum = media
	offWire.OffWireEnum()
	return projected.String(), true
}

func TestRFC3161MediaTypesReachRealMIMEHandoff(t *testing.T) {
	t.Parallel()

	cases := []timeproof.MediaType{
		timeproof.MediaTypeRequest,
		timeproof.MediaTypeResponse,
	}
	for _, media := range cases {
		media := media
		t.Run(media.String(), func(t *testing.T) {
			t.Parallel()

			projected, gotErr := media.HTTPMediaType()
			if gotErr != nil {
				t.Fatalf("MediaType.HTTPMediaType() error = %v, want nil", gotErr)
			}
			base, parameters, gotParseErr := mime.ParseMediaType(projected.String())
			if gotParseErr != nil || base != media.String() || len(parameters) != 0 {
				t.Fatalf("mime.ParseMediaType(%q) = (%q, %v, %v), want (%q, empty, nil)", projected.String(), base, parameters, gotParseErr, media.String())
			}
		})
	}
}
