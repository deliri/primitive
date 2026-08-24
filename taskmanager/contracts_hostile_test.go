package taskmanager

import (
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/id"
	"github.com/deliri/primitive/v2026/temporal"
)

func TestListProjectsRequestExhaustsLifecycleLimitAndCursorBoundaryClasses(t *testing.T) {
	t.Parallel()

	type mutation func(*ListProjectsRequest)
	cases := []struct {
		wantErr error
		mutate  mutation
		name    string
	}{
		{name: "active one-item floor without cursor is accepted"},
		{name: "active two-item page is accepted", mutate: func(r *ListProjectsRequest) { r.Limit = 2 }},
		{name: "active one below page ceiling is accepted", mutate: func(r *ListProjectsRequest) { r.Limit = PageLimitMaximum - 1 }},
		{name: "active exact page ceiling is accepted", mutate: func(r *ListProjectsRequest) { r.Limit = PageLimitMaximum }},
		{name: "completed one-item floor without cursor is accepted", mutate: func(r *ListProjectsRequest) { r.Lifecycle = ProjectLifecycleCompleted }},
		{name: "completed exact page ceiling is accepted", mutate: func(r *ListProjectsRequest) { r.Lifecycle = ProjectLifecycleCompleted; r.Limit = PageLimitMaximum }},
		{name: "active cursor ordinary instant is accepted", mutate: func(r *ListProjectsRequest) { r.After = projectCursorFixture(t) }},
		{name: "completed cursor ordinary instant is accepted", mutate: func(r *ListProjectsRequest) {
			r.Lifecycle = ProjectLifecycleCompleted
			r.After = projectCursorFixture(t)
		}},
		{name: "cursor at pre-epoch instant is accepted", mutate: func(r *ListProjectsRequest) { r.After = projectCursorAt(t, -1) }},
		{name: "cursor at maximum instant is accepted", mutate: func(r *ListProjectsRequest) { r.After = projectCursorAt(t, math.MaxInt64) }},
		{name: "zero page limit is rejected", mutate: func(r *ListProjectsRequest) { r.Limit = 0 }, wantErr: core.ErrTaskManagerContract},
		{name: "one above page ceiling is rejected", mutate: func(r *ListProjectsRequest) { r.Limit = PageLimitMaximum + 1 }, wantErr: core.ErrTaskManagerContract},
		{name: "two above page ceiling is rejected", mutate: func(r *ListProjectsRequest) { r.Limit = PageLimitMaximum + 2 }, wantErr: core.ErrTaskManagerContract},
		{name: "maximum page limit is rejected", mutate: func(r *ListProjectsRequest) { r.Limit = ^PageLimit(0) }, wantErr: core.ErrTaskManagerContract},
		{name: "unknown lifecycle is rejected", mutate: func(r *ListProjectsRequest) { r.Lifecycle = ProjectLifecycleUnknown }, wantErr: core.ErrTaskManagerContract},
		{name: "future lifecycle is rejected", mutate: func(r *ListProjectsRequest) { r.Lifecycle = projectLifecycleLimit }, wantErr: core.ErrTaskManagerContract},
		{name: "maximum lifecycle byte is rejected", mutate: func(r *ListProjectsRequest) { r.Lifecycle = ProjectLifecycle(255) }, wantErr: core.ErrTaskManagerContract},
		{name: "cursor without instant is rejected", mutate: func(r *ListProjectsRequest) {
			r.After = projectCursorFixture(t)
			r.After.UpdatedAt = temporal.NumericInstant{}
		}, wantErr: core.ErrTaskManagerContract},
		{name: "cursor without identity is rejected", mutate: func(r *ListProjectsRequest) { r.After = projectCursorFixture(t); r.After.ID = id.UUIDv7{} }, wantErr: core.ErrTaskManagerContract},
		{name: "invalid lifecycle and zero limit retain task-manager identity", mutate: func(r *ListProjectsRequest) { r.Lifecycle = ProjectLifecycleUnknown; r.Limit = 0 }, wantErr: core.ErrTaskManagerContract},
		{name: "invalid lifecycle and invalid cursor retain task-manager identity", mutate: func(r *ListProjectsRequest) { r.Lifecycle = projectLifecycleLimit; r.After = &ProjectCursor{} }, wantErr: core.ErrTaskManagerContract},
		{name: "over-limit page and invalid cursor retain task-manager identity", mutate: func(r *ListProjectsRequest) { r.Limit = PageLimitMaximum + 1; r.After = &ProjectCursor{} }, wantErr: core.ErrTaskManagerContract},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := ListProjectsRequest{Lifecycle: ProjectLifecycleActive, Order: PageOrderDescending, Limit: 1}
			if tc.mutate != nil {
				tc.mutate(&got)
			}
			gotErr := got.Validate()
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("ListProjectsRequest.Validate() error = %v, want %v", gotErr, tc.wantErr)
			}
		})
	}
}

