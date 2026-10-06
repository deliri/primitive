package jsonio_test

import (
	"io"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/jsonio"
	"github.com/deliri/primitive/v2026/testserial"
)

const streamedJSONString = `"value",`

type repeatedJSONStringReader struct {
	remaining int
	offset    int
}

func (r *repeatedJSONStringReader) Read(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}
	if r.remaining == 0 {
		return 0, io.EOF
	}
	n := copy(data, streamedJSONString[r.offset:])
	r.offset += n
	if r.offset == len(streamedJSONString) {
		r.offset = 0
		r.remaining--
	}
	return n, nil
}

func TestJSONTokensArrayRetainedMemoryDoesNotScaleWithItems(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{
		Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess,
	})
	const allowance = 256 << 10
	for _, items := range []int{4096, 65536, 262144} {
		source := &repeatedJSONStringReader{remaining: items - 1}
		before, err := hostfacts.ObserveCollectedGoHeap(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		seen := 0
		for token, err := range jsonio.Tokens(t.Context(), jsonio.TokenRequest{
			Source: io.MultiReader(strings.NewReader("["), source, strings.NewReader(`"value"]`)), NestingDepthMaximum: 1,
		}) {
			if err != nil {
				t.Fatal(err)
			}
			if token.Kind == jsonio.TokenString {
				if token.Text != "value" {
					t.Fatalf("text = %q, want exact value", token.Text)
				}
				seen++
			} else if token.Kind != jsonio.TokenArrayStart && token.Kind != jsonio.TokenArrayEnd {
				t.Fatalf("array token = %+v, want string or delimiter", token)
			}
		}
		after, err := hostfacts.ObserveCollectedGoHeap(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if seen != items || source.remaining != 0 {
			t.Fatalf("array = (%d,%d unread), want (%d,0)", seen, source.remaining, items)
		}
		retained := int64(after.Uint64()) - int64(before.Uint64())
		t.Logf("items=%d retained_bytes=%d allowance=%d", items, retained, allowance)
		if retained > allowance {
			t.Fatalf("retained bytes = %d, want <= %d independent of array length", retained, allowance)
		}
	}
}
