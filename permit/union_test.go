package permit

import (
	"errors"
	"fmt"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestActionUnionLayerTriad(t *testing.T) {
	t.Parallel()
	a, b := permitAction(t, "operation-a"), permitAction(t, "operation-b")
	initial, err := NewActions(b)
	if err != nil {
		t.Fatalf("NewActions = %v, want nil", err)
	}
	both, err := NewActions(a, b)
	if err != nil {
		t.Fatalf("NewActions both = %v, want nil", err)
	}
	for _, tc := range []struct {
		name    string
		action  Action
		want    Actions
		wantErr error
	}{
		{name: "new identity is sorted without replacing existing identity", action: a, want: both},
		{name: "zero identity refuses without changing receiver", wantErr: core.ErrPermitContract},
		{name: "repeated identity is an exact no-op", action: b, want: initial},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			before := initial
			got, err := initial.With(tc.action)
			if !errors.Is(err, tc.wantErr) || got != tc.want || initial != before {
				t.Fatalf("With = %v/%v; source = %v, want %v/%v; unchanged %v", got, err, initial, tc.want, tc.wantErr, before)
			}
		})
	}
}

func TestActionUnionCapacityPreservesExistingGrant(t *testing.T) {
	t.Parallel()
	var inputs [ActionsMaximumCount]Action
	for i := range inputs {
		inputs[i] = permitAction(t, fmt.Sprintf("operation-%02d", i))
	}
	full, err := NewActions(inputs[:]...)
	if err != nil {
		t.Fatalf("NewActions full = %v, want nil", err)
	}
	before := full
	got, err := full.With(permitAction(t, "new-operation"))
	if !errors.Is(err, core.ErrPermitContract) || got != (Actions{}) || full != before {
		t.Fatalf("overflow = %v/%v; source = %v, want zero/refusal; unchanged %v", got, err, full, before)
	}
	got, err = full.With(inputs[0])
	if err != nil || got != full {
		t.Fatalf("full-set replay = %v/%v, want %v/nil", got, err, full)
	}
}