func TestTaskSummaryPressuresEveryIdentityEnumRevisionAndTimeBoundary(t *testing.T) {
	t.Parallel()

	type mutation func(*TaskSummary)
	cases := []struct {
		wantErr error
		mutate  mutation
		name    string
	}{
		{name: "ordinary backlog feature is accepted"},
		{name: "ordinary in-progress bug is accepted", mutate: func(s *TaskSummary) { s.Kind = TaskKindBug; s.State = TaskStateInProgress }},
		{name: "ordinary blocked chore is accepted", mutate: func(s *TaskSummary) { s.Kind = TaskKindChore; s.State = TaskStateBlocked }},
		{name: "completed feature is accepted", mutate: func(s *TaskSummary) { s.State = TaskStateCompleted }},
		{name: "cancelled bug is accepted", mutate: func(s *TaskSummary) { s.Kind = TaskKindBug; s.State = TaskStateCancelled }},
		{name: "single-rune title is accepted", mutate: func(s *TaskSummary) { s.Title = mustTitle(t, "T") }},
		{name: "title one below ceiling is accepted", mutate: func(s *TaskSummary) { s.Title = mustTitle(t, strings.Repeat("t", TitleMaximumRunes-1)) }},
		{name: "title exact ASCII ceiling is accepted", mutate: func(s *TaskSummary) { s.Title = mustTitle(t, strings.Repeat("t", TitleMaximumRunes)) }},
		{name: "title exact Unicode ceiling is accepted", mutate: func(s *TaskSummary) { s.Title = mustTitle(t, strings.Repeat("界", TitleMaximumRunes)) }},
		{name: "maximum revision is accepted", mutate: func(s *TaskSummary) { s.Revision = mustRevision(t, math.MaxUint64) }},
		{name: "pre-epoch updated instant is accepted", mutate: func(s *TaskSummary) { s.UpdatedAt = instantFixture(t, -1) }},
		{name: "maximum updated instant is accepted", mutate: func(s *TaskSummary) { s.UpdatedAt = instantFixture(t, math.MaxInt64) }},
		{name: "unset task identity is rejected", mutate: func(s *TaskSummary) { s.ID = id.UUIDv7{} }, wantErr: core.ErrTaskManagerContract},
		{name: "unset project identity is rejected", mutate: func(s *TaskSummary) { s.ProjectID = id.UUIDv7{} }, wantErr: core.ErrTaskManagerContract},
		{name: "unset phase identity is rejected", mutate: func(s *TaskSummary) { s.PhaseID = id.UUIDv7{} }, wantErr: core.ErrTaskManagerContract},
		{name: "empty title is rejected", mutate: func(s *TaskSummary) { s.Title = Title("") }, wantErr: core.ErrTaskManagerContract},
		{name: "title one above ceiling is rejected", mutate: func(s *TaskSummary) { s.Title = Title(strings.Repeat("t", TitleMaximumRunes+1)) }, wantErr: core.ErrTaskManagerContract},
		{name: "unknown kind is rejected", mutate: func(s *TaskSummary) { s.Kind = TaskKindUnknown }, wantErr: core.ErrTaskManagerContract},
		{name: "future kind is rejected", mutate: func(s *TaskSummary) { s.Kind = taskKindLimit }, wantErr: core.ErrTaskManagerContract},
		{name: "maximum kind byte is rejected", mutate: func(s *TaskSummary) { s.Kind = TaskKind(255) }, wantErr: core.ErrTaskManagerContract},
		{name: "unknown state is rejected", mutate: func(s *TaskSummary) { s.State = TaskStateUnknown }, wantErr: core.ErrTaskManagerContract},
		{name: "future state is rejected", mutate: func(s *TaskSummary) { s.State = taskStateLimit }, wantErr: core.ErrTaskManagerContract},
		{name: "maximum state byte is rejected", mutate: func(s *TaskSummary) { s.State = TaskState(255) }, wantErr: core.ErrTaskManagerContract},
		{name: "zero revision is rejected", mutate: func(s *TaskSummary) { s.Revision = Revision{} }, wantErr: core.ErrTaskManagerContract},
		{name: "unset updated instant is rejected", mutate: func(s *TaskSummary) { s.UpdatedAt = temporal.NumericInstant{} }, wantErr: core.ErrTaskManagerContract},
		{name: "all identities unset are rejected", mutate: func(s *TaskSummary) { s.ID = id.UUIDv7{}; s.ProjectID = id.UUIDv7{}; s.PhaseID = id.UUIDv7{} }, wantErr: core.ErrTaskManagerContract},
		{name: "all enums unknown are rejected", mutate: func(s *TaskSummary) { s.Kind = TaskKindUnknown; s.State = TaskStateUnknown }, wantErr: core.ErrTaskManagerContract},
		{name: "all scalar facts unset are rejected", mutate: func(s *TaskSummary) { s.Title = ""; s.Revision = Revision{}; s.UpdatedAt = temporal.NumericInstant{} }, wantErr: core.ErrTaskManagerContract},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := taskSummaryFixture(t)
			if tc.mutate != nil {
				tc.mutate(&got)
			}
			gotErr := got.Validate()
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("TaskSummary.Validate() error = %v, want %v", gotErr, tc.wantErr)
			}
		})
	}
}

