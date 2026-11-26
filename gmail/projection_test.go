package gmail

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestGmailThreadPageIsBoundToItsRequest(t *testing.T) {
	t.Parallel()
	ids := func(amount int) []threadReferenceWire {
		out := make([]threadReferenceWire, 0, amount)
		for i := range amount {
			out = append(out, threadReferenceWire{ID: ThreadID(fmt.Sprintf("thread_%02d", i))})
		}
		return out
	}
	wide := strings.Repeat("x", idMaximumBytes)
	cursor := strings.Repeat("p", cursorMaximumBytes)
	cases := []struct {
		name   string
		wire   threadPageWire
		limit  uint8
		before PageToken
		want   error
	}{
		{name: "empty page has no continuation", limit: 1},
		{name: "exhausted cursor has clean zero rows", limit: 2, before: "old"},
		{name: "one row", wire: threadPageWire{Threads: ids(1)}, limit: 1},
		{name: "under requested limit", wire: threadPageWire{Threads: ids(2)}, limit: 3},
		{name: "exact requested count", wire: threadPageWire{Threads: ids(2), Next: "next"}, limit: 2},
		{name: "near package limit", wire: threadPageWire{Threads: ids(ThreadPageMaximum - 1)}, limit: ThreadPageMaximum},
		{name: "exact package max count", wire: threadPageWire{Threads: ids(ThreadPageMaximum)}, limit: ThreadPageMaximum},
		{name: "pagination recipient is a new cursor", wire: threadPageWire{Threads: ids(1), Next: "b"}, limit: 1, before: "a"},
		{name: "opaque cursor plus equals and ampersand", wire: threadPageWire{Threads: ids(1), Next: "abc+=&&"}, limit: 1},
		{name: "one character ID", wire: threadPageWire{Threads: []threadReferenceWire{{ID: "h"}}}, limit: 1},
		{name: "maximum ID bytes", wire: threadPageWire{Threads: []threadReferenceWire{{ID: ThreadID(wide)}}}, limit: 1},
		{name: "maximum continuation bytes", wire: threadPageWire{Threads: ids(1), Next: PageToken(cursor)}, limit: 1},
		{name: "provider runs over requested count", wire: threadPageWire{Threads: ids(2)}, limit: 1, want: core.ErrGmailResponse},
		{name: "provider runs one past sdk page bound", wire: threadPageWire{Threads: ids(ThreadPageMaximum + 1)}, limit: ThreadPageMaximum, want: core.ErrGmailResponse},
		{name: "empty page may not advance cursor", wire: threadPageWire{Next: "next"}, limit: 1, want: core.ErrGmailResponse},
		{name: "identical cursor may not loop", wire: threadPageWire{Threads: ids(1), Next: "same"}, limit: 1, before: "same", want: core.ErrGmailResponse},
		{name: "duplicate IDs refused", wire: threadPageWire{Threads: []threadReferenceWire{{ID: "x"}, {ID: "x"}}}, limit: 2, want: core.ErrGmailResponse},
		{name: "no ID in one returned row", wire: threadPageWire{Threads: []threadReferenceWire{{}}}, limit: 1, want: core.ErrGmailResponse},
		{name: "extra byte in ID", wire: threadPageWire{Threads: []threadReferenceWire{{ID: ThreadID(wide + "a")}}}, limit: 1, want: core.ErrGmailResponse},
		{name: "extra byte in cursor", wire: threadPageWire{Threads: ids(1), Next: PageToken(cursor + "a")}, limit: 1, want: core.ErrGmailResponse},
		{name: "separator in ID", wire: threadPageWire{Threads: []threadReferenceWire{{ID: "route%2fsecret"}}}, limit: 1, want: core.ErrGmailResponse},
		{name: "nonascii cursor", wire: threadPageWire{Threads: ids(1), Next: "é"}, limit: 1, want: core.ErrGmailResponse},
		{name: "cursor with line break", wire: threadPageWire{Threads: ids(1), Next: "abc\n"}, limit: 1, want: core.ErrGmailResponse},
		{name: "cursor with leading space", wire: threadPageWire{Threads: ids(1), Next: " next"}, limit: 1, want: core.ErrGmailResponse},
		{name: "path traversal in later row withholds prefix", wire: threadPageWire{Threads: []threadReferenceWire{{ID: "valid"}, {ID: "a/../b"}}}, limit: 2, want: core.ErrGmailResponse},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := tc.wire.project(ThreadListRequest{Limit: tc.limit, After: tc.before})
			if tc.want != nil {
				if !errors.Is(err, tc.want) || got.Threads != nil || got.Next != "" {
					t.Fatalf("project() = (%+v,%v), want zero and %v", got, err, tc.want)
				}
				return
			}
			want := make([]ThreadID, 0, len(tc.wire.Threads))
			for _, ref := range tc.wire.Threads {
				want = append(want, ref.ID)
			}
			if err != nil || !slices.Equal(got.Threads, want) || got.Next != tc.wire.Next {
				t.Fatalf("project() = (%+v,%v), want IDs %q and cursor %q", got, err, want, tc.wire.Next)
			}
		})
	}
}

