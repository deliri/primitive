package core

import (
	"crypto/sha256"
	"errors"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"
)

func FuzzDigestWriterStreamAndReset(f *testing.F) {
	for _, seed := range [][]byte{nil, {}, []byte{0, 0xff}, []byte("streamed bytes"), make([]byte, sha256.BlockSize+1)} {
		f.Add(seed, uint32(len(seed)/2), false)
		f.Add(seed, uint32(len(seed)/2), true)
	}
	f.Fuzz(func(t *testing.T, data []byte, rawSplit uint32, reset bool) {
		split := int(uint64(rawSplit) % uint64(len(data)+1))
		writer := NewDigestWriter()
		n, err := writer.Write(data[:split])
		if err != nil || n != split {
			t.Fatalf("prefix write=%d, %v; want %d", n, err, split)
		}
		prefix, count, err := writer.Digest()
		if err != nil || prefix != NewSHA256Digest(sha256.Sum256(data[:split])) || count.Uint64() != uint64(split) {
			t.Fatalf("prefix=%v, %v, %v; want exact first %d bytes", prefix, count, err, split)
		}
		wantBytes := data
		if reset {
			if err := writer.Reset(); err != nil {
				t.Fatal(err)
			}
			wantBytes = data[split:]
		}
		n, err = writer.Write(data[split:])
		if err != nil || n != len(data)-split {
			t.Fatalf("suffix write=%d, %v; want %d", n, err, len(data)-split)
		}
		sum, count, err := writer.Seal()
		want := NewSHA256Digest(sha256.Sum256(wantBytes))
		if err != nil || sum != want || count.Uint64() != uint64(len(wantBytes)) {
			t.Fatalf("sealed=%v, %v, %v; want Go digest %v and count %d", sum, count, err, want, len(wantBytes))
		}
		again, againCount, err := writer.Digest()
		if err != nil || again != sum || againCount != count {
			t.Fatalf("sealed peek=%v, %v, %v; want sealed facts", again, againCount, err)
		}
		n, err = writer.Write(nil)
		if n != 0 || !errors.Is(err, ErrPrimitiveContract) {
			t.Fatalf("sealed empty write=%d, %v; want persistent refusal", n, err)
		}
	})
}

func FuzzHTTPEndpointCanonicalProjection(f *testing.F) {
	for _, source := range []string{
		"https://example.invalid/", "http://example.invalid:80/%2f", "https://example.invalid/ ",
		"https://example.invalid/" + strings.Repeat(" ", httpEndpointMaximumBytes/3),
		"https://example.invalid/" + strings.Repeat("é", httpEndpointMaximumBytes/6),
		"https://user@example.invalid/", "https://example.invalid/#", "https://example.invalid:0/", "", "\xff",
	} {
		f.Add(source)
	}
	f.Fuzz(func(t *testing.T, source string) {
		native, nativeErr := url.Parse(source)
		wantOK := nativeErr == nil && len(source) > 0 && len(source) <= httpEndpointMaximumBytes && !strings.Contains(source, "#")
		if wantOK {
			wantOK = native.IsAbs() && native.Opaque == "" && (native.Scheme == httpSchemeText || native.Scheme == SchemeHTTPS) && native.User == nil && native.Hostname() != "" && native.Fragment == "" && native.RawFragment == "" && !strings.ContainsAny(native.Host, "\r\n\t ") && len(native.String()) <= httpEndpointMaximumBytes
		}
		if wantOK && native.Port() != "" {
			port, err := strconv.ParseUint(native.Port(), 10, 16)
			wantOK = err == nil && port != 0 && strconv.FormatUint(port, 10) == native.Port()
		}
		got, err := ParseHTTPEndpoint(source)
		if (err == nil) != wantOK {
			t.Fatalf("endpoint admission=%v; want %t for %d source bytes", err, wantOK, len(source))
		}
		if !wantOK {
			if got != (HTTPEndpoint{}) || !errors.Is(err, ErrPrimitiveContract) {
				t.Fatalf("refused endpoint=%v, %v; want zero and typed refusal", got, err)
			}
			return
		}
		if got.HTTPURL() != *native || got.String() != native.String() {
			t.Fatalf("endpoint=%v; want exact Go URL %v", got.HTTPURL(), native)
		}
		again, againErr := ParseHTTPEndpoint(got.String())
		if againErr != nil || again.String() != got.String() || !again.SameOrigin(got) {
			t.Fatalf("endpoint reparse=%v, %v; want stable publication", again, againErr)
		}
	})
}

