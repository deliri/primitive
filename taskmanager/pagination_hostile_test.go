package taskmanager

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/id"
	"github.com/deliri/primitive/v2026/temporal"
)

type pageValidationCase struct {
	wantErr  error
	validate func(*testing.T) error
	name     string
}

type requestValidationObservation struct {
	Unchanged bool
}

type requestValidationCase struct {
	wantErr  error
	exercise func(*testing.T) (requestValidationObservation, error)
	name     string
	want     requestValidationObservation
}

func TestEveryPagedRequestEnforcesOrderBoundAndCursorOwnershipWithoutMutation(t *testing.T) {
	t.Parallel()
	for _, tc := range requestValidationCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := tc.exercise(t)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("paged request Validate() error = %v, want %v", gotErr, tc.wantErr)
			}
			if got != tc.want {
				t.Fatalf("paged request Validate() observation = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func requestValidationCases() []requestValidationCase {
	wantUnchanged := requestValidationObservation{Unchanged: true}
	return []requestValidationCase{
		{name: "project floor bound is accepted", exercise: exerciseProjectRequest(func(request *ListProjectsRequest) { request.Limit = 1 }), want: wantUnchanged},
		{name: "project ceiling bound and cursor are accepted", exercise: exerciseProjectRequest(func(request *ListProjectsRequest) {
			request.Limit = PageLimitMaximum
			request.After = projectPageRequestCursor()
		}), want: wantUnchanged},
		{name: "project unknown order is rejected", exercise: exerciseProjectRequest(func(request *ListProjectsRequest) { request.Order = PageOrderUnknown }), want: wantUnchanged, wantErr: core.ErrTaskManagerContract},
		{name: "project zero bound is rejected", exercise: exerciseProjectRequest(func(request *ListProjectsRequest) { request.Limit = 0 }), want: wantUnchanged, wantErr: core.ErrTaskManagerContract},
		{name: "project one above ceiling is rejected", exercise: exerciseProjectRequest(func(request *ListProjectsRequest) { request.Limit = PageLimitMaximum + 1 }), want: wantUnchanged, wantErr: core.ErrTaskManagerContract},
		{name: "project malformed cursor is rejected", exercise: exerciseProjectRequest(func(request *ListProjectsRequest) { request.After = &ProjectCursor{} }), want: wantUnchanged, wantErr: core.ErrTaskManagerContract},

		{name: "phase floor bound is accepted", exercise: exercisePhaseRequest(func(request *ListPhasesRequest) { request.Limit = 1 }), want: wantUnchanged},
		{name: "phase ceiling bound and owned cursor are accepted", exercise: exercisePhaseRequest(func(request *ListPhasesRequest) {
			request.Limit = PageLimitMaximum
			request.After = phasePageRequestCursor(request.ProjectID)
		}), want: wantUnchanged},
		{name: "phase unknown order is rejected", exercise: exercisePhaseRequest(func(request *ListPhasesRequest) { request.Order = PageOrderUnknown }), want: wantUnchanged, wantErr: core.ErrTaskManagerContract},
		{name: "phase zero bound is rejected", exercise: exercisePhaseRequest(func(request *ListPhasesRequest) { request.Limit = 0 }), want: wantUnchanged, wantErr: core.ErrTaskManagerContract},
		{name: "phase one above ceiling is rejected", exercise: exercisePhaseRequest(func(request *ListPhasesRequest) { request.Limit = PageLimitMaximum + 1 }), want: wantUnchanged, wantErr: core.ErrTaskManagerContract},
		{name: "phase foreign-project cursor is rejected", exercise: exercisePhaseRequest(func(request *ListPhasesRequest) { request.After = phasePageRequestCursor(foreignRequestIdentity()) }), want: wantUnchanged, wantErr: core.ErrTaskManagerContract},

		{name: "task floor bound is accepted", exercise: exerciseTaskRequest(func(request *ListTasksRequest) { request.Limit = 1 }), want: wantUnchanged},
		{name: "task ceiling bound and owned cursor are accepted", exercise: exerciseTaskRequest(func(request *ListTasksRequest) {
			request.Limit = PageLimitMaximum
			request.After = taskPageRequestCursor(request.ProjectID)
		}), want: wantUnchanged},
		{name: "task unknown order is rejected", exercise: exerciseTaskRequest(func(request *ListTasksRequest) { request.Order = PageOrderUnknown }), want: wantUnchanged, wantErr: core.ErrTaskManagerContract},
		{name: "task zero bound is rejected", exercise: exerciseTaskRequest(func(request *ListTasksRequest) { request.Limit = 0 }), want: wantUnchanged, wantErr: core.ErrTaskManagerContract},
		{name: "task one above ceiling is rejected", exercise: exerciseTaskRequest(func(request *ListTasksRequest) { request.Limit = PageLimitMaximum + 1 }), want: wantUnchanged, wantErr: core.ErrTaskManagerContract},
		{name: "task foreign-project cursor is rejected", exercise: exerciseTaskRequest(func(request *ListTasksRequest) { request.After = taskPageRequestCursor(foreignRequestIdentity()) }), want: wantUnchanged, wantErr: core.ErrTaskManagerContract},
		{name: "task malformed phase selector is rejected", exercise: exerciseTaskRequest(func(request *ListTasksRequest) { invalid := id.UUIDv7{}; request.PhaseID = &invalid }), want: wantUnchanged, wantErr: core.ErrTaskManagerContract},

		{name: "evidence floor bound is accepted", exercise: exerciseEvidenceRequest(func(request *ListEvidenceRequest) { request.Limit = 1 }), want: wantUnchanged},
		{name: "evidence ceiling bound and owned cursor are accepted", exercise: exerciseEvidenceRequest(func(request *ListEvidenceRequest) {
			request.Limit = PageLimitMaximum
			request.After = evidencePageRequestCursor(request.ProjectID, request.TaskID)
		}), want: wantUnchanged},
		{name: "evidence unknown order is rejected", exercise: exerciseEvidenceRequest(func(request *ListEvidenceRequest) { request.Order = PageOrderUnknown }), want: wantUnchanged, wantErr: core.ErrTaskManagerContract},
		{name: "evidence zero bound is rejected", exercise: exerciseEvidenceRequest(func(request *ListEvidenceRequest) { request.Limit = 0 }), want: wantUnchanged, wantErr: core.ErrTaskManagerContract},
		{name: "evidence one above ceiling is rejected", exercise: exerciseEvidenceRequest(func(request *ListEvidenceRequest) { request.Limit = PageLimitMaximum + 1 }), want: wantUnchanged, wantErr: core.ErrTaskManagerContract},
		{name: "evidence foreign-project cursor is rejected", exercise: exerciseEvidenceRequest(func(request *ListEvidenceRequest) {
			request.After = evidencePageRequestCursor(foreignRequestIdentity(), request.TaskID)
		}), want: wantUnchanged, wantErr: core.ErrTaskManagerContract},
		{name: "evidence foreign-task cursor is rejected", exercise: exerciseEvidenceRequest(func(request *ListEvidenceRequest) {
			request.After = evidencePageRequestCursor(request.ProjectID, foreignRequestIdentity())
		}), want: wantUnchanged, wantErr: core.ErrTaskManagerContract},

		{name: "Git floor bound is accepted", exercise: exerciseGitRequest(func(request *ListGitCommitsRequest) { request.Limit = 1 }), want: wantUnchanged},
		{name: "Git ceiling bound and owned cursor are accepted", exercise: exerciseGitRequest(func(request *ListGitCommitsRequest) {
			request.Limit = PageLimitMaximum
			request.After = gitPageRequestCursor(request.ProjectID, request.TaskID)
		}), want: wantUnchanged},
		{name: "Git unknown order is rejected", exercise: exerciseGitRequest(func(request *ListGitCommitsRequest) { request.Order = PageOrderUnknown }), want: wantUnchanged, wantErr: core.ErrTaskManagerContract},
		{name: "Git zero bound is rejected", exercise: exerciseGitRequest(func(request *ListGitCommitsRequest) { request.Limit = 0 }), want: wantUnchanged, wantErr: core.ErrTaskManagerContract},
		{name: "Git one above ceiling is rejected", exercise: exerciseGitRequest(func(request *ListGitCommitsRequest) { request.Limit = PageLimitMaximum + 1 }), want: wantUnchanged, wantErr: core.ErrTaskManagerContract},
		{name: "Git foreign-project cursor is rejected", exercise: exerciseGitRequest(func(request *ListGitCommitsRequest) {
			request.After = gitPageRequestCursor(foreignRequestIdentity(), request.TaskID)
		}), want: wantUnchanged, wantErr: core.ErrTaskManagerContract},
		{name: "Git foreign-task cursor is rejected", exercise: exerciseGitRequest(func(request *ListGitCommitsRequest) {
			request.After = gitPageRequestCursor(request.ProjectID, foreignRequestIdentity())
		}), want: wantUnchanged, wantErr: core.ErrTaskManagerContract},
	}
}

func exerciseProjectRequest(change func(*ListProjectsRequest)) func(*testing.T) (requestValidationObservation, error) {
	return func(*testing.T) (requestValidationObservation, error) {
		request := ListProjectsRequest{Lifecycle: ProjectLifecycleActive, Order: PageOrderDescending, Limit: 7}
		change(&request)
		before := request
		beforeCursor, cursorSet := optionalSnapshot(request.After)
		gotErr := request.Validate()
		unchanged := request == before && optionalValueUnchanged(request.After, beforeCursor, cursorSet)
		return requestValidationObservation{Unchanged: unchanged}, gotErr
	}
}

func exercisePhaseRequest(change func(*ListPhasesRequest)) func(*testing.T) (requestValidationObservation, error) {
	return func(t *testing.T) (requestValidationObservation, error) {
		request := ListPhasesRequest{ProjectID: projectSummaryFixture(t).ID, Order: PageOrderAscending, Limit: 7}
		change(&request)
		before := request
		beforeCursor, cursorSet := optionalSnapshot(request.After)
		gotErr := request.Validate()
		unchanged := request == before && optionalValueUnchanged(request.After, beforeCursor, cursorSet)
		return requestValidationObservation{Unchanged: unchanged}, gotErr
	}
}

func exerciseTaskRequest(change func(*ListTasksRequest)) func(*testing.T) (requestValidationObservation, error) {
	return func(t *testing.T) (requestValidationObservation, error) {
		request := ListTasksRequest{ProjectID: taskSummaryFixture(t).ProjectID, Collection: TaskCollectionActive, Order: PageOrderDescending, Limit: 7}
		change(&request)
		before := request
		beforePhase, phaseSet := optionalSnapshot(request.PhaseID)
		beforeCursor, cursorSet := optionalSnapshot(request.After)
		gotErr := request.Validate()
		unchanged := request == before && optionalValueUnchanged(request.PhaseID, beforePhase, phaseSet) &&
			optionalValueUnchanged(request.After, beforeCursor, cursorSet)
		return requestValidationObservation{Unchanged: unchanged}, gotErr
	}
}

func exerciseEvidenceRequest(change func(*ListEvidenceRequest)) func(*testing.T) (requestValidationObservation, error) {
	return func(t *testing.T) (requestValidationObservation, error) {
		item := evidenceRecordFromRequest(t, appendEvidenceRequestFixture(t))
		request := ListEvidenceRequest{ProjectID: item.ProjectID, TaskID: item.TaskID, Order: PageOrderDescending, Limit: 7}
		change(&request)
		before := request
		beforeCursor, cursorSet := optionalSnapshot(request.After)
		gotErr := request.Validate()
		unchanged := request == before && optionalValueUnchanged(request.After, beforeCursor, cursorSet)
		return requestValidationObservation{Unchanged: unchanged}, gotErr
	}
}

func exerciseGitRequest(change func(*ListGitCommitsRequest)) func(*testing.T) (requestValidationObservation, error) {
	return func(t *testing.T) (requestValidationObservation, error) {
		item := gitCommitRecordFromRequest(t, appendGitCommitRequestFixture(t))
		request := ListGitCommitsRequest{ProjectID: item.ProjectID, TaskID: item.TaskID, Order: PageOrderDescending, Limit: 7}
		change(&request)
		before := request
		beforeCursor, cursorSet := optionalSnapshot(request.After)
		gotErr := request.Validate()
		unchanged := request == before && optionalValueUnchanged(request.After, beforeCursor, cursorSet)
		return requestValidationObservation{Unchanged: unchanged}, gotErr
	}
}

func optionalSnapshot[Value comparable](value *Value) (Value, bool) {
	if value == nil {
		var zero Value
		return zero, false
	}
	return *value, true
}

func optionalValueUnchanged[Value comparable](value *Value, before Value, wasSet bool) bool {
	if value == nil {
		return !wasSet
	}
	return wasSet && *value == before
}

func projectPageRequestCursor() *ProjectCursor {
	return &ProjectCursor{UpdatedAt: temporalRequestInstant(), ID: foreignRequestIdentity()}
}

func phasePageRequestCursor(projectID id.UUIDv7) *PhaseCursor {
	return &PhaseCursor{ProjectID: projectID, Position: 3, ID: foreignRequestIdentity()}
}

func taskPageRequestCursor(projectID id.UUIDv7) *TaskCursor {
	return &TaskCursor{ProjectID: projectID, UpdatedAt: temporalRequestInstant(), ID: foreignRequestIdentity()}
}

func evidencePageRequestCursor(projectID, taskID id.UUIDv7) *EvidenceCursor {
	return &EvidenceCursor{ProjectID: projectID, TaskID: taskID, CreatedAt: temporalRequestInstant(), ID: foreignRequestIdentity()}
}

func gitPageRequestCursor(projectID, taskID id.UUIDv7) *GitCommitCursor {
	return &GitCommitCursor{ProjectID: projectID, TaskID: taskID, CreatedAt: temporalRequestInstant(), ID: foreignRequestIdentity()}
}

func foreignRequestIdentity() id.UUIDv7 {
	value, _ := id.ParseUUIDv7("019ff548-29cb-7451-869e-aa644c0947e7")
	return value
}

func temporalRequestInstant() temporal.NumericInstant {
	value, _ := temporal.NewNumericInstant(temporal.InstantFromNanoseconds(1_776_000_000_000_000_000))
	return value
}

func TestEveryPageProjectionEnforcesBoundOwnershipOrderAndExactContinuation(t *testing.T) {
	t.Parallel()
	for _, tc := range pageValidationCases() {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotErr := tc.validate(t)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("page Validate() error = %v, want %v", gotErr, tc.wantErr)
			}
		})
	}
}