func TestTaskPageEnforcesBoundAndSingleProjectWithoutWorldBuilding(t *testing.T) {
	t.Parallel()

	type mutation func(*TaskPage)
	cases := []struct {
		wantErr error
		mutate  mutation
		name    string
	}{
		{name: "empty terminal page is accepted", mutate: func(p *TaskPage) { p.Items = nil; p.Next = nil }},
		{name: "one-item terminal page is accepted", mutate: func(p *TaskPage) { p.Next = nil }},
		{name: "one-item continued page is accepted"},
		{name: "two-item same-project terminal page is accepted", mutate: func(p *TaskPage) { p.Items = append(p.Items, secondTaskSummaryFixture(t)); p.Next = nil }},
		{name: "exact-ceiling terminal page is accepted", mutate: func(p *TaskPage) { p.Items = repeatedTaskSummaries(t, int(PageLimitMaximum)); p.Next = nil }},
		{name: "exact-ceiling continued page is accepted", mutate: func(p *TaskPage) { p.Items = repeatedTaskSummaries(t, int(PageLimitMaximum)) }},
		{name: "one-above-ceiling page is rejected", mutate: func(p *TaskPage) { p.Items = repeatedTaskSummaries(t, int(PageLimitMaximum)+1) }, wantErr: core.ErrTaskManagerContract},
		{name: "two-above-ceiling page is rejected", mutate: func(p *TaskPage) { p.Items = repeatedTaskSummaries(t, int(PageLimitMaximum)+2) }, wantErr: core.ErrTaskManagerContract},
		{name: "empty page with cursor is rejected", mutate: func(p *TaskPage) { p.Items = nil }, wantErr: core.ErrTaskManagerContract},
		{name: "foreign second item is rejected", mutate: func(p *TaskPage) {
			foreign := secondTaskSummaryFixture(t)
			foreign.ProjectID = uuidFixture(t, "019ff548-29cb-7451-869e-aa644c0947e7")
			p.Items = append(p.Items, foreign)
		}, wantErr: core.ErrTaskManagerContract},
		{name: "foreign continuation cursor is rejected", mutate: func(p *TaskPage) { p.Next.ProjectID = uuidFixture(t, "019ff548-29cb-7451-869e-aa644c0947e7") }, wantErr: core.ErrTaskManagerContract},
		{name: "cursor without project is rejected", mutate: func(p *TaskPage) { p.Next.ProjectID = id.UUIDv7{} }, wantErr: core.ErrTaskManagerContract},
		{name: "cursor without item identity is rejected", mutate: func(p *TaskPage) { p.Next.ID = id.UUIDv7{} }, wantErr: core.ErrTaskManagerContract},
		{name: "cursor without updated instant is rejected", mutate: func(p *TaskPage) { p.Next.UpdatedAt = temporal.NumericInstant{} }, wantErr: core.ErrTaskManagerContract},
		{name: "first item without project is rejected", mutate: func(p *TaskPage) { p.Items[0].ProjectID = id.UUIDv7{} }, wantErr: core.ErrTaskManagerContract},
		{name: "first item without task identity is rejected", mutate: func(p *TaskPage) { p.Items[0].ID = id.UUIDv7{} }, wantErr: core.ErrTaskManagerContract},
		{name: "first item without phase identity is rejected", mutate: func(p *TaskPage) { p.Items[0].PhaseID = id.UUIDv7{} }, wantErr: core.ErrTaskManagerContract},
		{name: "invalid item and foreign cursor retain task-manager identity", mutate: func(p *TaskPage) {
			p.Items[0].State = TaskStateUnknown
			p.Next.ProjectID = uuidFixture(t, "019ff548-29cb-7451-869e-aa644c0947e7")
		}, wantErr: core.ErrTaskManagerContract},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := TaskPage{
				ProjectID:  uuidFixture(t, "019ff548-29cb-7451-869e-aa644c0947e6"),
				Collection: TaskCollectionActive,
				Order:      PageOrderDescending,
				Items:      []TaskSummary{taskSummaryFixture(t)}, Next: taskCursorFixture(t),
			}
			if tc.mutate != nil {
				tc.mutate(&got)
			}
			gotErr := got.Validate()
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("TaskPage.Validate() error = %v, want %v", gotErr, tc.wantErr)
			}
		})
	}
}

