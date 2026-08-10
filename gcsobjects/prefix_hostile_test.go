package gcsobjects

import (
	"errors"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestGCSObjectSegmentHostileBoundaryTable(t *testing.T) {
	t.Parallel()

	maximum := strings.Repeat("a", GCSObjectNameMaximumBytes-1)
	cases := []struct {
		wantErr error
		name    string
		input   string
	}{
		{name: "minimum one byte", input: "a"},
		{name: "two bytes", input: "ab"},
		{name: "ordinary file leaf", input: "evidence.json"},
		{name: "uuid leaf", input: "00000000-0007-7000-8000-000000000007"},
		{name: "spaces remain provider-valid leaf data", input: "evidence copy"},
		{name: "leading dot ordinary leaf", input: ".manifest"},
		{name: "trailing dot ordinary leaf", input: "manifest."},
		{name: "multibyte leaf", input: "évidence"},
		{name: "one below maximum", input: maximum[:len(maximum)-1]},
		{name: "exact maximum segment", input: maximum},
		{name: "empty", wantErr: core.ErrObjectStoreContract},
		{name: "dot navigation", input: ".", wantErr: core.ErrObjectStoreContract},
		{name: "double-dot navigation", input: "..", wantErr: core.ErrObjectStoreContract},
		{name: "one above maximum", input: strings.Repeat("a", GCSObjectNameMaximumBytes), wantErr: core.ErrObjectStoreContract},
		{name: "slash", input: "parent/leaf", wantErr: core.ErrObjectStoreContract},
		{name: "leading slash", input: "/leaf", wantErr: core.ErrObjectStoreContract},
		{name: "trailing slash", input: "leaf/", wantErr: core.ErrObjectStoreContract},
		{name: "newline", input: "leaf\nname", wantErr: core.ErrObjectStoreContract},
		{name: "carriage return", input: "leaf\rname", wantErr: core.ErrObjectStoreContract},
		{name: "invalid utf8", input: string([]byte{0xff}), wantErr: core.ErrObjectStoreContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, gotErr := ParseGCSObjectSegment(tc.input)
			if tc.wantErr != nil {
				if !errors.Is(gotErr, tc.wantErr) || got != (GCSObjectSegment{}) {
					t.Fatalf("ParseGCSObjectSegment(%q) = (%q, %v), want zero and errors.Is %v",
						tc.input, got.String(), gotErr, tc.wantErr)
				}
				return
			}
			if gotErr != nil || got.String() != tc.input || got.Validate() != nil {
				t.Fatalf("ParseGCSObjectSegment(%q) = (%q, %v), want exact input and nil",
					tc.input, got.String(), gotErr)
			}
		})
	}
}

func TestGCSLogicalDirectoryCompositionLayerTriad(t *testing.T) {
	t.Parallel()

	segments := []string{
		"accounts", "witness", "bug", "peachfuzz", "chits", "uploads",
		"00000000-0007-7000-8000-000000000007", "evidence", "versions", "results.json",
	}
	rootSegment := parsedGCSSegment(t, segments[0])
	root, err := ComposeGCSRootPrefix(GCSRootPrefixRequest{Segment: rootSegment})
	if err != nil || root.String() != segments[0]+"/" {
		t.Fatalf("ComposeGCSRootPrefix() = (%q, %v), want exact root and nil", root.String(), err)
	}
	prefix := root
	for _, value := range segments[1 : len(segments)-1] {
		segment := parsedGCSSegment(t, value)
		prefix, err = ComposeGCSChildPrefix(GCSChildPrefixRequest{Parent: prefix, Segment: segment})
		if err != nil {
			t.Fatalf("ComposeGCSChildPrefix(%q) error = %v, want nil", value, err)
		}
	}
	leaf := parsedGCSSegment(t, segments[len(segments)-1])
	name, err := ComposeGCSObjectName(GCSObjectInPrefixRequest{Prefix: prefix, Leaf: leaf})
	if err != nil || name.String() != prefix.String()+leaf.String() {
		t.Fatalf("ComposeGCSObjectName() = (%q, %v), want exact typed prefix and leaf composition",
			name.String(), err)
	}

	invalid := []struct {
		run  func() error
		name string
	}{
		{name: "zero root request", run: func() error { _, err := ComposeGCSRootPrefix(GCSRootPrefixRequest{}); return err }},
		{name: "zero child request", run: func() error { _, err := ComposeGCSChildPrefix(GCSChildPrefixRequest{}); return err }},
		{name: "child missing parent", run: func() error { _, err := ComposeGCSChildPrefix(GCSChildPrefixRequest{Segment: rootSegment}); return err }},
		{name: "child missing segment", run: func() error { _, err := ComposeGCSChildPrefix(GCSChildPrefixRequest{Parent: root}); return err }},
		{name: "zero object request", run: func() error { _, err := ComposeGCSObjectName(GCSObjectInPrefixRequest{}); return err }},
		{name: "object missing prefix", run: func() error { _, err := ComposeGCSObjectName(GCSObjectInPrefixRequest{Leaf: leaf}); return err }},
		{name: "object missing leaf", run: func() error { _, err := ComposeGCSObjectName(GCSObjectInPrefixRequest{Prefix: root}); return err }},
		{name: "child exceeds object-name ceiling", run: func() error {
			wide := parsedGCSSegment(t, strings.Repeat("a", GCSObjectNameMaximumBytes-1))
			_, err := ComposeGCSChildPrefix(GCSChildPrefixRequest{Parent: root, Segment: wide})
			return err
		}},
		{name: "object exceeds object-name ceiling", run: func() error {
			wide := parsedGCSSegment(t, strings.Repeat("a", GCSObjectNameMaximumBytes-1))
			_, err := ComposeGCSObjectName(GCSObjectInPrefixRequest{Prefix: root, Leaf: wide})
			return err
		}},
		{name: "reserved acme root remains refused", run: func() error {
			wellKnown := parsedGCSSegment(t, ".well-known")
			acme := parsedGCSSegment(t, "acme-challenge")
			wellKnownPrefix, err := ComposeGCSRootPrefix(GCSRootPrefixRequest{Segment: wellKnown})
			if err != nil {
				return err
			}
			_, err = ComposeGCSChildPrefix(GCSChildPrefixRequest{Parent: wellKnownPrefix, Segment: acme})
			return err
		}},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if gotErr := tc.run(); !errors.Is(gotErr, core.ErrObjectStoreContract) {
				t.Fatalf("logical directory composition %s error = %v, want errors.Is %v",
					tc.name, gotErr, core.ErrObjectStoreContract)
			}
		})
	}

	zeroRoot, rootErr := ComposeGCSRootPrefix(GCSRootPrefixRequest{})
	zeroName, nameErr := ComposeGCSObjectName(GCSObjectInPrefixRequest{})
	if !errors.Is(rootErr, core.ErrObjectStoreContract) || zeroRoot != (GCSObjectPrefix{}) ||
		!errors.Is(nameErr, core.ErrObjectStoreContract) || zeroName != (GCSObjectName{}) {
		t.Fatalf("zero logical composition = ((%v, %v), (%v, %v)), want zero typed refusals",
			zeroRoot, rootErr, zeroName, nameErr)
	}
}

func parsedGCSSegment(t testing.TB, value string) GCSObjectSegment {
	t.Helper()

	segment, err := ParseGCSObjectSegment(value)
	if err != nil {
		t.Fatalf("ParseGCSObjectSegment(%q) error = %v, want nil", value, err)
	}
	return segment
}
