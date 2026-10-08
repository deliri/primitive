package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/hostfacts"
)

func TestRewindRestoresNativeReadCoordinateWithoutChangingBytes(t *testing.T) {
	t.Parallel()
	parent, err := hostfacts.TemporaryDirectory()
	if err != nil {
		t.Fatal(err)
	}
	err = filestore.WithScratchScope(t.Context(), filestore.ScratchScopeRequest{Parent: parent, Use: func(ctx context.Context, root *os.Root) (resultErr error) {
		path, err := core.ParseRelativePath("source")
		if err != nil {
			return err
		}
		file, err := filestore.OpenScratch(ctx, filestore.ScratchRequest{Location: filestore.Location{Root: root, Path: path}, Mode: 0o600})
		if err != nil {
			return err
		}
		content := []byte("exact immutable source")
		if n, err := file.Write(content); err != nil || n != len(content) {
			return errors.Join(err, io.ErrShortWrite, file.Close())
		}
		if err := file.Close(); err != nil {
			return err
		}
		file, err = filestore.OpenRead(ctx, filestore.ReadHandleRequest{Location: filestore.Location{Root: root, Path: path}})
		if err != nil {
			return err
		}
		defer func() { resultErr = errors.Join(resultErr, file.Close()) }()
		if _, err := io.Copy(io.Discard, file); err != nil {
			return err
		}
		if err := filestore.Rewind(ctx, filestore.RewindRequest{Source: file}); err != nil {
			return err
		}
		actual, err := io.ReadAll(file)
		if err != nil {
			return err
		}
		if !bytes.Equal(actual, content) {
			t.Errorf("replayed source=%q, want %q", actual, content)
		}
		if position, err := file.Seek(0, io.SeekCurrent); err != nil || position != int64(len(content)) {
			return errors.Join(err, core.ErrFilestoreContract)
		}
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
}

func TestRewindRefusesInvalidLifetimeAndPreservesNativeFailure(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		nilContext bool
		nilSource  bool
		canceled   bool
		closed     bool
		want       error
	}{
		{name: "nil context refuses before moving source", nilContext: true, want: core.ErrNilContext},
		{name: "missing source refuses", nilSource: true, want: core.ErrFilestoreContract},
		{name: "canceled ingress refuses before moving source", canceled: true, want: context.Canceled},
		{name: "closed native source retains closed identity", closed: true, want: fs.ErrClosed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			parent, err := hostfacts.TemporaryDirectory()
			if err != nil {
				t.Fatal(err)
			}
			err = filestore.WithScratchScope(t.Context(), filestore.ScratchScopeRequest{Parent: parent, Use: func(ctx context.Context, root *os.Root) (resultErr error) {
				path, err := core.ParseRelativePath("source")
				if err != nil {
					return err
				}
				file, err := filestore.OpenScratch(ctx, filestore.ScratchRequest{Location: filestore.Location{Root: root, Path: path}, Mode: 0o600})
				if err != nil {
					return err
				}
				if tc.closed {
					if err := file.Close(); err != nil {
						return err
					}
				} else {
					defer func() { resultErr = errors.Join(resultErr, file.Close()) }()
				}
				reader := bytes.NewReader([]byte("ab"))
				if _, err := reader.ReadByte(); err != nil {
					return err
				}
				var source io.Seeker = reader
				if tc.nilSource {
					source = nil
				}
				if tc.closed {
					source = file
				}
				operationContext := ctx
				if tc.nilContext {
					operationContext = nil
				}
				if tc.canceled {
					canceled, cancel := context.WithCancel(ctx)
					cancel()
					operationContext = canceled
				}
				got := filestore.Rewind(operationContext, filestore.RewindRequest{Source: source})
				if !errors.Is(got, tc.want) {
					t.Errorf("rewind error=%v, want %v", got, tc.want)
				}
				if reader.Len() != 1 {
					t.Errorf("refused rewind moved source: remaining=%d, want 1", reader.Len())
				}
				return nil
			}})
			if err != nil {
				t.Fatal(err)
			}
		})
	}
}

func FuzzRewindRestoresStandardStreamContent(f *testing.F) {
	f.Add([]byte("different prefix and suffix"))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, content []byte) {
		t.Parallel()
		source := bytes.NewReader(content)
		if _, err := source.Seek(int64(len(content)/2), io.SeekStart); err != nil {
			t.Fatal(err)
		}
		if err := filestore.Rewind(t.Context(), filestore.RewindRequest{Source: source}); err != nil {
			t.Fatal(err)
		}
		actual, err := io.ReadAll(source)
		if err != nil || !bytes.Equal(actual, content) {
			t.Fatalf("replayed content=%x, error=%v, want exact original %x", actual, err, content)
		}
	})
}

// This adversarial producer delegates to the real Go stream. It does not keep
// its own cursor; it can only cancel after execution or corrupt the observation.
type observedSeekSource struct {
	Source          io.Seeker
	After           func()
	CorruptPosition bool
}

func (s observedSeekSource) Seek(offset int64, origin int) (int64, error) {
	position, err := s.Source.Seek(offset, origin)
	if s.After != nil {
		s.After()
	}
	if s.CorruptPosition {
		position++
	}
	return position, err
}

var _ io.Seeker = observedSeekSource{}

func TestRewindRefusesCorruptObservationAndPreservesPostEffectCancellation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		cancelAfter bool
		corrupt     bool
		want        error
	}{
		{name: "native observation proves rewind"},
		{name: "corrupt observation cannot prove completion", corrupt: true, want: core.ErrFilestoreContract},
		{name: "post-effect cancellation remains cancellation", cancelAfter: true, want: context.Canceled},
		{name: "corrupt observation retains simultaneous cancellation", cancelAfter: true, corrupt: true, want: core.ErrFilestoreContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			source := bytes.NewReader([]byte("ab"))
			if _, err := source.ReadByte(); err != nil {
				t.Fatal(err)
			}
			observed := observedSeekSource{Source: source, CorruptPosition: tc.corrupt}
			if tc.cancelAfter {
				observed.After = cancel
			}
			err := filestore.Rewind(ctx, filestore.RewindRequest{Source: observed})
			if !errors.Is(err, tc.want) || errors.Is(err, context.Canceled) != tc.cancelAfter {
				t.Fatalf("rewind error=%v, want %v with canceled=%t", err, tc.want, tc.cancelAfter)
			}
			// A canceled result cannot be interpreted as an unchanged coordinate.
			if source.Len() != 2 {
				t.Fatalf("actual post-effect source remaining=%d, want 2", source.Len())
			}
		})
	}
}
