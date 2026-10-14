package jsonio_test

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	jsonv2 "encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/jsonio"
)

// This native streaming owner has no MarshalJSON whole-value method. Its
// marker proves Primitive selects Go's streaming hook rather than buffering
// a fallback projection or serializing private fields as an empty object.
type nativeOwnedStream struct {
	items     int
	calls     *uint64
	cause     error
	malformed bool
}

func (v nativeOwnedStream) Validate() error {
	if v.items < 0 || v.calls == nil {
		return core.ErrNumericOverflow
	}
	return nil
}
func (v nativeOwnedStream) MarshalJSONTo(encoder *jsontext.Encoder) error {
	*v.calls++
	if err := encoder.WriteToken(jsontext.BeginObject); err != nil {
		return err
	}
	if err := encoder.WriteToken(jsontext.String("items")); err != nil {
		return err
	}
	if err := encoder.WriteToken(jsontext.BeginArray); err != nil {
		return err
	}
	for index := range v.items {
		if err := encoder.WriteToken(jsontext.Int(int64(index))); err != nil {
			return err
		}
	}
	if v.cause != nil {
		return v.cause
	}
	if v.malformed {
		return nil
	}
	if err := encoder.WriteToken(jsontext.EndArray); err != nil {
		return err
	}
	return encoder.WriteToken(jsontext.EndObject)
}

type ownedStreamPrefixFailure struct {
	retained bytes.Buffer
	maximum  int
}

func (w *ownedStreamPrefixFailure) Write(data []byte) (int, error) {
	remaining := w.maximum - w.retained.Len()
	if remaining < 0 {
		remaining = 0
	}
	if len(data) > remaining {
		n, err := w.retained.Write(data[:remaining])
		return n, errors.Join(io.ErrClosedPipe, err)
	}
	return w.retained.Write(data)
}

type ownedStreamCancellationSink struct {
	cancel   context.CancelFunc
	retained bytes.Buffer
	writes   uint64
}

func (w *ownedStreamCancellationSink) Write(data []byte) (int, error) {
	w.writes++
	n, err := w.retained.Write(data)
	w.cancel()
	return n, err
}

func TestNativeOwnedStreamingCodecLayerTriadAndArrayEmbedding(t *testing.T) {
	t.Parallel()
	policy := jsonio.OwnedCodecPolicy{Root: jsonio.RootOwnedMethods}
	for _, extent := range []int{0, 1, 1023, 1024, 1025, 32769} {
		t.Run(fmt.Sprintf("native_stream_%d_items", extent), func(t *testing.T) {
			t.Parallel()
			var calls uint64
			document := nativeOwnedStream{items: extent, calls: &calls}
			var output bytes.Buffer
			err := jsonio.WriteValidated(context.Background(), &output, document, policy)
			if err != nil || calls != 1 || bytes.HasSuffix(output.Bytes(), []byte{'\n'}) {
				t.Fatalf("native stream bytes=%d calls=%d error=%v, want one compact native streaming hook", output.Len(), calls, err)
			}
			var decoded struct {
				Items []int64 `json:"items"`
			}
			if err := jsonv2.Unmarshal(output.Bytes(), &decoded); err != nil || len(decoded.Items) != extent {
				t.Fatalf("native stream decode=%d/%v, want %d", len(decoded.Items), err, extent)
			}
			for index, value := range decoded.Items {
				if value != int64(index) {
					t.Fatalf("native stream item%d=%d, want %d", index, value, index)
				}
			}
			var array bytes.Buffer
			encoder, err := jsonio.NewTokenEncoder(jsonio.ObjectDestinationRequest{Destination: &array})
			if err != nil {
				t.Fatal(err)
			}
			if err := encoder.Encode(context.Background(), jsonio.Token{Kind: jsonio.TokenArrayStart}); err != nil {
				t.Fatal(err)
			}
			if err := jsonio.EncodeOwned(context.Background(), encoder, document, policy); err != nil {
				t.Fatalf("embedded native stream=%v", err)
			}
			if err := jsonio.EncodeOwned(context.Background(), encoder, document, policy); err != nil {
				t.Fatalf("second embedded native stream=%v", err)
			}
			if err := encoder.Encode(context.Background(), jsonio.Token{Kind: jsonio.TokenArrayEnd}); err != nil {
				t.Fatal(err)
			}
			var embedded []struct {
				Items []int64 `json:"items"`
			}
			if err := jsonv2.Unmarshal(array.Bytes(), &embedded); err != nil || len(embedded) != 2 || len(embedded[0].Items) != extent || len(embedded[1].Items) != extent || calls != 3 {
				t.Fatalf("array native stream=%d entries/calls%d error%v, want two complete objects/three hooks", len(embedded), calls, err)
			}
		})
	}
}