func TestUpdateTaskRequestRequiresOneTypedChangeAndBindsMutationIdentity(t *testing.T) {
	t.Parallel()

	type mutation func(*UpdateTaskRequest)
	cases := []struct {
		wantErr error
		mutate  mutation
		name    string
	}{
		{name: "title-only update is accepted", mutate: func(r *UpdateTaskRequest) { r.Change = TaskChange{Title: titlePointer(t, "New title")} }},
		{name: "description-only update is accepted", mutate: func(r *UpdateTaskRequest) {
			r.Change = TaskChange{Description: descriptionPointer(t, "New description")}
		}},
		{name: "empty description clears description and is accepted", mutate: func(r *UpdateTaskRequest) { r.Change = TaskChange{Description: descriptionPointer(t, "")} }},
		{name: "kind-only feature update is accepted", mutate: func(r *UpdateTaskRequest) { kind := TaskKindFeature; r.Change = TaskChange{Kind: &kind} }},
		{name: "kind-only bug update is accepted", mutate: func(r *UpdateTaskRequest) { kind := TaskKindBug; r.Change = TaskChange{Kind: &kind} }},
		{name: "kind-only chore update is accepted", mutate: func(r *UpdateTaskRequest) { kind := TaskKindChore; r.Change = TaskChange{Kind: &kind} }},
		{name: "state-only backlog update is accepted", mutate: func(r *UpdateTaskRequest) { state := TaskStateBacklog; r.Change = TaskChange{State: &state} }},
		{name: "state-only in-progress update is accepted", mutate: func(r *UpdateTaskRequest) { state := TaskStateInProgress; r.Change = TaskChange{State: &state} }},
		{name: "state-only blocked update is accepted", mutate: func(r *UpdateTaskRequest) { state := TaskStateBlocked; r.Change = TaskChange{State: &state} }},
		{name: "state-only completed update is accepted", mutate: func(r *UpdateTaskRequest) { state := TaskStateCompleted; r.Change = TaskChange{State: &state} }},
		{name: "state-only cancelled update is accepted", mutate: func(r *UpdateTaskRequest) { state := TaskStateCancelled; r.Change = TaskChange{State: &state} }},
		{name: "all change fields present are accepted"},
		{name: "maximum entity revision is accepted", mutate: func(r *UpdateTaskRequest) { r.ExpectedRevision = mustRevision(t, math.MaxUint64) }},
		{name: "zero change is rejected", mutate: func(r *UpdateTaskRequest) { r.Change = TaskChange{} }, wantErr: core.ErrTaskManagerContract},
		{name: "unset project identity is rejected", mutate: func(r *UpdateTaskRequest) { r.ProjectID = id.UUIDv7{} }, wantErr: core.ErrTaskManagerContract},
		{name: "unset task identity is rejected", mutate: func(r *UpdateTaskRequest) { r.TaskID = id.UUIDv7{} }, wantErr: core.ErrTaskManagerContract},
		{name: "unset mutation identity is rejected", mutate: func(r *UpdateTaskRequest) { r.MutationID = id.UUIDv7{} }, wantErr: core.ErrTaskManagerContract},
		{name: "zero entity revision is rejected", mutate: func(r *UpdateTaskRequest) { r.ExpectedRevision = Revision{} }, wantErr: core.ErrTaskManagerContract},
		{name: "empty title is rejected", mutate: func(r *UpdateTaskRequest) { title := Title(""); r.Change = TaskChange{Title: &title} }, wantErr: core.ErrTaskManagerContract},
		{name: "title above ceiling is rejected", mutate: func(r *UpdateTaskRequest) {
			title := Title(strings.Repeat("t", TitleMaximumRunes+1))
			r.Change = TaskChange{Title: &title}
		}, wantErr: core.ErrTaskManagerContract},
		{name: "description above ceiling is rejected", mutate: func(r *UpdateTaskRequest) {
			description := Description(strings.Repeat("d", DescriptionMaximumRunes+1))
			r.Change = TaskChange{Description: &description}
		}, wantErr: core.ErrTaskManagerContract},
		{name: "unknown kind is rejected", mutate: func(r *UpdateTaskRequest) { kind := TaskKindUnknown; r.Change = TaskChange{Kind: &kind} }, wantErr: core.ErrTaskManagerContract},
		{name: "future kind is rejected", mutate: func(r *UpdateTaskRequest) { kind := taskKindLimit; r.Change = TaskChange{Kind: &kind} }, wantErr: core.ErrTaskManagerContract},
		{name: "unknown state is rejected", mutate: func(r *UpdateTaskRequest) { state := TaskStateUnknown; r.Change = TaskChange{State: &state} }, wantErr: core.ErrTaskManagerContract},
		{name: "future state is rejected", mutate: func(r *UpdateTaskRequest) { state := taskStateLimit; r.Change = TaskChange{State: &state} }, wantErr: core.ErrTaskManagerContract},
		{name: "all request identities unset are rejected", mutate: func(r *UpdateTaskRequest) {
			r.ProjectID = id.UUIDv7{}
			r.TaskID = id.UUIDv7{}
			r.MutationID = id.UUIDv7{}
		}, wantErr: core.ErrTaskManagerContract},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got := updateTaskRequestFixture(t)
			if tc.mutate != nil {
				tc.mutate(&got)
			}
			gotErr := got.Validate()
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("UpdateTaskRequest.Validate() error = %v, want %v", gotErr, tc.wantErr)
			}
			if tc.wantErr != nil {
				return
			}
			key, keyErr := got.IdempotencyKey()
			if keyErr != nil || key.String() != got.MutationID.String() {
				t.Fatalf("UpdateTaskRequest.IdempotencyKey() = (%q, %v), want (%q, nil)", key.String(), keyErr, got.MutationID.String())
			}
		})
	}
}

