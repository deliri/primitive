package jsonio_test

import (
	"context"
	"encoding/json/jsontext"
	"errors"
	"io"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/jsonio"
	"github.com/deliri/primitive/v2026/temporal"
	"github.com/deliri/primitive/v2026/testserial"
)

func TestValidateDocumentRefusesGrammarAndMultipleValues(t *testing.T) {
	t.Parallel()
	for _, value := range []string{`null`, `true`, `-42.5`, `"é"`, `{"items":[1,{},null]}`, " \n [] \t "} {
		if err := jsonio.ValidateDocument(t.Context(), iotest.OneByteReader(strings.NewReader(value))); err != nil {
			t.Fatalf("valid document %q = %v", value, err)
		}
	}
	for _, value := range []string{"", " \n ", `[}`, `{"a":1,"a":2}`, `"` + string([]byte{0xff}) + `"`, `null true`, `[] {}`, `[] garbage`, `{"open":`} {
		if err := jsonio.ValidateDocument(t.Context(), iotest.OneByteReader(strings.NewReader(value))); !errors.Is(err, core.ErrJSONContract) {
			t.Fatalf("invalid document %q = %v, want typed refusal", value, err)
		}
	}
}

func TestValidateDocumentPreservesSourceAndCancellation(t *testing.T) {
	t.Parallel()
	if err := jsonio.ValidateDocument(nil, strings.NewReader(`null`)); !errors.Is(err, core.ErrNilContext) {
		t.Fatal(err)
	}
	var absent *strings.Reader
	if err := jsonio.ValidateDocument(t.Context(), absent); !errors.Is(err, core.ErrJSONContract) {
		t.Fatal(err)
	}
	for _, value := range []string{`null`, `{"a":`} {
		source := io.MultiReader(strings.NewReader(value), iotest.ErrReader(core.ErrHostFactsObservation))
		if err := jsonio.ValidateDocument(t.Context(), source); !errors.Is(err, core.ErrHostFactsObservation) || !errors.Is(err, core.ErrJSONContract) {
			t.Fatalf("source failure after %q = %v, want both identities", value, err)
		}
	}
	ctx, cancel, err := temporal.WithCancellation(temporal.CancellationRequest{Parent: t.Context()})
	if err != nil {
		t.Fatal(err)
	}
	defer cancel(nil)
	cancel(nil)
	if err := jsonio.ValidateDocument(ctx, strings.NewReader(`null`)); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	for _, value := range []string{"", `null`, `[]`} {
		reading, stop, err := temporal.WithCancellation(temporal.CancellationRequest{Parent: t.Context()})
		if err != nil {
			t.Fatal(err)
		}
		err = jsonio.ValidateDocument(reading, tokenCancelReader{source: strings.NewReader(value), cancel: stop})
		stop(nil)
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("cancellation during read of %q = %v, want cancellation", value, err)
		}
	}
}

func TestValidateDocumentStreamsAcrossWorkingWindows(t *testing.T) {
	t.Parallel()
	for _, items := range []int{4096, 65536, 262144} {
		repeated := &repeatedJSONStringReader{remaining: items - 1}
		source := io.MultiReader(strings.NewReader("["), repeated, strings.NewReader(`"value"]`))
		if err := jsonio.ValidateDocument(t.Context(), source); err != nil || repeated.remaining != 0 {
			t.Fatalf("items=%d validation=%v unread=%d, want complete streamed document", items, err, repeated.remaining)
		}
	}
}

func TestValidateDocumentAllocatedMemoryDoesNotScaleWithExtent(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	const allowance = 256 << 10
	for _, items := range []int{4096, 65536, 262144} {
		repeated := &repeatedJSONStringReader{remaining: items - 1}
		source := io.MultiReader(strings.NewReader("["), repeated, strings.NewReader(`"value"]`))
		before, err := hostfacts.ObserveGoAllocationTotal(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if err := jsonio.ValidateDocument(t.Context(), source); err != nil {
			t.Fatal(err)
		}
		after, err := hostfacts.ObserveGoAllocationTotal(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		allocated := after.Uint64() - before.Uint64()
		t.Logf("items=%d allocated_bytes=%d allowance=%d", items, allocated, allowance)
		if allocated > allowance {
			t.Fatalf("allocated bytes = %d, want bounded working storage <= %d", allocated, allowance)
		}
	}
}

func FuzzValidateDocumentAgainstGo(f *testing.F) {
	for _, value := range []string{`null`, `{"a":[1,"é"]}`, "", `{}[]`, `{"a":1,"a":2}`, `"` + string([]byte{0xff}) + `"`} {
		f.Add(value)
	}
	f.Fuzz(func(t *testing.T, value string) {
		decoder := jsontext.NewDecoder(strings.NewReader(value))
		valid := decoder.SkipValue() == nil && decoder.SkipValue() == io.EOF
		err := jsonio.ValidateDocument(t.Context(), iotest.OneByteReader(strings.NewReader(value)))
		if valid && err != nil {
			t.Fatalf("Go accepted document, Primitive refused: %v", err)
		}
		if !valid && !errors.Is(err, core.ErrJSONContract) {
			t.Fatalf("Go refused document, Primitive returned: %v", err)
		}
	})
}