func TestGmailDeadlineAndTextLimits(t *testing.T) {
	t.Parallel()
	t.Run("provider instant normalization", func(t *testing.T) {
		t.Parallel()
		maximum := uint64(math.MaxInt64 / temporal.NanosecondsPerMillisecond)
		for _, tc := range []struct {
			raw    internalDateWire
			millis uint64
		}{
			{raw: "1", millis: 1},
			{raw: internalDateWire(strconv.FormatUint(maximum, 10)), millis: maximum},
			{raw: "0"}, {raw: "-1"}, {raw: "+1"}, {raw: "01"}, {raw: "1.0"}, {raw: ""},
			{raw: internalDateWire(strconv.FormatUint(maximum+1, 10))},
		} {
			got, err := projectDate(tc.raw)
			if tc.millis == 0 {
				if !errors.Is(err, core.ErrGmailResponse) || got != (temporal.Instant{}) {
					t.Fatalf("projectDate(%q) = (%v,%v)", tc.raw, got, err)
				}
			} else if err != nil || got != temporal.InstantFromNanoseconds(int64(tc.millis*temporal.NanosecondsPerMillisecond)) {
				t.Fatalf("projectDate(%q) = (%v,%v), want %d ms", tc.raw, got, err, tc.millis)
			}
		}
	})
	t.Run("plain body limit both base64 encodings", func(t *testing.T) {
		t.Parallel()
		for _, enc := range []*base64.Encoding{base64.URLEncoding, base64.RawURLEncoding} {
			for _, length := range []int{MessageTextMaximumBytes - 1, MessageTextMaximumBytes, MessageTextMaximumBytes + 1} {
				plain := strings.Repeat("x", length)
				got, kind, err := base64URLWire(enc.EncodeToString([]byte(plain))).plainText()
				if length <= MessageTextMaximumBytes {
					if err != nil || kind != BodyPlainText || got != PlainText(plain) {
						t.Fatalf("plainText(%d) = (%d,%v,%v)", length, len(got), kind, err)
					}
				} else if !errors.Is(err, core.ErrExchangeBodyLimit) || got != "" || kind != BodyInvalid {
					t.Fatalf("plainText(%d) = (%d,%v,%v), want extent refusal", length, len(got), kind, err)
				}
			}
		}
	})
}

func TestGmailCredentialCustodyAndUnboundOperation(t *testing.T) {
	t.Parallel()
	access := fixtureAccess(t)
	copy := access
	if data, err := json.Marshal(access); !errors.Is(err, core.ErrJSONContract) || len(data) != 0 {
		t.Fatalf("marshal access = (%d bytes,%v), want refusal", len(data), err)
	}
	if err := copy.Close(); err != nil || access.Validate() == nil {
		t.Fatalf("closing copy = (%v, original %v), want invalidated original", err, access.Validate())
	}
	operation := Server{}.ReadThread()
	if err := operation.Validate(); !errors.Is(err, core.ErrGmailContract) {
		t.Fatalf("unbound operation.Validate() = %v", err)
	}
	value, err := operation.Execute(t.Context(), ThreadReadRequest{})
	if err == nil || value.ID != "" || value.Messages != nil {
		t.Fatalf("unbound operation produced (%+v,%v)", value, err)
	}
}