func pageValidationCases() []pageValidationCase {
	return []pageValidationCase{
		{name: "project empty descending terminal page is accepted", validate: func(t *testing.T) error {
			page := projectPageFixture(t)
			page.Items = nil
			page.Next = nil
			return page.Validate()
		}},
		{name: "phase empty ascending terminal page is accepted", validate: func(t *testing.T) error {
			page := phasePageFixture(t)
			page.Items = nil
			page.Next = nil
			return page.Validate()
		}},
		{name: "task empty descending terminal page is accepted", validate: func(t *testing.T) error {
			page := taskPageFixture(t)
			page.Items = nil
			page.Next = nil
			return page.Validate()
		}},
		{name: "evidence empty descending terminal page is accepted", validate: func(t *testing.T) error {
			page := evidencePageFixture(t)
			page.Items = nil
			page.Next = nil
			return page.Validate()
		}},
		{name: "Git empty descending terminal page is accepted", validate: func(t *testing.T) error {
			page := gitCommitPageFixture(t)
			page.Items = nil
			page.Next = nil
			return page.Validate()
		}},
		{name: "project one-row exact continuation is accepted", validate: func(t *testing.T) error { return projectPageFixture(t).Validate() }},
		{name: "phase one-row exact continuation is accepted", validate: func(t *testing.T) error { return phasePageFixture(t).Validate() }},
		{name: "task one-row exact continuation is accepted", validate: func(t *testing.T) error { return taskPageFixture(t).Validate() }},
		{name: "evidence one-row exact continuation is accepted", validate: func(t *testing.T) error { return evidencePageFixture(t).Validate() }},
		{name: "Git one-row exact continuation is accepted", validate: func(t *testing.T) error { return gitCommitPageFixture(t).Validate() }},
		{name: "project exact ceiling terminal page is accepted", validate: func(t *testing.T) error {
			page := projectPageFixture(t)
			page.Items = repeatProjectSummaries(t, int(PageLimitMaximum))
			page.Next = nil
			return page.Validate()
		}},
		{name: "phase exact ceiling terminal page is accepted", validate: func(t *testing.T) error {
			page := phasePageFixture(t)
			page.Items = repeatPhaseSummaries(t, int(PageLimitMaximum))
			page.Next = nil
			return page.Validate()
		}},
		{name: "task exact ceiling terminal page is accepted", validate: func(t *testing.T) error {
			page := taskPageFixture(t)
			page.Items = repeatedTaskSummaries(t, int(PageLimitMaximum))
			page.Next = nil
			return page.Validate()
		}},
		{name: "evidence exact ceiling terminal page is accepted", validate: func(t *testing.T) error {
			page := evidencePageFixture(t)
			page.Items = repeatEvidenceRecords(t, int(PageLimitMaximum))
			page.Next = nil
			return page.Validate()
		}},
		{name: "Git exact ceiling terminal page is accepted", validate: func(t *testing.T) error {
			page := gitCommitPageFixture(t)
			page.Items = repeatGitCommitRecords(t, int(PageLimitMaximum))
			page.Next = nil
			return page.Validate()
		}},
		{name: "project unknown order is rejected", validate: func(t *testing.T) error {
			page := projectPageFixture(t)
			page.Order = PageOrderUnknown
			return page.Validate()
		}, wantErr: core.ErrTaskManagerContract},
		{name: "phase unknown order is rejected", validate: func(t *testing.T) error {
			page := phasePageFixture(t)
			page.Order = PageOrderUnknown
			return page.Validate()
		}, wantErr: core.ErrTaskManagerContract},
		{name: "task unknown order is rejected", validate: func(t *testing.T) error {
			page := taskPageFixture(t)
			page.Order = PageOrderUnknown
			return page.Validate()
		}, wantErr: core.ErrTaskManagerContract},
		{name: "evidence unknown order is rejected", validate: func(t *testing.T) error {
			page := evidencePageFixture(t)
			page.Order = PageOrderUnknown
			return page.Validate()
		}, wantErr: core.ErrTaskManagerContract},
		{name: "Git unknown order is rejected", validate: func(t *testing.T) error {
			page := gitCommitPageFixture(t)
			page.Order = PageOrderUnknown
			return page.Validate()
		}, wantErr: core.ErrTaskManagerContract},
		{name: "project one above page ceiling is rejected", validate: func(t *testing.T) error {
			page := projectPageFixture(t)
			page.Items = repeatProjectSummaries(t, int(PageLimitMaximum)+1)
			page.Next = nil
			return page.Validate()
		}, wantErr: core.ErrTaskManagerContract},
		{name: "phase one above page ceiling is rejected", validate: func(t *testing.T) error {
			page := phasePageFixture(t)
			page.Items = repeatPhaseSummaries(t, int(PageLimitMaximum)+1)
			page.Next = nil
			return page.Validate()
		}, wantErr: core.ErrTaskManagerContract},
		{name: "task one above page ceiling is rejected", validate: func(t *testing.T) error {
			page := taskPageFixture(t)
			page.Items = repeatedTaskSummaries(t, int(PageLimitMaximum)+1)
			page.Next = nil
			return page.Validate()
		}, wantErr: core.ErrTaskManagerContract},
		{name: "evidence one above page ceiling is rejected", validate: func(t *testing.T) error {
			page := evidencePageFixture(t)
			page.Items = repeatEvidenceRecords(t, int(PageLimitMaximum)+1)
			page.Next = nil
			return page.Validate()
		}, wantErr: core.ErrTaskManagerContract},
		{name: "Git one above page ceiling is rejected", validate: func(t *testing.T) error {
			page := gitCommitPageFixture(t)
			page.Items = repeatGitCommitRecords(t, int(PageLimitMaximum)+1)
			page.Next = nil
			return page.Validate()
		}, wantErr: core.ErrTaskManagerContract},
		{name: "project empty page with continuation is rejected", validate: func(t *testing.T) error { page := projectPageFixture(t); page.Items = nil; return page.Validate() }, wantErr: core.ErrTaskManagerContract},
		{name: "phase empty page with continuation is rejected", validate: func(t *testing.T) error { page := phasePageFixture(t); page.Items = nil; return page.Validate() }, wantErr: core.ErrTaskManagerContract},
		{name: "task empty page with continuation is rejected", validate: func(t *testing.T) error { page := taskPageFixture(t); page.Items = nil; return page.Validate() }, wantErr: core.ErrTaskManagerContract},
		{name: "evidence empty page with continuation is rejected", validate: func(t *testing.T) error { page := evidencePageFixture(t); page.Items = nil; return page.Validate() }, wantErr: core.ErrTaskManagerContract},
		{name: "Git empty page with continuation is rejected", validate: func(t *testing.T) error { page := gitCommitPageFixture(t); page.Items = nil; return page.Validate() }, wantErr: core.ErrTaskManagerContract},
		{name: "project lifecycle-spliced row is rejected", validate: func(t *testing.T) error {
			page := projectPageFixture(t)
			page.Items[0].Lifecycle = ProjectLifecycleCompleted
			return page.Validate()
		}, wantErr: core.ErrTaskManagerContract},
		{name: "phase project-spliced row is rejected", validate: func(t *testing.T) error {
			page := phasePageFixture(t)
			page.Items[0].ProjectID = foreignPageIdentity(t)
			return page.Validate()
		}, wantErr: core.ErrTaskManagerContract},
		{name: "task collection-spliced row is rejected", validate: func(t *testing.T) error {
			page := taskPageFixture(t)
			page.Items[0].State = TaskStateCompleted
			return page.Validate()
		}, wantErr: core.ErrTaskManagerContract},
		{name: "evidence task-spliced row is rejected", validate: func(t *testing.T) error {
			page := evidencePageFixture(t)
			page.Items[0].TaskID = foreignPageIdentity(t)
			return page.Validate()
		}, wantErr: core.ErrTaskManagerContract},
		{name: "Git task-spliced row is rejected", validate: func(t *testing.T) error {
			page := gitCommitPageFixture(t)
			page.Items[0].TaskID = foreignPageIdentity(t)
			return page.Validate()
		}, wantErr: core.ErrTaskManagerContract},
		{name: "project continuation not derived from last row is rejected", validate: func(t *testing.T) error {
			page := projectPageFixture(t)
			page.Next.ID = foreignPageIdentity(t)
			return page.Validate()
		}, wantErr: core.ErrTaskManagerContract},
		{name: "phase continuation not derived from last row is rejected", validate: func(t *testing.T) error { page := phasePageFixture(t); page.Next.Position++; return page.Validate() }, wantErr: core.ErrTaskManagerContract},
		{name: "task continuation not derived from last row is rejected", validate: func(t *testing.T) error {
			page := taskPageFixture(t)
			page.Next.ID = foreignPageIdentity(t)
			return page.Validate()
		}, wantErr: core.ErrTaskManagerContract},
		{name: "evidence continuation not derived from last row is rejected", validate: func(t *testing.T) error {
			page := evidencePageFixture(t)
			page.Next.ID = foreignPageIdentity(t)
			return page.Validate()
		}, wantErr: core.ErrTaskManagerContract},
		{name: "Git continuation not derived from last row is rejected", validate: func(t *testing.T) error {
			page := gitCommitPageFixture(t)
			page.Next.ID = foreignPageIdentity(t)
			return page.Validate()
		}, wantErr: core.ErrTaskManagerContract},
	}
}

