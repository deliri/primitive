package taskmanager

import (
	"bytes"
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func FuzzTitleAndDescriptionParsersSemanticClosure(f *testing.F) {
	f.Add(uint8(textBoundaryTitle), "Task manager")
	f.Add(uint8(textBoundaryDescription), "Bounded pages")
	f.Add(uint8(textBoundaryDescription), "")
	f.Add(uint8(textBoundaryTitle), "")
	f.Add(uint8(textBoundaryTitle), " Task")
	f.Add(uint8(textBoundaryDescription), "Description\nnext")

	f.Fuzz(func(t *testing.T, rawKind uint8, value string) {
		kind := textBoundaryKind(rawKind%2 + 1)
		got, gotErr := parseTextBoundary(textBoundaryCase{value: value, kind: kind})
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrTaskManagerContract) || got != "" {
				t.Fatalf("text parser rejected result = (%q, %v), want zero and %v", got, gotErr, core.ErrTaskManagerContract)
			}
			return
		}
		if got != value {
			t.Fatalf("text parser accepted result = %q, want exact %q", got, value)
		}
	})
}

func FuzzRevisionJSONSemanticClosure(f *testing.F) {
	for _, value := range []uint64{1, 2, 42, 1<<63 - 1, 1 << 63, ^uint64(0)} {
		revision := mustRevision(f, value)
		encoded, err := revision.MarshalJSON()
		if err != nil {
			f.Fatalf("Revision.MarshalJSON(seed) error = %v, want nil", err)
		}
		f.Add(encoded)
	}
	f.Add([]byte{})
	f.Add([]byte("0"))
	f.Add([]byte("01"))
	f.Add([]byte("null"))

	f.Fuzz(func(t *testing.T, data []byte) {
		got := mustRevision(t, 99)
		before := got
		gotErr := got.UnmarshalJSON(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrJSONContract) || got != before {
				t.Fatalf("Revision.UnmarshalJSON(rejected) = (%v, %v), want receiver %v and %v", got, gotErr, before, core.ErrJSONContract)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("Revision.UnmarshalJSON(accepted).Validate() error = %v, want nil", err)
		}
		encoded, err := got.MarshalJSON()
		if err != nil {
			t.Fatalf("Revision.MarshalJSON(accepted) error = %v, want nil", err)
		}
		var roundTrip Revision
		if err := roundTrip.UnmarshalJSON(encoded); err != nil || roundTrip != got {
			t.Fatalf("Revision canonical round trip = (%v, %v), want (%v, nil)", roundTrip, err, got)
		}
		second, err := roundTrip.MarshalJSON()
		if err != nil || !bytes.Equal(second, encoded) {
			t.Fatalf("Revision second canonical projection = (%q, %v), want (%q, nil)", second, err, encoded)
		}
	})
}

func FuzzPageLimitJSONSemanticClosure(f *testing.F) {
	for _, limit := range []PageLimit{1, 2, PageLimitMaximum - 1, PageLimitMaximum} {
		encoded, err := limit.MarshalJSON()
		if err != nil {
			f.Fatalf("PageLimit.MarshalJSON(seed) error = %v, want nil", err)
		}
		f.Add(encoded)
	}
	f.Add([]byte{})
	f.Add([]byte("0"))
	f.Add([]byte("01"))
	f.Add([]byte("101"))
	f.Add([]byte("null"))

	f.Fuzz(func(t *testing.T, data []byte) {
		got := PageLimit(17)
		before := got
		gotErr := got.UnmarshalJSON(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrJSONContract) || got != before {
				t.Fatalf("PageLimit.UnmarshalJSON(rejected) = (%v, %v), want receiver %v and %v", got, gotErr, before, core.ErrJSONContract)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("PageLimit.UnmarshalJSON(accepted).Validate() error = %v, want nil", err)
		}
		encoded, err := got.MarshalJSON()
		if err != nil {
			t.Fatalf("PageLimit.MarshalJSON(accepted) error = %v, want nil", err)
		}
		var roundTrip PageLimit
		if err := roundTrip.UnmarshalJSON(encoded); err != nil || roundTrip != got {
			t.Fatalf("PageLimit canonical round trip = (%v, %v), want (%v, nil)", roundTrip, err, got)
		}
		second, err := roundTrip.MarshalJSON()
		if err != nil || !bytes.Equal(second, encoded) {
			t.Fatalf("PageLimit second canonical projection = (%q, %v), want (%q, nil)", second, err, encoded)
		}
	})
}