func updateTaskRequestFixture(t testing.TB) UpdateTaskRequest {
	t.Helper()
	kind := TaskKindFeature
	state := TaskStateInProgress
	return UpdateTaskRequest{
		ProjectID:        uuidFixture(t, "019ff548-29cb-7451-869e-aa644c0947e6"),
		TaskID:           uuidFixture(t, "019ff548-346e-77cc-be1e-be78ab803328"),
		MutationID:       uuidFixture(t, "019ff548-346e-77cc-be1e-be78ab803329"),
		ExpectedRevision: mustRevision(t, 7),
		Change: TaskChange{
			Title:       titlePointer(t, "Bounded task-manager socket"),
			Description: descriptionPointer(t, "Keep storage and application policy consumer-owned."),
			Kind:        &kind,
			State:       &state,
		},
	}
}

func taskSummaryFixture(t testing.TB) TaskSummary {
	t.Helper()
	return TaskSummary{
		ID:        uuidFixture(t, "019ff548-346e-77cc-be1e-be78ab803328"),
		ProjectID: uuidFixture(t, "019ff548-29cb-7451-869e-aa644c0947e6"),
		PhaseID:   uuidFixture(t, "019ff548-346e-77cc-be1e-be78ab803327"),
		Title:     mustTitle(t, "Build the typed task-manager socket"),
		Kind:      TaskKindFeature,
		State:     TaskStateBacklog,
		Revision:  mustRevision(t, 1),
		UpdatedAt: instantFixture(t, 1_777_000_000_000_000_000),
	}
}