func projectPageFixture(t testing.TB) ProjectPage {
	t.Helper()
	item := projectSummaryFixture(t)
	return ProjectPage{Items: []ProjectSummary{item}, Next: &ProjectCursor{UpdatedAt: item.UpdatedAt, ID: item.ID}, Lifecycle: item.Lifecycle, Order: PageOrderDescending}
}

func phasePageFixture(t testing.TB) PhasePage {
	t.Helper()
	item := phaseSummaryFromRequest(t, createPhaseRequestFixture(t))
	return PhasePage{ProjectID: item.ProjectID, Items: []PhaseSummary{item}, Next: &PhaseCursor{ProjectID: item.ProjectID, Position: item.Position, ID: item.ID}, Order: PageOrderAscending}
}

func taskPageFixture(t testing.TB) TaskPage {
	t.Helper()
	item := taskSummaryFixture(t)
	return TaskPage{ProjectID: item.ProjectID, Collection: TaskCollectionActive, Order: PageOrderDescending, Items: []TaskSummary{item}, Next: &TaskCursor{ProjectID: item.ProjectID, UpdatedAt: item.UpdatedAt, ID: item.ID}}
}

func evidencePageFixture(t testing.TB) EvidencePage {
	t.Helper()
	item := evidenceRecordFromRequest(t, appendEvidenceRequestFixture(t))
	return EvidencePage{ProjectID: item.ProjectID, TaskID: item.TaskID, Items: []EvidenceRecord{item}, Next: &EvidenceCursor{ProjectID: item.ProjectID, TaskID: item.TaskID, CreatedAt: item.CreatedAt, ID: item.ID}, Order: PageOrderDescending}
}