func FuzzEnumJSONSemanticClosure(f *testing.F) {
	for _, lifecycle := range []ProjectLifecycle{ProjectLifecycleActive, ProjectLifecycleCompleted} {
		encoded, err := lifecycle.MarshalJSON()
		if err != nil {
			f.Fatalf("ProjectLifecycle.MarshalJSON(seed) error = %v, want nil", err)
		}
		f.Add(uint8(0), encoded)
	}
	for _, kind := range []TaskKind{TaskKindFeature, TaskKindBug, TaskKindChore} {
		encoded, err := kind.MarshalJSON()
		if err != nil {
			f.Fatalf("TaskKind.MarshalJSON(seed) error = %v, want nil", err)
		}
		f.Add(uint8(1), encoded)
	}
	for _, state := range []TaskState{TaskStateBacklog, TaskStateInProgress, TaskStateBlocked, TaskStateCompleted, TaskStateCancelled} {
		encoded, err := state.MarshalJSON()
		if err != nil {
			f.Fatalf("TaskState.MarshalJSON(seed) error = %v, want nil", err)
		}
		f.Add(uint8(2), encoded)
	}
	for _, collection := range []TaskCollection{TaskCollectionActive, TaskCollectionCompleted} {
		encoded, err := collection.MarshalJSON()
		if err != nil {
			f.Fatalf("TaskCollection.MarshalJSON(seed) error = %v, want nil", err)
		}
		f.Add(uint8(3), encoded)
	}
	for _, kind := range []EvidenceKind{
		EvidenceKindNote, EvidenceKindTest, EvidenceKindBenchmark, EvidenceKindScreenshot,
		EvidenceKindGitInventory, EvidenceKindArtifact,
	} {
		encoded, err := kind.MarshalJSON()
		if err != nil {
			f.Fatalf("EvidenceKind.MarshalJSON(seed) error = %v, want nil", err)
		}
		f.Add(uint8(4), encoded)
	}
	for _, order := range []PageOrder{PageOrderAscending, PageOrderDescending} {
		encoded, err := order.MarshalJSON()
		if err != nil {
			f.Fatalf("PageOrder.MarshalJSON(seed) error = %v, want nil", err)
		}
		f.Add(uint8(5), encoded)
	}
	for _, malformed := range [][]byte{nil, []byte(`""`), []byte(`"future"`), []byte("null"), []byte("1"), []byte("{}")} {
		f.Add(uint8(0), malformed)
		f.Add(uint8(1), malformed)
		f.Add(uint8(2), malformed)
		f.Add(uint8(3), malformed)
		f.Add(uint8(4), malformed)
		f.Add(uint8(5), malformed)
	}

	f.Fuzz(func(t *testing.T, rawKind uint8, data []byte) {
		switch rawKind % 6 {
		case 0:
			got := ProjectLifecycleActive
			before := got
			gotErr := got.UnmarshalJSON(data)
			proveLifecycleJSONClosure(t, got, before, gotErr)
		case 1:
			got := TaskKindFeature
			before := got
			gotErr := got.UnmarshalJSON(data)
			proveTaskKindJSONClosure(t, got, before, gotErr)
		case 2:
			got := TaskStateBacklog
			before := got
			gotErr := got.UnmarshalJSON(data)
			proveTaskStateJSONClosure(t, got, before, gotErr)
		case 3:
			got := TaskCollectionActive
			before := got
			gotErr := got.UnmarshalJSON(data)
			proveTaskCollectionJSONClosure(t, got, before, gotErr)
		case 4:
			got := EvidenceKindNote
			before := got
			gotErr := got.UnmarshalJSON(data)
			proveEvidenceKindJSONClosure(t, got, before, gotErr)
		case 5:
			got := PageOrderAscending
			before := got
			gotErr := got.UnmarshalJSON(data)
			provePageOrderJSONClosure(t, got, before, gotErr)
		default:
			t.Fatalf("enum selector %d escaped modulo domain", rawKind)
		}
	})
}

