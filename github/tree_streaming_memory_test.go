package github

import (
	"bufio"
	"bytes"
	"crypto/sha256"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"github.com/deliri/primitive/v2026/core"
	"io"
	"strings"
	"testing"
)

type repeatedTreeByteReader struct {
	remaining, read int64
	value           byte
}

func (r *repeatedTreeByteReader) Read(p []byte) (int, error) {
	if r.remaining == 0 {
		return 0, io.EOF
	}
	n := min(int64(len(p)), r.remaining)
	for i := range int(n) {
		p[i] = r.value
	}
	r.remaining -= n
	r.read += n
	return int(n), nil
}

type streamingPathVisitor struct {
	visits, bytes uint64
	wantByte      byte
}

func (v *streamingPathVisitor) VisitGitHubTreeEntry(stream *TreeEntryStream) error {
	before, err := stream.Observation()
	if !errors.Is(err, core.ErrGitHubResponse) || before != (TreeEntry{}) {
		return core.ErrGitHubResponse
	}
	digest := sha256.New()
	var scratch [4096]byte
	var count uint64
	for {
		n, err := stream.Read(scratch[:])
		for _, value := range scratch[:n] {
			if value != v.wantByte {
				return core.ErrGitHubResponse
			}
		}
		if _, hashErr := digest.Write(scratch[:n]); hashErr != nil {
			return hashErr
		}
		count += uint64(n)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return err
		}
	}
	entry, err := stream.Observation()
	if err != nil {
		return err
	}
	var sum [sha256.Size]byte
	digest.Sum(sum[:0])
	if entry.Kind != TreeEntryBlob || entry.PathLength.Uint64() != count || entry.PathSHA256 != core.NewSHA256Digest(sum) {
		return core.ErrGitHubResponse
	}
	v.visits++
	v.bytes += count
	return nil
}
func TestTreeStreamsLargeValuesWithoutInputCeilings(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, prefix, suffix  string
		value                 byte
		wantVisits, wantBytes uint64
	}{
		{name: "large path belongs to caller", prefix: `{"sha":"s","url":"u","tree":[{"path":"`, suffix: `","type":"blob"}],"truncated":false}`, value: 'a', wantVisits: 1, wantBytes: 2 << 20},
		{name: "large ignored root SHA", prefix: `{"sha":"`, suffix: `","url":"u","tree":[],"truncated":false}`, value: 'a'},
		{name: "large ignored root URL", prefix: `{"sha":"s","url":"`, suffix: `","tree":[],"truncated":false}`, value: 'a'},
		{name: "large ignored entry URL", prefix: `{"sha":"s","url":"u","tree":[{"url":"`, suffix: `","path":"a","type":"blob"}],"truncated":false}`, value: 'a', wantVisits: 1, wantBytes: 1},
		{name: "large leading whitespace", suffix: `{"sha":"s","url":"u","tree":[],"truncated":false}`, value: ' '},
		{name: "large inter-member whitespace", prefix: `{"sha":"s",`, suffix: `"url":"u","tree":[],"truncated":false}`, value: ' '},
		{name: "large trailing whitespace", prefix: `{"sha":"s","url":"u","tree":[],"truncated":false}`, value: ' '},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			source := &repeatedTreeByteReader{remaining: 2 << 20, value: tc.value}
			visitor := &streamingPathVisitor{wantByte: 'a'}
			got, err := decodeTree(io.MultiReader(strings.NewReader(tc.prefix), source, strings.NewReader(tc.suffix)), visitor)
			if err != nil || got != tc.wantVisits || visitor.visits != tc.wantVisits || visitor.bytes != tc.wantBytes || source.read != 2<<20 {
				t.Fatalf("stream=%d/%v visits=%d path bytes=%d source consumed=%d, want %d/%d and entire 2 MiB source", got, err, visitor.visits, visitor.bytes, source.read, tc.wantVisits, tc.wantBytes)
			}
		})
	}
}
func BenchmarkTreePathStreaming(b *testing.B) {
	b.ReportAllocs()
	for _, size := range []int64{2 << 20, 16 << 20} {
		b.Run(fmt.Sprintf("path_bytes_%d", size), func(b *testing.B) {
			b.ReportAllocs()
			b.SetBytes(size)
			for b.Loop() {
				source := &repeatedTreeByteReader{remaining: size, value: 'a'}
				visitor := &streamingPathVisitor{wantByte: 'a'}
				got, err := decodeTree(io.MultiReader(strings.NewReader(`{"sha":"s","url":"u","tree":[{"path":"`), source, strings.NewReader(`","type":"blob"}],"truncated":false}`)), visitor)
				if err != nil || got != 1 || visitor.bytes != uint64(size) || source.read != size {
					b.Fatalf("stream=%d/%v path=%d source=%d, want one complete %d-byte path", got, err, visitor.bytes, source.read, size)
				}
			}
		})
	}
}
func FuzzGitHubJSONStringStreamingSemanticClosure(f *testing.F) {
	for _, value := range []string{"ordinary", "é😀", "escaped\ntext", ""} {
		seed, err := json.Marshal(value)
		if err != nil {
			f.Fatal(err)
		}
		f.Add(seed)
	}
	f.Add([]byte{'"', 0xff, '"'})
	f.Add([]byte(`"\uD800"`))
	f.Add([]byte(`"truncated`))
	f.Fuzz(func(t *testing.T, payload []byte) {
		if len(payload) > 256<<10 {
			t.Skip("input exceeds caller-owned aggregate oracle custody")
		}
		var want string
		wantErr := json.Unmarshal(payload, &want)
		// JSON null is not a string even though Go's string unmarshaler clears it.
		if bytes.Equal(bytes.TrimSpace(payload), []byte("null")) {
			wantErr = core.ErrGitHubResponse
		}
		decoder := treeDecoder{source: bufio.NewReader(bytes.NewReader(payload))}
		var got bytes.Buffer
		err := decoder.expect('"')
		if err == nil {
			stream := jsonStringStream{source: decoder.source}
			_, err = io.Copy(&got, &stream)
			if err == nil {
				if _, endErr := decoder.next(); !errors.Is(endErr, io.EOF) {
					err = responseError(endErr)
				}
			}
		}
		if wantErr != nil {
			if !errors.Is(err, core.ErrGitHubResponse) {
				t.Fatalf("string refusal=%v, want typed refusal", err)
			}
			return
		}
		if err != nil || got.String() != want {
			t.Fatalf("string=%q/%v, want %q/nil", got.String(), err, want)
		}
	})
}
