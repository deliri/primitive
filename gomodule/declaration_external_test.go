package gomodule_test

import (
	"context"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/gomodule"
)

func TestModuleDeclarationNativeReadPreservesClosureRefusal(t *testing.T) {
	t.Parallel()
	parent, err := core.ParseAbsolutePath(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	err = filestore.WithScratchScope(t.Context(), filestore.ScratchScopeRequest{
		Parent: parent,
		Use: func(ctx context.Context, root *os.Root) error {
			path, err := core.ParseRelativePath("go.mod")
			if err != nil {
				return err
			}
			temporary, err := core.ParseRelativePath("go.mod.stage")
			if err != nil {
				return err
			}
			location := filestore.Location{Root: root, Path: path}
			if _, err := filestore.Write(ctx, filestore.WriteRequest{
				Location: location, Temporary: temporary, Mode: 0o600, Install: filestore.InstallCreate,
				Source: strings.NewReader("// native source\nmodule example.com/app\n"),
			}); err != nil {
				return err
			}
			var borrowed io.Reader
			result, err := filestore.WithReadScope(ctx, filestore.ReadScopeRequest{
				Location: location,
				Use: func(ctx context.Context, source io.ReadSeeker) error {
					borrowed = source
					got, err := gomodule.ObserveDeclaration(ctx, gomodule.DeclarationRequest{Source: source})
					if err != nil {
						return err
					}
					if got.Presence != gomodule.DeclarationPresent || got.Path.String() != "example.com/app" {
						t.Errorf("native module projection = %+v", got)
					}
					return nil
				},
			})
			if err != nil {
				return err
			}
			if err := errors.Join(result.OperationError(), result.CleanupError()); err != nil {
				return err
			}
			// Deliberately probe the invalid borrowed lifetime after Primitive closes it.
			got, err := gomodule.ObserveDeclaration(ctx, gomodule.DeclarationRequest{Source: borrowed})
			if !errors.Is(err, fs.ErrClosed) || !errors.Is(err, core.ErrGoModuleContract) || got.Validate() == nil {
				t.Errorf("closed native source fabricated an observation: %+v/%v", got, err)
			}
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestModuleDeclarationStreamsIgnoredExtentAndRefusesInvalidIdentity(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		source   string
		path     string
		presence gomodule.DeclarationPresence
		refusal  error
	}{
		{"canonical directive", "module example.com/app\n", "example.com/app", gomodule.DeclarationPresent, nil},
		{"identity at EOF", "module example.com/app", "example.com/app", gomodule.DeclarationPresent, nil},
		{"quoted identity", "module \"example.com/app\"\n", "example.com/app", gomodule.DeclarationPresent, nil},
		{"quoted escape", "module \"example.com/\\x61pp\"\n", "example.com/app", gomodule.DeclarationPresent, nil},
		{"initial BOM", "\uFEFFmodule example.com/app\n", "example.com/app", gomodule.DeclarationPresent, nil},
		{"CRLF and tab separator", "module\texample.com/app\r\n", "example.com/app", gomodule.DeclarationPresent, nil},
		{"comment after identity", "module example.com/app // note\n", "example.com/app", gomodule.DeclarationPresent, nil},
		{"comment ending at EOF", "module example.com/app // note", "example.com/app", gomodule.DeclarationPresent, nil},
		{"first declaration owns projection", "module example.com/first\nmodule example.com/second\n", "example.com/first", gomodule.DeclarationPresent, nil},
		{"huge ignored token before declaration", strings.Repeat("x", 2<<20) + "\nmodule example.com/app\n", "example.com/app", gomodule.DeclarationPresent, nil},
		{"huge comment before declaration", "// " + strings.Repeat("x", 2<<20) + "\nmodule example.com/app\n", "example.com/app", gomodule.DeclarationPresent, nil},
		{"huge comment after identity", "module example.com/app // " + strings.Repeat("x", 2<<20), "example.com/app", gomodule.DeclarationPresent, nil},
		{"empty source reaches EOF", "", "", gomodule.DeclarationAbsent, nil},
		{"other directive has no identity", "go 1.27\n", "", gomodule.DeclarationAbsent, nil},
		{"comment is not a directive", "// module example.com/app\n", "", gomodule.DeclarationAbsent, nil},
		{"foreign keyword is not a directive", "modules example.com/app\n", "", gomodule.DeclarationAbsent, nil},
		{"missing identity at newline", "module\n", "", 0, core.ErrGoModuleContract},
		{"missing identity at EOF", "module", "", 0, core.ErrGoModuleContract},
		{"additional identity is refused", "module example.com/app extra\n", "", 0, core.ErrGoModuleContract},
		{"unterminated quote is refused", "module \"example.com/app\n", "", 0, core.ErrGoModuleContract},
		{"invalid quoted escape is refused", "module \"example.com/app\\q\"\n", "", 0, core.ErrGoModuleContract},
		{"absent quoted identity is refused", "module \"\"\n", "", 0, core.ErrGoModuleContract},
		{"invalid module domain is refused", "module app\n", "", 0, core.ErrGoModuleContract},
		{"path traversal is refused", "module example.com/app/../other\n", "", 0, core.ErrGoModuleContract},
		{"NUL is refused", "module example.com/\x00app\n", "", 0, core.ErrGoModuleContract},
		{"malformed UTF8 is refused", "module example.com/\xffapp\n", "", 0, core.ErrGoModuleContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := gomodule.ObserveDeclaration(t.Context(), gomodule.DeclarationRequest{Source: strings.NewReader(tc.source)})
			if !errors.Is(err, tc.refusal) || got.Path.String() != tc.path || got.Presence != tc.presence {
				t.Fatalf("declaration = %+v, %v; want %q, %v, %v", got, err, tc.path, tc.presence, tc.refusal)
			}
			if err == nil && got.Validate() != nil {
				t.Fatal("successful projection returned an invalid observation")
			}
			if err != nil && got.Validate() == nil {
				t.Fatal("refused projection returned an admitted observation")
			}
		})
	}
}

func TestModuleDeclarationIngressPreservesCancellationAndZeroMeaning(t *testing.T) {
	t.Parallel()
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	for _, tc := range []struct {
		name   string
		ctx    context.Context
		source io.Reader
		cause  error
	}{
		{"nil reader", t.Context(), nil, core.ErrGoModuleContract},
		{"typed nil reader", t.Context(), (*strings.Reader)(nil), core.ErrGoModuleContract},
		{"nil context", nil, strings.NewReader(""), core.ErrNilContext},
		{"cancelled context", ctx, strings.NewReader("module example.com/app\n"), context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := gomodule.ObserveDeclaration(tc.ctx, gomodule.DeclarationRequest{Source: tc.source})
			if !errors.Is(err, tc.cause) || got.Validate() == nil {
				t.Fatalf("refused ingress = %+v/%v, want %v and invalid zero", got, err, tc.cause)
			}
		})
	}
	for _, presence := range []gomodule.DeclarationPresence{0, 3, 255} {
		if !errors.Is(presence.Validate(), core.ErrGoModuleContract) {
			t.Fatalf("foreign presence %d acquired authority", presence)
		}
	}
	path, err := gomodule.ParsePath("example.com/app")
	if err != nil {
		t.Fatal(err)
	}
	if err := (gomodule.DeclarationObservation{Presence: gomodule.DeclarationAbsent, Path: path}).Validate(); !errors.Is(err, core.ErrGoModuleContract) {
		t.Fatalf("absent observation admitted a path: %v", err)
	}
}

func FuzzModuleDeclarationProjectionHasExclusivePresenceAndRefusal(f *testing.F) {
	for mode := uint8(0); mode < 6; mode++ {
		f.Add([]byte("ignored source"), mode)
	}
	f.Fuzz(func(t *testing.T, ignored []byte, mode uint8) {
		prefix := "// " + hex.EncodeToString(ignored) + "\n"
		body := "module example.com/app\n"
		presence := gomodule.DeclarationPresent
		var refusal error
		switch mode % 6 {
		case 1:
			body = "go 1.27\n"
			presence = gomodule.DeclarationAbsent
		case 2:
			body = "module example.com/app extra\n"
			presence, refusal = 0, core.ErrGoModuleContract
		case 3:
			body = "module \"example.com/app\" // observation\n"
		case 4:
			body = "modules example.com/app\n"
			presence = gomodule.DeclarationAbsent
		case 5:
			body = "module example.com/../app\n"
			presence, refusal = 0, core.ErrGoModuleContract
		}
		got, err := gomodule.ObserveDeclaration(t.Context(), gomodule.DeclarationRequest{Source: strings.NewReader(prefix + body)})
		if got.Presence != presence || !errors.Is(err, refusal) {
			t.Fatalf("projection acquired wrong authority: %+v/%v", got, err)
		}
		if presence == gomodule.DeclarationPresent && got.Path.String() != "example.com/app" {
			t.Fatalf("ignored extent changed identity: %v", got)
		}
		if presence != gomodule.DeclarationPresent && got.Path.String() != "" {
			t.Fatalf("absence or refusal fabricated identity: %v", got)
		}
	})
}

func BenchmarkModuleDeclarationIgnoredExtent(b *testing.B) {
	for _, size := range []int{2 << 10, 2 << 20} {
		b.Run(fmt.Sprintf("ignored-bytes-%d", size), func(b *testing.B) {
			source := strings.Repeat("x", size) + "\nmodule example.com/app\n"
			b.ReportAllocs()
			b.SetBytes(int64(len(source)))
			for b.Loop() {
				got, err := gomodule.ObserveDeclaration(b.Context(), gomodule.DeclarationRequest{Source: strings.NewReader(source)})
				if err != nil || got.Path.String() != "example.com/app" {
					b.Fatalf("projection = %+v/%v", got, err)
				}
			}
		})
	}
}