func provePageOrderJSONClosure(t *testing.T, got, before PageOrder, gotErr error) {
	t.Helper()
	if gotErr != nil {
		if !errors.Is(gotErr, core.ErrJSONContract) || got != before {
			t.Fatalf("PageOrder.UnmarshalJSON(rejected) = (%v, %v), want (%v, %v)", got, gotErr, before, core.ErrJSONContract)
		}
		return
	}
	encoded, err := got.MarshalJSON()
	if err != nil || got.Validate() != nil {
		t.Fatalf("PageOrder accepted closure = (%q, %v), want validated canonical bytes", encoded, err)
	}
	var roundTrip PageOrder
	if err := roundTrip.UnmarshalJSON(encoded); err != nil || roundTrip != got {
		t.Fatalf("PageOrder canonical round trip = (%v, %v), want (%v, nil)", roundTrip, err, got)
	}
}

func proveTaskCollectionJSONClosure(t *testing.T, got, before TaskCollection, gotErr error) {
	t.Helper()
	if gotErr != nil {
		if !errors.Is(gotErr, core.ErrJSONContract) || got != before {
			t.Fatalf("TaskCollection.UnmarshalJSON(rejected) = (%v, %v), want (%v, %v)", got, gotErr, before, core.ErrJSONContract)
		}
		return
	}
	encoded, err := got.MarshalJSON()
	if err != nil || got.Validate() != nil {
		t.Fatalf("TaskCollection accepted closure = (%q, %v), want validated canonical bytes", encoded, err)
	}
	var roundTrip TaskCollection
	if err := roundTrip.UnmarshalJSON(encoded); err != nil || roundTrip != got {
		t.Fatalf("TaskCollection canonical round trip = (%v, %v), want (%v, nil)", roundTrip, err, got)
	}
}

func proveEvidenceKindJSONClosure(t *testing.T, got, before EvidenceKind, gotErr error) {
	t.Helper()
	if gotErr != nil {
		if !errors.Is(gotErr, core.ErrJSONContract) || got != before {
			t.Fatalf("EvidenceKind.UnmarshalJSON(rejected) = (%v, %v), want (%v, %v)", got, gotErr, before, core.ErrJSONContract)
		}
		return
	}
	encoded, err := got.MarshalJSON()
	if err != nil || got.Validate() != nil {
		t.Fatalf("EvidenceKind accepted closure = (%q, %v), want validated canonical bytes", encoded, err)
	}
	var roundTrip EvidenceKind
	if err := roundTrip.UnmarshalJSON(encoded); err != nil || roundTrip != got {
		t.Fatalf("EvidenceKind canonical round trip = (%v, %v), want (%v, nil)", roundTrip, err, got)
	}
}

func proveLifecycleJSONClosure(t *testing.T, got, before ProjectLifecycle, gotErr error) {
	t.Helper()
	if gotErr != nil {
		if !errors.Is(gotErr, core.ErrJSONContract) || got != before {
			t.Fatalf("ProjectLifecycle.UnmarshalJSON(rejected) = (%v, %v), want (%v, %v)", got, gotErr, before, core.ErrJSONContract)
		}
		return
	}
	encoded, err := got.MarshalJSON()
	if err != nil || got.Validate() != nil {
		t.Fatalf("ProjectLifecycle accepted closure = (%q, %v), want validated canonical bytes", encoded, err)
	}
	var roundTrip ProjectLifecycle
	if err := roundTrip.UnmarshalJSON(encoded); err != nil || roundTrip != got {
		t.Fatalf("ProjectLifecycle canonical round trip = (%v, %v), want (%v, nil)", roundTrip, err, got)
	}
}

