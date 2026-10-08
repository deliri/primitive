package filestore_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestPipeTransferJoinsNativeEndpointsAndPreservesExactBytes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		payload []byte
	}{
		{name: "empty transfer owns a real native lifetime"},
		{name: "binary bytes cross native pipe unchanged", payload: []byte{0, 255, 13, 10}},
		{name: "transfer exceeds native pipe capacity without a retained inventory", payload: bytes.Repeat([]byte{0, 255, 13, 10}, 1<<18)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var source, destination *os.File
			var got bytes.Buffer
			producerCalls, consumerCalls := 0, 0
			result, err := filestore.TransferPipe(t.Context(), filestore.PipeTransferRequest{
				Produce: func(_ context.Context, w io.Writer) error {
					producerCalls++
					destination = w.(*os.File)
					_, err := io.Copy(w, bytes.NewReader(tc.payload))
					return err
				},
				Consume: func(_ context.Context, r io.Reader) error {
					consumerCalls++
					source = r.(*os.File)
					_, err := io.Copy(&got, r)
					return err
				},
			})
			if err != nil || result.Validate() != nil || result.ProducerError != nil || result.ConsumerError != nil || result.CleanupError != nil {
				t.Fatalf("native transfer = (%+v,%v), want joined lifetime without failure", result, err)
			}
			if producerCalls != 1 || consumerCalls != 1 || !bytes.Equal(got.Bytes(), tc.payload) {
				t.Errorf("transfer calls=%d/%d bytes=%d, want one call each and %d exact bytes", producerCalls, consumerCalls, got.Len(), len(tc.payload))
			}
			for _, endpoint := range []*os.File{source, destination} {
				if _, err := endpoint.Stat(); !errors.Is(err, os.ErrClosed) {
					t.Errorf("borrowed endpoint after join = %v, want %v", err, os.ErrClosed)
				}
			}
		})
	}
}

func TestPipeTransferRefusalAndPanicCannotLeaveProducerDetached(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name          string
		consumerErr   error
		producerErr   error
		producerPanic bool
		consumerPanic bool
	}{
		{name: "consumer refusal closes native backpressure and joins producer", consumerErr: io.ErrUnexpectedEOF},
		{name: "producer refusal preserves its typed cause", producerErr: io.ErrClosedPipe},
		{name: "producer panic remains a refusal after native closure", producerPanic: true},
		{name: "consumer panic remains a refusal and releases native producer", consumerPanic: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var source, destination *os.File
			producerReturned := false
			result, err := filestore.TransferPipe(t.Context(), filestore.PipeTransferRequest{
				Produce: func(_ context.Context, w io.Writer) error {
					destination = w.(*os.File)
					defer func() { producerReturned = true }()
					if tc.producerPanic {
						panic(io.ErrShortWrite)
					}
					if tc.producerErr != nil {
						return tc.producerErr
					}
					block := [4096]byte{}
					for range 4096 {
						if _, err := w.Write(block[:]); err != nil {
							return err
						}
					}
					return nil
				},
				Consume: func(_ context.Context, r io.Reader) error {
					source = r.(*os.File)
					if tc.consumerPanic {
						panic(io.ErrShortBuffer)
					}
					if tc.consumerErr != nil {
						return tc.consumerErr
					}
					_, err := io.Copy(io.Discard, r)
					return err
				},
			})
			if err != nil || result.Validate() != nil || !producerReturned {
				t.Fatalf("transfer refusal = (%+v,%v), producer returned=%t; want joined lifetime", result, err, producerReturned)
			}
			if tc.consumerErr != nil && !errors.Is(result.ConsumerError, tc.consumerErr) {
				t.Errorf("consumer error=%v, want %v", result.ConsumerError, tc.consumerErr)
			}
			if tc.producerErr != nil && !errors.Is(result.ProducerError, tc.producerErr) {
				t.Errorf("producer error=%v, want %v", result.ProducerError, tc.producerErr)
			}
			if tc.producerPanic && !errors.Is(result.ProducerError, core.ErrFilestoreContract) {
				t.Errorf("producer panic=%v, want %v", result.ProducerError, core.ErrFilestoreContract)
			}
			if tc.consumerPanic && !errors.Is(result.ConsumerError, core.ErrFilestoreContract) {
				t.Errorf("consumer panic=%v, want %v", result.ConsumerError, core.ErrFilestoreContract)
			}
			if (tc.consumerPanic || tc.consumerErr != nil) && result.ProducerError == nil {
				t.Errorf("producer unexpectedly succeeded after reader refusal: %+v", result)
			}
			for _, endpoint := range []*os.File{source, destination} {
				if _, err := endpoint.Stat(); !errors.Is(err, os.ErrClosed) {
					t.Errorf("borrowed endpoint after refusal = %v, want %v", err, os.ErrClosed)
				}
			}
		})
	}
}

func TestPipeTransferMissingCapabilitiesAcquireNoLifetime(t *testing.T) {
	t.Parallel()
	called := false
	for _, request := range []filestore.PipeTransferRequest{
		{},
		{Produce: func(context.Context, io.Writer) error { called = true; return nil }},
		{Consume: func(context.Context, io.Reader) error { called = true; return nil }},
	} {
		result, err := filestore.TransferPipe(t.Context(), request)
		if !errors.Is(err, core.ErrFilestoreContract) || result.Validate() == nil || called {
			t.Fatalf("missing capability = (%+v,%v), called=%t; want invalid zero result and typed refusal", result, err, called)
		}
	}
}

func FuzzPipeTransferPreservesNativeBytes(f *testing.F) {
	for _, payload := range [][]byte{nil, {0, 255, 13, 10}, bytes.Repeat([]byte("native"), 1<<14)} {
		f.Add(payload)
	}
	f.Fuzz(func(t *testing.T, payload []byte) {
		var got bytes.Buffer
		result, err := filestore.TransferPipe(t.Context(), filestore.PipeTransferRequest{
			Produce: func(_ context.Context, w io.Writer) error { _, err := io.Copy(w, bytes.NewReader(payload)); return err },
			Consume: func(_ context.Context, r io.Reader) error { _, err := io.Copy(&got, r); return err },
		})
		if err != nil || result.Validate() != nil || result.ProducerError != nil || result.ConsumerError != nil || result.CleanupError != nil || !bytes.Equal(got.Bytes(), payload) {
			t.Fatalf("native pipe bytes=%d, expected=%d, outcome=(%+v,%v); want exact bytes and joined error-free lifetime", got.Len(), len(payload), result, err)
		}
	})
}

func TestPipeTransferContextRefusalNeverInvokesCallbacks(t *testing.T) {
	t.Parallel()
	canceled, cancel := context.WithCancel(t.Context())
	cancel()
	for _, tc := range []struct {
		name    string
		ctx     context.Context
		wantErr error
	}{
		{name: "missing context", wantErr: core.ErrNilContext},
		{name: "canceled parent", ctx: canceled, wantErr: context.Canceled},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			called := false
			result, err := filestore.TransferPipe(tc.ctx, filestore.PipeTransferRequest{
				Produce: func(context.Context, io.Writer) error { called = true; return nil },
				Consume: func(context.Context, io.Reader) error { called = true; return nil },
			})
			if !errors.Is(err, tc.wantErr) || result.Validate() == nil || called {
				t.Fatalf("context refusal=(%+v,%v), called=%t; want invalid zero result and %v", result, err, called, tc.wantErr)
			}
		})
	}
}
