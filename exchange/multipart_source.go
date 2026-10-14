package exchange

import (
	"context"
	"errors"
	"io"
	"iter"
	"mime/multipart"
	"strings"
	"sync"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
)

// multipartOwnedSource joins exactly one native close even when cancellation
// races normal completion. No previously consumed source is retained.
type multipartOwnedSource struct {
	source   io.ReadCloser
	once     sync.Once
	closeErr error
}

func (s *multipartOwnedSource) close() error {
	s.once.Do(func() {
		returned := false
		defer func() {
			if recover() != nil {
				s.closeErr = core.ErrExchangeCallbackPanic
			} else if !returned {
				s.closeErr = core.ErrExchangeCallbackExit
			}
		}()
		s.closeErr = s.source.Close()
		returned = true
	})
	return s.closeErr
}

// multipartLifetime holds only the currently admitted source and native pipe.
type multipartLifetime struct {
	mu       sync.Mutex
	active   *multipartOwnedSource
	reader   *io.PipeReader
	writer   *io.PipeWriter
	closeErr error
}

func (s *multipartLifetime) admit(ctx context.Context, source io.ReadCloser) (*multipartOwnedSource, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := contextstate.Validate(ctx); err != nil {
		return nil, err
	}
	owned := &multipartOwnedSource{source: source}
	s.active = owned
	return owned, nil
}
func (s *multipartLifetime) release(owned *multipartOwnedSource) error {
	err := owned.close()
	s.mu.Lock()
	s.active = nil
	if err != nil && s.closeErr == nil {
		// A failed native close stops production. Keep that one custody fact
		// even when Goexit unwinds the source without returning its result.
		s.closeErr = err
	}
	s.mu.Unlock()
	return err
}
func (s *multipartLifetime) closeCause() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.closeErr
}
func (s *multipartLifetime) interrupt(cause error) {
	// Native pipe closure releases a consumer before joining an active Read.
	_ = s.reader.CloseWithError(cause)
	_ = s.writer.CloseWithError(cause)
	s.mu.Lock()
	active := s.active
	s.mu.Unlock()
	if active != nil {
		_ = active.close()
	}
}

// multipartSourceReader prevents a borrowed consumer from acquiring endpoint
// cleanup authority through io.Closer's optional method set.
type multipartSourceReader struct{ reader *io.PipeReader }

func (r multipartSourceReader) Read(data []byte) (int, error) { return r.reader.Read(data) }

// WithMultipartSource produces native multipart framing through one unbuffered
// io.Pipe. Go owns quoting, boundaries and file headers (application/octet-stream).
// Exchange holds one TransferBufferBytes window and one admitted source. Cancel
// closes that source and both endpoints, then joins producer and cancellation
// callback before returning. The consumer must finish reading to EOF for complete
// production; an early return stops further output and preserves producer error.
// Callback panics are contained as Core identities without rendering their data.
// Producer Goexit closes the pipe with its distinct callback-exit identity;
// consumer Goexit preserves its native goroutine exit after deferred scope cleanup.
// Sources: https://pkg.go.dev/mime/multipart#Writer.CreateFormFile
// https://pkg.go.dev/io#Pipe and https://pkg.go.dev/context#AfterFunc
func WithMultipartSource(ctx context.Context, request MultipartSourceRequest) (resultErr error) {
	if err := contextstate.Validate(ctx); err != nil {
		return err
	}
	if err := request.Validate(); err != nil {
		return err
	}
	lifetime, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	reader, writer := io.Pipe()
	owner := &multipartLifetime{reader: reader, writer: writer}
	form := multipart.NewWriter(writer)
	contentType, err := core.ParseHTTPMediaType(form.FormDataContentType())
	if err != nil {
		return errors.Join(err, reader.Close(), writer.Close())
	}
	source := MultipartSource{Reader: multipartSourceReader{reader: reader}, ContentType: contentType}
	done := make(chan error, 1)
	afterDone := make(chan struct{})
	stop := context.AfterFunc(lifetime, func() { defer close(afterDone); owner.interrupt(context.Cause(lifetime)) })
	var useErr error
	consumerReturned := false
	defer func() {
		// Goexit unwinds defers without returning from Use. Custody cleanup
		// still cancels the scope and joins its actual native lifetimes.
		if !consumerReturned {
			useErr = core.ErrExchangeCallbackExit
		}
		cancel(useErr)
		if stop() {
			owner.interrupt(context.Cause(lifetime))
		} else {
			<-afterDone
		}
		produceErr := <-done
		resultErr = errors.Join(useErr, produceErr, contextstate.Validate(ctx))
	}()
	go func() {
		var produceErr error
		returned := false
		defer func() {
			if !returned {
				produceErr = core.ErrExchangeCallbackExit
			}
			produceErr = errors.Join(produceErr, owner.closeCause())
			done <- errors.Join(produceErr, writer.CloseWithError(produceErr))
		}()
		produceErr = produceMultipart(lifetime, owner, form, request.Parts)
		returned = true
	}()
	useErr = useMultipart(lifetime, request.Use, source)
	consumerReturned = true
	return nil
}

func useMultipart(ctx context.Context, use func(context.Context, MultipartSource) error, source MultipartSource) (resultErr error) {
	defer func() {
		if recover() != nil {
			resultErr = core.ErrExchangeCallbackPanic
		}
	}()
	return use(ctx, source)
}

func produceMultipart(ctx context.Context, owner *multipartLifetime, form *multipart.Writer, parts iter.Seq[MultipartPart]) (resultErr error) {
	defer func() {
		if recover() != nil {
			resultErr = core.ErrExchangeCallbackPanic
		}
	}()
	buffer := make([]byte, TransferBufferBytes)
	for part := range parts {
		if err := contextstate.Validate(ctx); err != nil {
			return err
		}
		if err := part.Validate(); err != nil {
			return err
		}
		if err := writeMultipartPart(ctx, owner, form, part, buffer); err != nil {
			return err
		}
	}
	if err := contextstate.Validate(ctx); err != nil {
		return err
	}
	return form.Close()
}

func writeMultipartPart(ctx context.Context, owner *multipartLifetime, form *multipart.Writer, part MultipartPart, buffer []byte) (resultErr error) {
	var destination io.Writer
	var source io.Reader
	switch part.Kind {
	case MultipartPartField:
		var err error
		destination, err = form.CreateFormField(part.Name.String())
		if err != nil {
			return err
		}
		source = strings.NewReader(part.Text.String())
	case MultipartPartFile:
		owned, err := owner.admit(ctx, part.Source)
		if err != nil {
			return err
		}
		defer func() { resultErr = errors.Join(resultErr, owner.release(owned)) }()
		destination, err = form.CreateFormFile(part.Name.String(), part.FileName.String())
		if err != nil {
			return err
		}
		source = part.Source
	default:
		return core.ErrExchangeContract
	}
	_, err := copyDownload(downloadCopyRequest{context: ctx, source: source, destination: destination, buffer: buffer})
	return err
}