func proveTaskKindJSONClosure(t *testing.T, got, before TaskKind, gotErr error) {
	t.Helper()
	if gotErr != nil {
		if !errors.Is(gotErr, core.ErrJSONContract) || got != before {
			t.Fatalf("TaskKind.UnmarshalJSON(rejected) = (%v, %v), want (%v, %v)", got, gotErr, before, core.ErrJSONContract)
		}
		return
	}
	encoded, err := got.MarshalJSON()
	if err != nil || got.Validate() != nil {
		t.Fatalf("TaskKind accepted closure = (%q, %v), want validated canonical bytes", encoded, err)
	}
	var roundTrip TaskKind
	if err := roundTrip.UnmarshalJSON(encoded); err != nil || roundTrip != got {
		t.Fatalf("TaskKind canonical round trip = (%v, %v), want (%v, nil)", roundTrip, err, got)
	}
}

func proveTaskStateJSONClosure(t *testing.T, got, before TaskState, gotErr error) {
	t.Helper()
	if gotErr != nil {
		if !errors.Is(gotErr, core.ErrJSONContract) || got != before {
			t.Fatalf("TaskState.UnmarshalJSON(rejected) = (%v, %v), want (%v, %v)", got, gotErr, before, core.ErrJSONContract)
		}
		return
	}
	encoded, err := got.MarshalJSON()
	if err != nil || got.Validate() != nil {
		t.Fatalf("TaskState accepted closure = (%q, %v), want validated canonical bytes", encoded, err)
	}
	var roundTrip TaskState
	if err := roundTrip.UnmarshalJSON(encoded); err != nil || roundTrip != got {
		t.Fatalf("TaskState canonical round trip = (%v, %v), want (%v, nil)", roundTrip, err, got)
	}
}

func FuzzUpdateTaskRequestJSONSemanticClosure(f *testing.F) {
	request := updateTaskRequestFixture(f)
	canonical, err := request.MarshalJSON()
	if err != nil {
		f.Fatalf("UpdateTaskRequest.MarshalJSON(seed) error = %v, want nil", err)
	}
	f.Add(canonical)
	f.Add([]byte{})
	f.Add([]byte("{}"))
	f.Add([]byte("null"))
	f.Add([]byte(`{"project_id":null}`))

	f.Fuzz(func(t *testing.T, data []byte) {
		got := request
		gotErr := got.UnmarshalJSON(data)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrJSONContract) || !updateTaskRequestsEqual(got, request) {
				t.Fatalf("UpdateTaskRequest.UnmarshalJSON(rejected) = (%+v, %v), want receiver preserved and %v", got, gotErr, core.ErrJSONContract)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("UpdateTaskRequest.UnmarshalJSON(accepted).Validate() error = %v, want nil", err)
		}
		encoded, err := got.MarshalJSON()
		if err != nil {
			t.Fatalf("UpdateTaskRequest.MarshalJSON(accepted) error = %v, want nil", err)
		}
		var roundTrip UpdateTaskRequest
		if err := roundTrip.UnmarshalJSON(encoded); err != nil || !updateTaskRequestsEqual(roundTrip, got) {
			t.Fatalf("UpdateTaskRequest canonical round trip = (%+v, %v), want (%+v, nil)", roundTrip, err, got)
		}
		second, err := roundTrip.MarshalJSON()
		if err != nil || !bytes.Equal(second, encoded) {
			t.Fatalf("UpdateTaskRequest second canonical projection = (%q, %v), want (%q, nil)", second, err, encoded)
		}
	})
}

func updateTaskRequestsEqual(left, right UpdateTaskRequest) bool {
	leftJSON, leftErr := left.MarshalJSON()
	rightJSON, rightErr := right.MarshalJSON()
	return leftErr == nil && rightErr == nil && bytes.Equal(leftJSON, rightJSON)
}