func secondTaskSummaryFixture(t testing.TB) TaskSummary {
	t.Helper()
	got := taskSummaryFixture(t)
	got.ID = uuidFixture(t, "019ff548-346e-77cc-be1e-be78ab80332a")
	got.Title = mustTitle(t, "Second bounded task")
	return got
}

func repeatedTaskSummaries(t testing.TB, count int) []TaskSummary {
	t.Helper()
	items := make([]TaskSummary, count)
	for index := range items {
		items[index] = taskSummaryFixture(t)
	}
	return items
}

func projectCursorFixture(t testing.TB) *ProjectCursor {
	t.Helper()
	return projectCursorAt(t, 1_777_000_000_000_000_000)
}

func projectCursorAt(t testing.TB, nanoseconds int64) *ProjectCursor {
	t.Helper()
	return &ProjectCursor{UpdatedAt: instantFixture(t, nanoseconds), ID: uuidFixture(t, "019ff548-29cb-7451-869e-aa644c0947e6")}
}

func taskCursorFixture(t testing.TB) *TaskCursor {
	t.Helper()
	return &TaskCursor{
		ProjectID: uuidFixture(t, "019ff548-29cb-7451-869e-aa644c0947e6"),
		UpdatedAt: instantFixture(t, 1_777_000_000_000_000_000),
		ID:        uuidFixture(t, "019ff548-346e-77cc-be1e-be78ab803328"),
	}
}

func uuidFixture(t testing.TB, value string) id.UUIDv7 {
	t.Helper()
	parsed, err := id.ParseUUIDv7(value)
	if err != nil {
		t.Fatalf("id.ParseUUIDv7(%q) error = %v, want nil", value, err)
	}
	return parsed
}

func instantFixture(t testing.TB, nanoseconds int64) temporal.NumericInstant {
	t.Helper()
	value, err := temporal.NewNumericInstant(temporal.InstantFromNanoseconds(nanoseconds))
	if err != nil {
		t.Fatalf("temporal.NewNumericInstant() error = %v, want nil", err)
	}
	return value
}

func mustTitle(t testing.TB, value string) Title {
	t.Helper()
	parsed, err := ParseTitle(value)
	if err != nil {
		t.Fatalf("ParseTitle(%q) error = %v, want nil", value, err)
	}
	return parsed
}

func titlePointer(t testing.TB, value string) *Title {
	t.Helper()
	parsed := mustTitle(t, value)
	return &parsed
}

func descriptionPointer(t testing.TB, value string) *Description {
	t.Helper()
	parsed, err := ParseDescription(value)
	if err != nil {
		t.Fatalf("ParseDescription(%q) error = %v, want nil", value, err)
	}
	return &parsed
}