func TestNativeOwnedStreamingPolicyRefusalsPrecedeEffects(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name      string
		policy    jsonio.OwnedCodecPolicy
		items     int
		cancelled bool
		wantErr   error
	}{
		{"strict root refuses native hook", jsonio.OwnedCodecPolicy{}, 1, false, core.ErrJSONContract},
		{"future root policy refuses before hook", jsonio.OwnedCodecPolicy{Root: jsonio.RootCodecPolicy(255)}, 1, false, core.ErrJSONContract},
		{"future null policy refuses before hook", jsonio.OwnedCodecPolicy{Root: jsonio.RootOwnedMethods, PointerNulls: jsonio.PointerNullPolicy(255)}, 1, false, core.ErrJSONContract},
		{"owner validation refuses before hook", jsonio.OwnedCodecPolicy{Root: jsonio.RootOwnedMethods}, -1, false, core.ErrNumericOverflow},
		{"cancelled lifetime refuses before hook", jsonio.OwnedCodecPolicy{Root: jsonio.RootOwnedMethods}, 1, true, context.Canceled},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if row.cancelled {
				cancel()
			}
			var calls uint64
			document := nativeOwnedStream{items: row.items, calls: &calls}
			var output bytes.Buffer
			err := jsonio.WriteValidated(ctx, &output, document, row.policy)
			encoder, encoderErr := jsonio.NewTokenEncoder(jsonio.ObjectDestinationRequest{Destination: &output})
			if encoderErr != nil {
				t.Fatal(encoderErr)
			}
			arrayErr := jsonio.EncodeOwned(ctx, encoder, document, row.policy)
			if !errors.Is(err, row.wantErr) || !errors.Is(arrayErr, row.wantErr) || calls != 0 || output.Len() != 0 {
				t.Fatalf("stream refusal calls=%d bytes=%d errors%v/%v, want zero effects/%v", calls, output.Len(), err, arrayErr, row.wantErr)
			}
		})
	}
	var calls uint64
	document := nativeOwnedStream{items: 1, calls: &calls}
	err := jsonio.WriteValidated(context.Background(), nil, document, jsonio.OwnedCodecPolicy{Root: jsonio.RootOwnedMethods})
	if !errors.Is(err, core.ErrJSONContract) || calls != 0 {
		t.Fatalf("nil destination=%v/calls%d, want contract/nohook", err, calls)
	}
}

func TestNativeOwnedStreamingFailurePrefixesKeepNativeIdentities(t *testing.T) {
	t.Parallel()
	policy := jsonio.OwnedCodecPolicy{Root: jsonio.RootOwnedMethods}
	for _, maximum := range []int{0, 1, 32767, 32768, 32769} {
		t.Run(fmt.Sprintf("native_writer_refuses_after_%d_bytes", maximum), func(t *testing.T) {
			t.Parallel()
			var calls uint64
			sink := &ownedStreamPrefixFailure{maximum: maximum}
			err := jsonio.WriteValidated(context.Background(), sink, nativeOwnedStream{items: 32769, calls: &calls}, policy)
			if !errors.Is(err, io.ErrClosedPipe) || errors.Is(err, core.ErrJSONContract) || sink.retained.Len() != maximum || calls != 1 {
				t.Fatalf("native writer refusal=%v bytes%d/calls%d, want original closedpipe/exactprefix%d/onehook", err, sink.retained.Len(), calls, maximum)
			}
		})
	}
	for _, row := range []struct {
		name      string
		cause     error
		malformed bool
		wantErr   error
	}{{"owned streaming hook refusal", core.ErrCurrencyOverflow, false, core.ErrCurrencyOverflow}, {"incomplete streaming hook grammar", nil, true, core.ErrJSONContract}} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			var calls uint64
			var output bytes.Buffer
			err := jsonio.WriteValidated(context.Background(), &output, nativeOwnedStream{items: 32769, calls: &calls, cause: row.cause, malformed: row.malformed}, policy)
			if !errors.Is(err, row.wantErr) || calls != 1 || output.Len() == 0 {
				t.Fatalf("stream hook failure=%v bytes%d/calls%d, want %v/provisionalprefix/onehook", err, output.Len(), calls, row.wantErr)
			}
		})
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var calls uint64
	sink := &ownedStreamCancellationSink{cancel: cancel}
	err := jsonio.WriteValidated(ctx, sink, nativeOwnedStream{items: 32769, calls: &calls}, policy)
	if !errors.Is(err, context.Canceled) || errors.Is(err, core.ErrJSONContract) || calls != 1 || sink.retained.Len() == 0 || sink.writes != 1 {
		t.Fatalf("native write cancellation=%v bytes%d/calls%d/writes%d, want cancellation/provisionalprefix/onehook/no second write", err, sink.retained.Len(), calls, sink.writes)
	}
}

func FuzzOwnedNativeStreamingSemanticClosure(f *testing.F) {
	var calls uint64
	seed := nativeOwnedStream{items: 3, calls: &calls}
	if err := seed.Validate(); err != nil {
		f.Fatal(err)
	}
	var output bytes.Buffer
	if err := jsonio.WriteValidated(context.Background(), &output, seed, jsonio.OwnedCodecPolicy{Root: jsonio.RootOwnedMethods}); err != nil {
		f.Fatal(err)
	}
	f.Add(uint16(seed.items), false)
	f.Add(uint16(0), false)
	f.Add(uint16(1025), true)
	f.Fuzz(func(t *testing.T, items uint16, refuse bool) {
		var calls uint64
		document := nativeOwnedStream{items: int(items), calls: &calls}
		if refuse {
			document.cause = io.ErrClosedPipe
		}
		var output bytes.Buffer
		err := jsonio.WriteValidated(context.Background(), &output, document, jsonio.OwnedCodecPolicy{Root: jsonio.RootOwnedMethods})
		if refuse {
			if !errors.Is(err, io.ErrClosedPipe) || calls != 1 {
				t.Fatalf("streaming refusal=%v/calls%d, want cause/onehook", err, calls)
			}
			return
		}
		var decoded struct {
			Items []int64 `json:"items"`
		}
		decodeErr := jsonv2.Unmarshal(output.Bytes(), &decoded)
		if err != nil || decodeErr != nil || len(decoded.Items) != int(items) || calls != 1 || bytes.HasSuffix(output.Bytes(), []byte{'\n'}) {
			t.Fatalf("native stream closure=%d/calls%d errors%v/%v, want %d/onehook/compact", len(decoded.Items), calls, err, decodeErr, items)
		}
		for index, value := range decoded.Items {
			if value != int64(index) {
				t.Fatalf("native streamed item%d=%d, want %d", index, value, index)
			}
		}
	})
}
