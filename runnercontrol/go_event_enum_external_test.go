package runnercontrol_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runnercontrol"
)

// The expected spellings are cmd/go's wire vocabulary, independently declared
// here so a producer and decoder agreeing on the same wrong token cannot pass.
func TestGoEventEnumWireVocabularyAndReceiverPreservation(t *testing.T) {
	t.Parallel()
	actions := []string{"", "start", "run", "pause", "cont", "pass", "bench", "fail", "output", "skip", "build-output", "build-fail", "attr", "artifacts"}
	kinds := []string{"", "frame", "error", "error-continue"}
	for raw := range 256 {
		action := runnercontrol.GoEventAction(raw)
		valid := raw > 0 && raw < len(actions)
		if action.IsValid() != valid || (action.Validate() == nil) != valid {
			t.Fatalf("action %d validity = %t/%v, want %t", raw, action.IsValid(), action.Validate(), valid)
		}
		proveGoEventVocabulary(t, action, valid, actions, raw, (*runnercontrol.GoEventAction).UnmarshalJSON)
		kind := runnercontrol.GoEventOutputKind(raw)
		valid = raw < len(kinds)
		if kind.IsValid() != valid || (kind.Validate() == nil) != valid {
			t.Fatalf("output kind %d validity = %t/%v, want %t", raw, kind.IsValid(), kind.Validate(), valid)
		}
		proveGoEventVocabulary(t, kind, valid, kinds, raw, (*runnercontrol.GoEventOutputKind).UnmarshalJSON)
	}
	for _, data := range [][]byte{nil, []byte(`null`), []byte(`0`), []byte(`{}`), []byte(`"future"`), []byte(`"run" "run"`), []byte(`"\ud800"`), {'"', 0xff, '"'}} {
		action := runnercontrol.GoEventActionRun
		if err := action.UnmarshalJSON(data); !errors.Is(err, core.ErrJSONContract) || action != runnercontrol.GoEventActionRun {
			t.Fatalf("action decode %q = %v/%v, want preserved run/JSON refusal", data, action, err)
		}
		kind := runnercontrol.GoEventOutputFrame
		if err := kind.UnmarshalJSON(data); !errors.Is(err, core.ErrJSONContract) || kind != runnercontrol.GoEventOutputFrame {
			t.Fatalf("output decode %q = %v/%v, want preserved frame/JSON refusal", data, kind, err)
		}
	}
	var action *runnercontrol.GoEventAction
	var kind *runnercontrol.GoEventOutputKind
	if err := action.UnmarshalJSON([]byte(`"run"`)); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("nil action decode = %v, want JSON refusal", err)
	}
	if err := kind.UnmarshalJSON([]byte(`"frame"`)); !errors.Is(err, core.ErrJSONContract) {
		t.Fatalf("nil output decode = %v, want JSON refusal", err)
	}
}

func proveGoEventVocabulary[T enumJSONValue](t *testing.T, value T, valid bool, vocabulary []string, raw int, decode func(*T, []byte) error) {
	t.Helper()
	got, err := value.MarshalJSON()
	if !valid {
		if !errors.Is(err, core.ErrJSONContract) || got != nil {
			t.Fatalf("enum %d marshal = %q/%v, want nil/JSON refusal", raw, got, err)
		}
		return
	}
	want := []byte(`"` + vocabulary[raw] + `"`)
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("enum %d marshal = %q/%v, want %q/nil", raw, got, err, want)
	}
	var received T
	if err := decode(&received, want); err != nil || received != value {
		t.Fatalf("decode wire %q = %v/%v, want %v/nil", want, received, err, value)
	}
}

func TestGoEventFieldRemainsAClosedCallbackDiscriminator(t *testing.T) {
	t.Parallel()
	fields := []string{"", "Action", "Package", "Test", "Output", "OutputType", "Time", "FailedBuild", "Elapsed", "ImportPath", "Key", "Value", "Path"}
	for raw := range 256 {
		field := runnercontrol.GoEventField(raw)
		valid := raw > 0 && raw < len(fields)
		field.OffWireEnum()
		if field.IsValid() != valid || (field.Validate() == nil) != valid {
			t.Fatalf("field %d validity = %t/%v, want %t", raw, field.IsValid(), field.Validate(), valid)
		}
		if valid && field.String() != fields[raw] {
			t.Fatalf("field %d spelling = %q, want %q", raw, field.String(), fields[raw])
		}
	}
}