func gitCommitPageFixture(t testing.TB) GitCommitPage {
	t.Helper()
	item := gitCommitRecordFromRequest(t, appendGitCommitRequestFixture(t))
	return GitCommitPage{ProjectID: item.ProjectID, TaskID: item.TaskID, Items: []GitCommitRecord{item}, Next: &GitCommitCursor{ProjectID: item.ProjectID, TaskID: item.TaskID, CreatedAt: item.CreatedAt, ID: item.ID}, Order: PageOrderDescending}
}

func repeatProjectSummaries(t testing.TB, count int) []ProjectSummary {
	t.Helper()
	items := make([]ProjectSummary, count)
	for index := range items {
		items[index] = projectSummaryFixture(t)
	}
	return items
}

func repeatPhaseSummaries(t testing.TB, count int) []PhaseSummary {
	t.Helper()
	items := make([]PhaseSummary, count)
	for index := range items {
		items[index] = phaseSummaryFromRequest(t, createPhaseRequestFixture(t))
	}
	return items
}

func repeatEvidenceRecords(t testing.TB, count int) []EvidenceRecord {
	t.Helper()
	items := make([]EvidenceRecord, count)
	for index := range items {
		items[index] = evidenceRecordFromRequest(t, appendEvidenceRequestFixture(t))
	}
	return items
}

func repeatGitCommitRecords(t testing.TB, count int) []GitCommitRecord {
	t.Helper()
	items := make([]GitCommitRecord, count)
	for index := range items {
		items[index] = gitCommitRecordFromRequest(t, appendGitCommitRequestFixture(t))
	}
	return items
}

func foreignPageIdentity(t testing.TB) id.UUIDv7 {
	t.Helper()
	return uuidFixture(t, "019ff548-29cb-7451-869e-aa644c0947e7")
}
