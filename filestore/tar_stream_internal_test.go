package filestore

import (
	"errors"
	"go/ast"
	"io"
	"math"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestTARStreamCounterPreservesOutputDomainAndNativeFailures(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name     string
		count    uint64
		data     []byte
		writer   io.Writer
		want     error
		accepted uint64
	}{
		{name: "exact-signed-boundary", count: math.MaxInt64 - 1, data: []byte{1}, writer: io.Discard, accepted: math.MaxInt64},
		{name: "cross-signed-boundary", count: math.MaxInt64, data: []byte{1}, writer: io.Discard, want: core.ErrNumericOverflow, accepted: math.MaxInt64},
		{name: "invalid-counter-cannot-wrap", count: math.MaxUint64, data: []byte{1}, writer: io.Discard, want: core.ErrNumericOverflow, accepted: math.MaxUint64},
		{name: "negative-native-count", data: []byte{1}, writer: tarStreamNativeFault{count: -1}, want: io.ErrShortWrite},
		{name: "overreported-native-count", data: []byte{1}, writer: tarStreamNativeFault{count: 2}, want: io.ErrShortWrite},
		{name: "short-native-count", data: []byte{1, 2}, writer: tarStreamNativeFault{count: 1}, want: io.ErrShortWrite, accepted: 1},
		{name: "partial-native-failure", data: []byte{1, 2}, writer: tarStreamNativeFault{count: 1, cause: io.ErrClosedPipe}, want: io.ErrClosedPipe, accepted: 1},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			sink := tarStreamDestination{destination: row.writer, count: row.count}
			_, err := sink.Write(row.data)
			if !errors.Is(err, row.want) || sink.count != row.accepted {
				t.Fatalf("counter=%d err=%v want=%d/%v", sink.count, err, row.accepted, row.want)
			}
			if row.want != nil {
				n, again := sink.Write(nil)
				if n != 0 || !errors.Is(again, row.want) {
					t.Fatalf("failure latch lost: %d/%v", n, again)
				}
			}
		})
	}
}

type tarStreamNativeFault struct {
	cause error
	count int
}

func (w tarStreamNativeFault) Write([]byte) (int, error) { return w.count, w.cause }

// Hash elimination has no archive-byte signature. This narrow ownership
// ratchet proves the stream entry point selects the common encoder without a
// whole-archive digest; native equivalence and performance are separate proof.
func TestTARStreamWholeArchiveIndexOwnership(t *testing.T) {
	t.Parallel()
	found := false
	for _, source := range parseProductionFiles(t) {
		for _, declaration := range source.file.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok || function.Name.Name != "StreamTAR" {
				continue
			}
			found = true
			calls := 0
			ast.Inspect(function.Body, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				name, ok := call.Fun.(*ast.Ident)
				if !ok || name.Name != "writeTAR" {
					t.Errorf("stream entry point adds another operation, want only the common native encoder")
					return true
				}
				calls++
				if len(call.Args) != 3 {
					t.Errorf("common encoder selection has %d arguments", len(call.Args))
					return true
				}
				digest, ok := call.Args[2].(*ast.Ident)
				if !ok || digest.Name != "nil" {
					t.Error("stream entry point constructs a whole-archive digest")
				}
				return true
			})
			if calls != 1 {
				t.Errorf("stream entry point selects %d encoders want1", calls)
			}
		}
	}
	if !found {
		t.Fatal("stream entry point absent")
	}
}