func FuzzAbsolutePathResolveTextIngress(f *testing.F) {
	for _, source := range []string{".", "..", "child", "discard/../kept", "\x00/../kept", "\xff/../kept", strings.Repeat("/", filesystemPathMaximumRunes+1)} {
		f.Add(source)
	}
	base, err := ParseAbsolutePath(filepath.Join(filepath.VolumeName(f.TempDir())+string(filepath.Separator), "resolve-base"))
	if err != nil {
		f.Fatal(err)
	}
	f.Fuzz(func(t *testing.T, source string) {
		wantText := filepath.Join(base.String(), source)
		if filepath.IsAbs(source) {
			wantText = filepath.Clean(source)
		}
		wantPath, pathErr := ParseAbsolutePath(wantText)
		// Go owns lexical resolution; the raw input gate is independently stated
		// here so normalization cannot erase malformed bytes before admission.
		wantOK := source != "" && utf8.ValidString(source) && utf8.RuneCountInString(source) <= filesystemPathMaximumRunes && !strings.ContainsRune(source, 0) && pathErr == nil
		got, err := base.ResolveText(source)
		if (err == nil) != wantOK || wantOK && got != wantPath {
			t.Fatalf("resolution=%v, %v; want Go path %v with raw admission %t", got, err, wantPath, wantOK)
		}
		if !wantOK && (got != (AbsolutePath{}) || !errors.Is(err, ErrPrimitiveContract)) {
			t.Fatalf("refused resolution=%v, %v; want zero and contract error", got, err)
		}
	})
}

func FuzzIssueProjectionStructuralLimits(f *testing.F) {
	for _, wire := range []string{`{}`, `{"a":1,"A":2}`, `{"K":1,"K":2}`, `[1,2,3]`, `"é"`, `null`, `{"a":[{"b":1}]}`} {
		f.Add([]byte(wire), uint16(32), uint8(3), uint8(3), uint8(3))
	}
	f.Fuzz(func(t *testing.T, wire []byte, rawBytes uint16, rawDepth, rawFields, rawItems uint8) {
		limits := DefaultStrictJSONLimits()
		limits.DocumentMaximumBytes = ByteCount{value: uint64(rawBytes) + 1}
		limits.NestingDepthMaximum = uint16(rawDepth%JSONNestingDepthMaximum) + 1
		limits.ObjectFieldMaximum = uint16(rawFields) + 1
		limits.ArrayItemMaximum = uint64(rawItems) + 1
		// The independent public structural door owns the same mechanical limits;
		// a product's always-successful semantic validator cannot bypass that door.
		_, structureErr := DecodeStrictJSONStructure[jsonProjectionShape](wire, limits)
		wantOK := structureErr == nil && strings.TrimSpace(string(wire)) != jsonNullLiteralText
		calls := 0
		got, err := EncodeValidatedJSON(untrustedIssueProjection{wire: wire, calls: &calls}, limits)
		if (err == nil) != wantOK {
			t.Fatalf("projection=%d bytes, %v; structural admission=%t", len(got), err, wantOK)
		}
		if !wantOK {
			if got != nil || calls != 0 || !errors.Is(err, ErrJSONContract) {
				t.Fatalf("refused projection=%d bytes, calls=%d, %v; want no publication or callback", len(got), calls, err)
			}
		} else if string(got) != string(wire) || calls != 1 {
			t.Fatalf("projection=%d bytes, calls=%d; want exact %d bytes and one callback", len(got), calls, len(wire))
		}
	})
}

// This fixture accepts any syntactically valid JSON after Core's structural
// scanner. Its method deliberately adds no type-specific grammar or policy.
type jsonProjectionShape struct{}

func (*jsonProjectionShape) UnmarshalJSON([]byte) error { return nil }
