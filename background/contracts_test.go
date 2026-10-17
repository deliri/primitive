package background

import (
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestBackgroundContractsLayerTriad(t *testing.T) {
	t.Parallel()
	duration, err := temporal.DurationFromSeconds(600)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name   string
		policy Policy
		want   error
	}{
		{name: "one worker and one waiting callback", policy: Policy{1, 1, duration}},
		{name: "four workers and bounded waiting callbacks", policy: Policy{4, 256, duration}},
		{name: "maximum native cardinalities remain typed", policy: Policy{65535, 65535, duration}},
		{name: "zero workers refuses unexecutable work", policy: Policy{0, 1, duration}, want: core.ErrBackgroundContract},
		{name: "zero waiting memory refuses missing admission window", policy: Policy{1, 0, duration}, want: core.ErrBackgroundContract},
		{name: "zero timeout refuses absent operation lifetime", policy: Policy{1, 1, temporal.Duration{}}, want: core.ErrBackgroundContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.policy.Validate(); !errors.Is(got, tc.want) {
				t.Fatalf("Policy.Validate()=%v,want %v", got, tc.want)
			}
		})
	}
	for _, tc := range []struct {
		name string
		work Work
		want error
	}{
		{name: "both borrowed callbacks", work: Work{Run: func(context.Context) error { return nil }, Complete: func(error) {}}},
		{name: "missing action", work: Work{Complete: func(error) {}}, want: core.ErrBackgroundContract},
		{name: "missing completion", work: Work{Run: func(context.Context) error { return nil }}, want: core.ErrBackgroundContract},
		{name: "no callback capability", want: core.ErrBackgroundContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.work.Validate(); !errors.Is(got, tc.want) {
				t.Fatalf("Work.Validate()=%v,want %v", got, tc.want)
			}
		})
	}
	if got := (OpenRequest{Policy: Policy{1, 1, duration}}).Validate(); !errors.Is(got, core.ErrBackgroundContract) {
		t.Fatalf("nil parent admission=%v,want contract refusal", got)
	}
	if got := (*Executor)(nil).Stop(t.Context()); !errors.Is(got, core.ErrBackgroundContract) {
		t.Fatalf("nil Stop=%v,want contract refusal", got)
	}
	if got := (&Executor{}).Submit(Work{}); !errors.Is(got, core.ErrBackgroundContract) {
		t.Fatalf("unset Submit=%v,want contract refusal", got)
	}
}

func TestBackgroundProductionDataFlowInventory(t *testing.T) {
	t.Parallel()
	// Policy/OpenRequest/Work/TaskRequest/PullScopeRequest: borrowed execution contracts;
	// Executor/Task: native capabilities; TaskResult: completion observation.
	// PullSource: scoped native coroutine loan, with no retained value inventory.
	// PairRequest: source/consumer loans; PairResult: joined observations;
	// pairOffer: one synchronous native handoff, never an input inventory.
	// OnceActionRequest: transferred action; OnceAction/onceAction: shared native
	// sync.Once custody and first observation, with no second product owner.
	// WallGapWatch owns a native Task and fixed maximum; its request lends clock
	// coordinates, and WallGapSnapshot carries one native observation.
	want := []string{"Executor", "OnceAction", "OnceActionRequest", "OpenRequest", "PairRequest", "PairResult", "Policy", "PullScopeRequest", "PullSource", "Task", "TaskRequest", "TaskResult", "WallGapSnapshot", "WallGapWatch", "WallGapWatchRequest", "Work", "onceAction", "pairOffer"}
	var got []string
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), entry.Name(), nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.TypeSpec:
				if _, ok := node.Type.(*ast.StructType); ok {
					got = append(got, node.Name.Name)
				}
			case *ast.MapType, *ast.InterfaceType:
				t.Errorf("production node=%T,want no payload bag", node)
			}
			return true
		})
	}
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Fatalf("production structs=%v,want classified %v", got, want)
	}
}
