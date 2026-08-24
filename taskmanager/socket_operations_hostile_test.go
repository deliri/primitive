package taskmanager

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/id"
)

type socketOperationObservation struct {
	Route     Route
	EntityID  id.UUIDv7
	ProjectID id.UUIDv7
	TaskID    id.UUIDv7
	Items     uint16
	State     TaskState
	Revision  Revision
}

type socketOperationCase struct {
	wantErr  error
	exercise func(*testing.T) (socketOperationObservation, error)
	name     string
	want     socketOperationObservation
}

func TestEveryTaskManagerOperationCrossesItsPairedRealTLSSocket(t *testing.T) {
	t.Parallel()
	for _, tc := range taskManagerSocketOperationCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := tc.exercise(t)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("task-manager socket operation error = %v, want %v", gotErr, tc.wantErr)
			}
			if got != tc.want {
				t.Fatalf("task-manager socket observation = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func taskManagerSocketOperationCases(t testing.TB) []socketOperationCase {
	t.Helper()
	projectRequest := createProjectRequestFixture(t)
	projectDetail := projectDetailFromRequest(t, projectRequest)
	phaseRequest := createPhaseRequestFixture(t)
	phaseSummary := phaseSummaryFromRequest(t, phaseRequest)
	taskRequest := createTaskRequestFixture(t)
	taskDetail := taskDetailFromCreate(t, taskRequest)
	updateRequest := updateTaskRequestFixture(t)
	updatedTask := updatedTaskDetailFromRequest(t, updateRequest)
	evidenceRequest := appendEvidenceRequestFixture(t)
	evidenceRecord := evidenceRecordFromRequest(t, evidenceRequest)
	gitRequest := appendGitCommitRequestFixture(t)
	gitRecord := gitCommitRecordFromRequest(t, gitRequest)
	completeRequest := completeTaskRequestFixture(t)
	completedTask := completedTaskFromRequest(t, completeRequest)
	listProjectsRequest := ListProjectsRequest{Lifecycle: ProjectLifecycleActive, Order: PageOrderDescending, Limit: 7}
	listProjectsPage := ProjectPage{Lifecycle: ProjectLifecycleActive, Order: PageOrderDescending, Items: []ProjectSummary{projectDetail.Summary}}
	getProjectRequest := GetProjectRequest{ProjectID: projectRequest.ID}
	listPhasesRequest := ListPhasesRequest{ProjectID: phaseRequest.ProjectID, Order: PageOrderAscending, Limit: 7}
	listPhasesPage := PhasePage{ProjectID: phaseRequest.ProjectID, Order: PageOrderAscending, Items: []PhaseSummary{phaseSummary}}
	listTasksRequest := ListTasksRequest{ProjectID: taskRequest.ProjectID, Collection: TaskCollectionActive, Order: PageOrderDescending, Limit: 7}
	listTasksPage := TaskPage{ProjectID: taskRequest.ProjectID, Collection: TaskCollectionActive, Order: PageOrderDescending, Items: []TaskSummary{taskDetail.Summary}}
	getTaskRequest := GetTaskRequest{ProjectID: taskRequest.ProjectID, TaskID: taskRequest.ID}
	listEvidenceRequest := ListEvidenceRequest{ProjectID: evidenceRequest.ProjectID, TaskID: evidenceRequest.TaskID, Order: PageOrderDescending, Limit: 7}
	listEvidencePage := EvidencePage{ProjectID: evidenceRequest.ProjectID, TaskID: evidenceRequest.TaskID, Order: PageOrderDescending, Items: []EvidenceRecord{evidenceRecord}}
	listGitRequest := ListGitCommitsRequest{ProjectID: gitRequest.ProjectID, TaskID: gitRequest.TaskID, Order: PageOrderDescending, Limit: 7}
	listGitPage := GitCommitPage{ProjectID: gitRequest.ProjectID, TaskID: gitRequest.TaskID, Order: PageOrderDescending, Items: []GitCommitRecord{gitRecord}}

	return []socketOperationCase{
		{
			name: "list projects admits one bounded lifecycle page",
			exercise: func(t *testing.T) (socketOperationObservation, error) {
				return exerciseSocketOperation(t, func(writer http.ResponseWriter, request *http.Request) {
					received, err := ReceiveListProjects(request)
					if err != nil || *received.Body != listProjectsRequest || WriteProjectPage(writer, listProjectsPage) != nil {
						writeTaskManagerTestFailure(writer)
					}
				}, func(client Client) (socketOperationObservation, error) {
					page, err := client.ListProjects(context.Background(), listProjectsRequest)
					got := socketOperationObservation{Route: RouteListProjects, Items: uint16(len(page.Items))}
					if len(page.Items) == 1 {
						got.EntityID = page.Items[0].ID
					}
					return got, err
				})
			},
			want: socketOperationObservation{Route: RouteListProjects, EntityID: projectRequest.ID, Items: 1},
		},
		{
			name: "get project returns one directly addressed detail",
			exercise: func(t *testing.T) (socketOperationObservation, error) {
				return exerciseSocketOperation(t, func(writer http.ResponseWriter, request *http.Request) {
					received, err := ReceiveGetProject(request)
					if err != nil || *received.Body != getProjectRequest || WriteProjectDetail(writer, projectDetail) != nil {
						writeTaskManagerTestFailure(writer)
					}
				}, func(client Client) (socketOperationObservation, error) {
					got, err := client.GetProject(context.Background(), getProjectRequest)
					return socketOperationObservation{Route: RouteGetProject, EntityID: got.Summary.ID, ProjectID: got.Summary.ID, Revision: got.Summary.Revision}, err
				})
			},
			want: socketOperationObservation{Route: RouteGetProject, EntityID: projectRequest.ID, ProjectID: projectRequest.ID, Revision: projectDetail.Summary.Revision},
		},
		{
			name: "create project binds mutation and entity identities",
			exercise: func(t *testing.T) (socketOperationObservation, error) {
				return exerciseSocketOperation(t, func(writer http.ResponseWriter, request *http.Request) {
					received, err := ReceiveCreateProject(request)
					if err != nil || *received.Body != projectRequest || WriteProjectDetail(writer, projectDetail) != nil {
						writeTaskManagerTestFailure(writer)
					}
				}, func(client Client) (socketOperationObservation, error) {
					got, err := client.CreateProject(context.Background(), projectRequest)
					return socketOperationObservation{Route: RouteCreateProject, EntityID: got.Summary.ID, ProjectID: got.Summary.ID, Revision: got.Summary.Revision}, err
				})
			},
			want: socketOperationObservation{Route: RouteCreateProject, EntityID: projectRequest.ID, ProjectID: projectRequest.ID, Revision: projectDetail.Summary.Revision},
		},
		{
			name: "list phases admits one project-local bounded page",
			exercise: func(t *testing.T) (socketOperationObservation, error) {
				return exerciseSocketOperation(t, func(writer http.ResponseWriter, request *http.Request) {
					received, err := ReceiveListPhases(request)
					if err != nil || *received.Body != listPhasesRequest || WritePhasePage(writer, listPhasesPage) != nil {
						writeTaskManagerTestFailure(writer)
					}
				}, func(client Client) (socketOperationObservation, error) {
					page, err := client.ListPhases(context.Background(), listPhasesRequest)
					got := socketOperationObservation{Route: RouteListPhases, ProjectID: page.ProjectID, Items: uint16(len(page.Items))}
					if len(page.Items) == 1 {
						got.EntityID = page.Items[0].ID
					}
					return got, err
				})
			},
			want: socketOperationObservation{Route: RouteListPhases, EntityID: phaseRequest.ID, ProjectID: phaseRequest.ProjectID, Items: 1},
		},
		{
			name: "create phase binds project position and entity identities",
			exercise: func(t *testing.T) (socketOperationObservation, error) {
				return exerciseSocketOperation(t, func(writer http.ResponseWriter, request *http.Request) {
					received, err := ReceiveCreatePhase(request)
					if err != nil || *received.Body != phaseRequest || WritePhaseSummary(writer, phaseSummary) != nil {
						writeTaskManagerTestFailure(writer)
					}
				}, func(client Client) (socketOperationObservation, error) {
					got, err := client.CreatePhase(context.Background(), phaseRequest)
					return socketOperationObservation{Route: RouteCreatePhase, EntityID: got.ID, ProjectID: got.ProjectID, Revision: got.Revision}, err
				})
			},
			want: socketOperationObservation{Route: RouteCreatePhase, EntityID: phaseRequest.ID, ProjectID: phaseRequest.ProjectID, Revision: phaseSummary.Revision},
		},
		{
			name: "list tasks admits one active project-local bounded page",
			exercise: func(t *testing.T) (socketOperationObservation, error) {
				return exerciseSocketOperation(t, func(writer http.ResponseWriter, request *http.Request) {
					received, err := ReceiveListTasks(request)
					if err != nil || !listTaskRequestsEqual(*received.Body, listTasksRequest) || WriteTaskPage(writer, listTasksPage) != nil {
						writeTaskManagerTestFailure(writer)
					}
				}, func(client Client) (socketOperationObservation, error) {
					page, err := client.ListTasks(context.Background(), listTasksRequest)
					got := socketOperationObservation{Route: RouteListTasks, ProjectID: page.ProjectID, Items: uint16(len(page.Items))}
					if len(page.Items) == 1 {
						got.EntityID = page.Items[0].ID
						got.TaskID = page.Items[0].ID
						got.State = page.Items[0].State
					}
					return got, err
				})
			},
			want: socketOperationObservation{Route: RouteListTasks, EntityID: taskRequest.ID, ProjectID: taskRequest.ProjectID, TaskID: taskRequest.ID, Items: 1, State: TaskStateBacklog},
		},
		{
			name: "get task returns detail without embedded proof history",
			exercise: func(t *testing.T) (socketOperationObservation, error) {
				return exerciseSocketOperation(t, func(writer http.ResponseWriter, request *http.Request) {
					received, err := ReceiveGetTask(request)
					if err != nil || *received.Body != getTaskRequest || WriteTaskDetail(writer, taskDetail) != nil {
						writeTaskManagerTestFailure(writer)
					}
				}, func(client Client) (socketOperationObservation, error) {
					got, err := client.GetTask(context.Background(), getTaskRequest)
					return socketOperationObservation{Route: RouteGetTask, EntityID: got.Summary.ID, ProjectID: got.Summary.ProjectID, TaskID: got.Summary.ID, State: got.Summary.State, Revision: got.Summary.Revision}, err
				})
			},
			want: socketOperationObservation{Route: RouteGetTask, EntityID: taskRequest.ID, ProjectID: taskRequest.ProjectID, TaskID: taskRequest.ID, State: TaskStateBacklog, Revision: taskDetail.Summary.Revision},
		},
		{
			name: "create task binds project phase kind and entity identities",
			exercise: func(t *testing.T) (socketOperationObservation, error) {
				return exerciseSocketOperation(t, func(writer http.ResponseWriter, request *http.Request) {
					received, err := ReceiveCreateTask(request)
					if err != nil || *received.Body != taskRequest || WriteTaskDetail(writer, taskDetail) != nil {
						writeTaskManagerTestFailure(writer)
					}
				}, func(client Client) (socketOperationObservation, error) {
					got, err := client.CreateTask(context.Background(), taskRequest)
					return socketOperationObservation{Route: RouteCreateTask, EntityID: got.Summary.ID, ProjectID: got.Summary.ProjectID, TaskID: got.Summary.ID, State: got.Summary.State, Revision: got.Summary.Revision}, err
				})
			},
			want: socketOperationObservation{Route: RouteCreateTask, EntityID: taskRequest.ID, ProjectID: taskRequest.ProjectID, TaskID: taskRequest.ID, State: TaskStateBacklog, Revision: taskDetail.Summary.Revision},
		},
		{
			name: "update task binds replay identity revision and every changed field",
			exercise: func(t *testing.T) (socketOperationObservation, error) {
				return exerciseSocketOperation(t, func(writer http.ResponseWriter, request *http.Request) {
					received, err := ReceiveUpdateTask(request)
					if err != nil || !updateTaskRequestsEqual(*received.Body, updateRequest) || WriteTaskDetail(writer, updatedTask) != nil {
						writeTaskManagerTestFailure(writer)
					}
				}, func(client Client) (socketOperationObservation, error) {
					got, err := client.UpdateTask(context.Background(), updateRequest)
					return socketOperationObservation{Route: RouteUpdateTask, EntityID: got.Summary.ID, ProjectID: got.Summary.ProjectID, TaskID: got.Summary.ID, State: got.Summary.State, Revision: got.Summary.Revision}, err
				})
			},
			want: socketOperationObservation{Route: RouteUpdateTask, EntityID: updateRequest.TaskID, ProjectID: updateRequest.ProjectID, TaskID: updateRequest.TaskID, State: *updateRequest.Change.State, Revision: updatedTask.Summary.Revision},
		},
		{
			name: "list evidence admits one task-local bounded proof page",
			exercise: func(t *testing.T) (socketOperationObservation, error) {
				return exerciseSocketOperation(t, func(writer http.ResponseWriter, request *http.Request) {
					received, err := ReceiveListEvidence(request)
					if err != nil || *received.Body != listEvidenceRequest || WriteEvidencePage(writer, listEvidencePage) != nil {
						writeTaskManagerTestFailure(writer)
					}
				}, func(client Client) (socketOperationObservation, error) {
					page, err := client.ListEvidence(context.Background(), listEvidenceRequest)
					got := socketOperationObservation{Route: RouteListEvidence, ProjectID: page.ProjectID, TaskID: page.TaskID, Items: uint16(len(page.Items))}
					if len(page.Items) == 1 {
						got.EntityID = page.Items[0].ID
						got.Revision = page.Items[0].TaskRevision
					}
					return got, err
				})
			},
			want: socketOperationObservation{Route: RouteListEvidence, EntityID: evidenceRequest.ID, ProjectID: evidenceRequest.ProjectID, TaskID: evidenceRequest.TaskID, Items: 1, Revision: evidenceRecord.TaskRevision},
		},
		{
			name: "append evidence binds immutable object reference and task revision",
			exercise: func(t *testing.T) (socketOperationObservation, error) {
				return exerciseSocketOperation(t, func(writer http.ResponseWriter, request *http.Request) {
					received, err := ReceiveAppendEvidence(request)
					if err != nil || *received.Body != evidenceRequest || WriteEvidenceRecord(writer, evidenceRecord) != nil {
						writeTaskManagerTestFailure(writer)
					}
				}, func(client Client) (socketOperationObservation, error) {
					got, err := client.AppendEvidence(context.Background(), evidenceRequest)
					return socketOperationObservation{Route: RouteAppendEvidence, EntityID: got.ID, ProjectID: got.ProjectID, TaskID: got.TaskID, Revision: got.TaskRevision}, err
				})
			},
			want: socketOperationObservation{Route: RouteAppendEvidence, EntityID: evidenceRequest.ID, ProjectID: evidenceRequest.ProjectID, TaskID: evidenceRequest.TaskID, Revision: evidenceRecord.TaskRevision},
		},
		{
			name: "list Git commits admits one task-local bounded proof page",
			exercise: func(t *testing.T) (socketOperationObservation, error) {
				return exerciseSocketOperation(t, func(writer http.ResponseWriter, request *http.Request) {
					received, err := ReceiveListGitCommits(request)
					if err != nil || *received.Body != listGitRequest || WriteGitCommitPage(writer, listGitPage) != nil {
						writeTaskManagerTestFailure(writer)
					}
				}, func(client Client) (socketOperationObservation, error) {
					page, err := client.ListGitCommits(context.Background(), listGitRequest)
					got := socketOperationObservation{Route: RouteListGitCommits, ProjectID: page.ProjectID, TaskID: page.TaskID, Items: uint16(len(page.Items))}
					if len(page.Items) == 1 {
						got.EntityID = page.Items[0].ID
						got.Revision = page.Items[0].TaskRevision
					}
					return got, err
				})
			},
			want: socketOperationObservation{Route: RouteListGitCommits, EntityID: gitRequest.ID, ProjectID: gitRequest.ProjectID, TaskID: gitRequest.TaskID, Items: 1, Revision: gitRecord.TaskRevision},
		},
		{
			name: "append Git commit binds exact repository parent result and task revision",
			exercise: func(t *testing.T) (socketOperationObservation, error) {
				return exerciseSocketOperation(t, func(writer http.ResponseWriter, request *http.Request) {
					received, err := ReceiveAppendGitCommit(request)
					if err != nil || *received.Body != gitRequest || WriteGitCommitRecord(writer, gitRecord) != nil {
						writeTaskManagerTestFailure(writer)
					}
				}, func(client Client) (socketOperationObservation, error) {
					got, err := client.AppendGitCommit(context.Background(), gitRequest)
					return socketOperationObservation{Route: RouteAppendGitCommit, EntityID: got.ID, ProjectID: got.ProjectID, TaskID: got.TaskID, Revision: got.TaskRevision}, err
				})
			},
			want: socketOperationObservation{Route: RouteAppendGitCommit, EntityID: gitRequest.ID, ProjectID: gitRequest.ProjectID, TaskID: gitRequest.TaskID, Revision: gitRecord.TaskRevision},
		},
		{
			name: "complete task binds exact revision and completed response",
			exercise: func(t *testing.T) (socketOperationObservation, error) {
				return exerciseSocketOperation(t, func(writer http.ResponseWriter, request *http.Request) {
					received, err := ReceiveCompleteTask(request)
					if err != nil || *received.Body != completeRequest || WriteTaskSummary(writer, completedTask) != nil {
						writeTaskManagerTestFailure(writer)
					}
				}, func(client Client) (socketOperationObservation, error) {
					got, err := client.CompleteTask(context.Background(), completeRequest)
					return socketOperationObservation{Route: RouteCompleteTask, EntityID: got.ID, ProjectID: got.ProjectID, TaskID: got.ID, State: got.State, Revision: got.Revision}, err
				})
			},
			want: socketOperationObservation{Route: RouteCompleteTask, EntityID: completeRequest.TaskID, ProjectID: completeRequest.ProjectID, TaskID: completeRequest.TaskID, State: TaskStateCompleted, Revision: completedTask.Revision},
		},
	}
}

type socketOperationHandler func(http.ResponseWriter, *http.Request)
type socketOperationCall func(Client) (socketOperationObservation, error)

func exerciseSocketOperation(t *testing.T, handler socketOperationHandler, call socketOperationCall) (socketOperationObservation, error) {
	t.Helper()
	server := httptest.NewTLSServer(http.HandlerFunc(handler))
	defer server.Close()
	return call(socketClientFixture(t, server))
}

type unboundSocketObservation struct{ Nonzero bool }

type unboundSocketCase struct {
	wantErr  error
	exercise func(*testing.T) (unboundSocketObservation, error)
	name     string
	want     unboundSocketObservation
}

func TestTaskManagerClientRejectsValidButUnboundResponses(t *testing.T) {
	t.Parallel()
	for _, tc := range taskManagerUnboundSocketCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, gotErr := tc.exercise(t)
			if !errors.Is(gotErr, tc.wantErr) {
				t.Fatalf("task-manager unbound response error = %v, want %v", gotErr, tc.wantErr)
			}
			if got != tc.want {
				t.Fatalf("task-manager unbound response observation = %+v, want %+v", got, tc.want)
			}
		})
	}
}

func taskManagerUnboundSocketCases(t testing.TB) []unboundSocketCase {
	t.Helper()
	foreignID := uuidFixture(t, "019ff548-29cb-7451-869e-aa644c0947e7")
	projectRequest := createProjectRequestFixture(t)
	taskRequest := createTaskRequestFixture(t)
	updateRequest := updateTaskRequestFixture(t)
	evidenceRequest := appendEvidenceRequestFixture(t)
	gitRequest := appendGitCommitRequestFixture(t)
	completeRequest := completeTaskRequestFixture(t)
	return []unboundSocketCase{
		unboundProjectCreateCase(t, foreignID, projectRequest),
		unboundProjectGetCase(t, foreignID, projectRequest),
		unboundTaskListCase(t),
		unboundTaskGetCase(t, foreignID, taskRequest),
		unboundTaskCreateCase(t, taskRequest),
		unboundTaskUpdateCase(t, updateRequest),
		unboundEvidenceListCase(t, foreignID, evidenceRequest),
		unboundEvidenceAppendCase(t, evidenceRequest),
		unboundGitListCase(t, foreignID, gitRequest),
		unboundGitAppendCase(t, gitRequest),
		unboundCompleteCase(t, completeRequest),
	}
}

type unboundSocketCall func(Client) (bool, error)

func exerciseUnboundSocket(t *testing.T, handler socketOperationHandler, call unboundSocketCall) (unboundSocketObservation, error) {
	t.Helper()
	server := httptest.NewTLSServer(http.HandlerFunc(handler))
	defer server.Close()
	nonzero, err := call(socketClientFixture(t, server))
	return unboundSocketObservation{Nonzero: nonzero}, err
}

func unboundProjectCreateCase(t testing.TB, foreignID id.UUIDv7, request CreateProjectRequest) unboundSocketCase {
	t.Helper()
	response := projectDetailFromRequest(t, request)
	response.Summary.ID = foreignID
	return unboundSocketCase{name: "project creation rejects another valid project identity", wantErr: core.ErrTaskManagerContract, exercise: func(t *testing.T) (unboundSocketObservation, error) {
		return exerciseUnboundSocket(t, func(writer http.ResponseWriter, _ *http.Request) { _ = WriteProjectDetail(writer, response) }, func(client Client) (bool, error) {
			got, err := client.CreateProject(context.Background(), request)
			return got != (ProjectDetail{}), err
		})
	}}
}

func unboundProjectGetCase(t testing.TB, foreignID id.UUIDv7, request CreateProjectRequest) unboundSocketCase {
	t.Helper()
	response := projectDetailFromRequest(t, request)
	response.Summary.ID = foreignID
	return unboundSocketCase{name: "direct project read rejects another valid project identity", wantErr: core.ErrTaskManagerContract, exercise: func(t *testing.T) (unboundSocketObservation, error) {
		return exerciseUnboundSocket(t, func(writer http.ResponseWriter, _ *http.Request) { _ = WriteProjectDetail(writer, response) }, func(client Client) (bool, error) {
			got, err := client.GetProject(context.Background(), GetProjectRequest{ProjectID: request.ID})
			return got != (ProjectDetail{}), err
		})
	}}
}

func unboundTaskListCase(t testing.TB) unboundSocketCase {
	t.Helper()
	request := listTasksRequestFixture(t)
	request.Limit = 1
	page := TaskPage{ProjectID: request.ProjectID, Collection: request.Collection, Order: request.Order, Items: []TaskSummary{taskSummaryFixture(t), secondTaskSummaryFixture(t)}}
	return unboundSocketCase{name: "task listing rejects a page larger than the requested bound", wantErr: core.ErrTaskManagerContract, exercise: func(t *testing.T) (unboundSocketObservation, error) {
		return exerciseUnboundSocket(t, func(writer http.ResponseWriter, _ *http.Request) { _ = WriteTaskPage(writer, page) }, func(client Client) (bool, error) {
			got, err := client.ListTasks(context.Background(), request)
			return got.ProjectID != (id.UUIDv7{}) || got.Collection != TaskCollectionUnknown || len(got.Items) != 0 || got.Next != nil, err
		})
	}}
}

func unboundTaskGetCase(t testing.TB, foreignID id.UUIDv7, request CreateTaskRequest) unboundSocketCase {
	t.Helper()
	response := taskDetailFromCreate(t, request)
	response.Summary.ID = foreignID
	return unboundSocketCase{name: "direct task read rejects another valid task identity", wantErr: core.ErrTaskManagerContract, exercise: func(t *testing.T) (unboundSocketObservation, error) {
		return exerciseUnboundSocket(t, func(writer http.ResponseWriter, _ *http.Request) { _ = WriteTaskDetail(writer, response) }, func(client Client) (bool, error) {
			got, err := client.GetTask(context.Background(), GetTaskRequest{ProjectID: request.ProjectID, TaskID: request.ID})
			return got != (TaskDetail{}), err
		})
	}}
}

func unboundTaskCreateCase(t testing.TB, request CreateTaskRequest) unboundSocketCase {
	t.Helper()
	response := taskDetailFromCreate(t, request)
	response.Description = mustDescription(t, "Foreign task description")
	return unboundSocketCase{name: "task creation rejects another valid description", wantErr: core.ErrTaskManagerContract, exercise: func(t *testing.T) (unboundSocketObservation, error) {
		return exerciseUnboundSocket(t, func(writer http.ResponseWriter, _ *http.Request) { _ = WriteTaskDetail(writer, response) }, func(client Client) (bool, error) {
			got, err := client.CreateTask(context.Background(), request)
			return got != (TaskDetail{}), err
		})
	}}
}

func unboundTaskUpdateCase(t testing.TB, request UpdateTaskRequest) unboundSocketCase {
	t.Helper()
	response := updatedTaskDetailFromRequest(t, request)
	response.Description = mustDescription(t, "Different description")
	return unboundSocketCase{name: "task update rejects a valid response that omits the requested change", wantErr: core.ErrTaskManagerContract, exercise: func(t *testing.T) (unboundSocketObservation, error) {
		return exerciseUnboundSocket(t, func(writer http.ResponseWriter, _ *http.Request) { _ = WriteTaskDetail(writer, response) }, func(client Client) (bool, error) {
			got, err := client.UpdateTask(context.Background(), request)
			return got != (TaskDetail{}), err
		})
	}}
}

func unboundEvidenceListCase(t testing.TB, foreignID id.UUIDv7, source AppendEvidenceRequest) unboundSocketCase {
	t.Helper()
	page := EvidencePage{ProjectID: source.ProjectID, TaskID: foreignID, Order: PageOrderDescending}
	request := ListEvidenceRequest{ProjectID: source.ProjectID, TaskID: source.TaskID, Order: PageOrderDescending, Limit: 7}
	return unboundSocketCase{name: "evidence listing rejects a valid page for another task", wantErr: core.ErrTaskManagerContract, exercise: func(t *testing.T) (unboundSocketObservation, error) {
		return exerciseUnboundSocket(t, func(writer http.ResponseWriter, _ *http.Request) { _ = WriteEvidencePage(writer, page) }, func(client Client) (bool, error) {
			got, err := client.ListEvidence(context.Background(), request)
			return got.ProjectID != (id.UUIDv7{}) || got.TaskID != (id.UUIDv7{}) || len(got.Items) != 0 || got.Next != nil, err
		})
	}}
}

func unboundEvidenceAppendCase(t testing.TB, request AppendEvidenceRequest) unboundSocketCase {
	t.Helper()
	response := evidenceRecordFromRequest(t, request)
	response.Location, _ = core.ParseHTTPEndpoint("https://storage.googleapis.com/foreign-proof/image.png")
	return unboundSocketCase{name: "evidence append rejects another valid object URL", wantErr: core.ErrTaskManagerContract, exercise: func(t *testing.T) (unboundSocketObservation, error) {
		return exerciseUnboundSocket(t, func(writer http.ResponseWriter, _ *http.Request) { _ = WriteEvidenceRecord(writer, response) }, func(client Client) (bool, error) {
			got, err := client.AppendEvidence(context.Background(), request)
			return got != (EvidenceRecord{}), err
		})
	}}
}

func unboundGitListCase(t testing.TB, foreignID id.UUIDv7, source AppendGitCommitRequest) unboundSocketCase {
	t.Helper()
	page := GitCommitPage{ProjectID: source.ProjectID, TaskID: foreignID, Order: PageOrderDescending}
	request := ListGitCommitsRequest{ProjectID: source.ProjectID, TaskID: source.TaskID, Order: PageOrderDescending, Limit: 7}
	return unboundSocketCase{name: "Git listing rejects a valid page for another task", wantErr: core.ErrTaskManagerContract, exercise: func(t *testing.T) (unboundSocketObservation, error) {
		return exerciseUnboundSocket(t, func(writer http.ResponseWriter, _ *http.Request) { _ = WriteGitCommitPage(writer, page) }, func(client Client) (bool, error) {
			got, err := client.ListGitCommits(context.Background(), request)
			return got.ProjectID != (id.UUIDv7{}) || got.TaskID != (id.UUIDv7{}) || len(got.Items) != 0 || got.Next != nil, err
		})
	}}
}

func unboundGitAppendCase(t testing.TB, request AppendGitCommitRequest) unboundSocketCase {
	t.Helper()
	response := gitCommitRecordFromRequest(t, request)
	response.Result, _ = core.ParseBuildCommit("cccccccccccccccccccccccccccccccccccccccc")
	return unboundSocketCase{name: "Git append rejects another valid result commit", wantErr: core.ErrTaskManagerContract, exercise: func(t *testing.T) (unboundSocketObservation, error) {
		return exerciseUnboundSocket(t, func(writer http.ResponseWriter, _ *http.Request) { _ = WriteGitCommitRecord(writer, response) }, func(client Client) (bool, error) {
			got, err := client.AppendGitCommit(context.Background(), request)
			return got != (GitCommitRecord{}), err
		})
	}}
}

func unboundCompleteCase(t testing.TB, request CompleteTaskRequest) unboundSocketCase {
	t.Helper()
	response := completedTaskFromRequest(t, request)
	response.Revision = request.ExpectedRevision
	return unboundSocketCase{name: "completion rejects a non-advancing revision", wantErr: core.ErrTaskManagerContract, exercise: func(t *testing.T) (unboundSocketObservation, error) {
		return exerciseUnboundSocket(t, func(writer http.ResponseWriter, _ *http.Request) { _ = WriteTaskSummary(writer, response) }, func(client Client) (bool, error) {
			got, err := client.CompleteTask(context.Background(), request)
			return got != (TaskSummary{}), err
		})
	}}
}

func phaseSummaryFromRequest(t testing.TB, request CreatePhaseRequest) PhaseSummary {
	t.Helper()
	return PhaseSummary{
		ID: request.ID, ProjectID: request.ProjectID, Name: request.Name, Description: request.Description,
		Revision: mustRevision(t, 1), CreatedAt: instantFixture(t, 1_776_000_000_000_000_000),
		UpdatedAt: instantFixture(t, 1_776_000_000_000_000_000), Position: request.Position,
	}
}

func projectDetailFromRequest(t testing.TB, request CreateProjectRequest) ProjectDetail {
	t.Helper()
	summary := projectSummaryFixture(t)
	summary.ID = request.ID
	summary.Name = request.Name
	summary.Lifecycle = request.Lifecycle
	return ProjectDetail{Summary: summary, Description: request.Description}
}

func taskSummaryFromCreate(t testing.TB, request CreateTaskRequest) TaskSummary {
	t.Helper()
	return TaskSummary{
		ID: request.ID, ProjectID: request.ProjectID, PhaseID: request.PhaseID,
		Title: request.Title, Kind: request.Kind, State: request.State,
		Revision: mustRevision(t, 1), UpdatedAt: instantFixture(t, 1_776_000_000_000_000_000),
	}
}

func taskDetailFromCreate(t testing.TB, request CreateTaskRequest) TaskDetail {
	t.Helper()
	return TaskDetail{Summary: taskSummaryFromCreate(t, request), Description: request.Description, CreatedAt: instantFixture(t, 1_776_000_000_000_000_000)}
}

func updatedTaskDetailFromRequest(t testing.TB, request UpdateTaskRequest) TaskDetail {
	t.Helper()
	detail := TaskDetail{Summary: taskSummaryFixture(t), Description: mustDescription(t, "Existing description"), CreatedAt: instantFixture(t, 1_775_000_000_000_000_000)}
	detail.Summary.ID = request.TaskID
	detail.Summary.ProjectID = request.ProjectID
	detail.Summary.Revision = revisionAfter(t, request.ExpectedRevision)
	if request.Change.Title != nil {
		detail.Summary.Title = *request.Change.Title
	}
	if request.Change.Description != nil {
		detail.Description = *request.Change.Description
	}
	if request.Change.Kind != nil {
		detail.Summary.Kind = *request.Change.Kind
	}
	if request.Change.State != nil {
		detail.Summary.State = *request.Change.State
	}
	return detail
}

func completedTaskFromRequest(t testing.TB, request CompleteTaskRequest) TaskSummary {
	t.Helper()
	response := taskSummaryFixture(t)
	response.ID = request.TaskID
	response.ProjectID = request.ProjectID
	response.State = TaskStateCompleted
	response.Revision = revisionAfter(t, request.ExpectedRevision)
	return response
}

func evidenceRecordFromRequest(t testing.TB, request AppendEvidenceRequest) EvidenceRecord {
	t.Helper()
	return EvidenceRecord{ID: request.ID, ProjectID: request.ProjectID, TaskID: request.TaskID, Kind: request.Kind, Summary: request.Summary, Location: request.Location, Digest: request.Digest, TaskRevision: revisionAfter(t, request.ExpectedRevision), CreatedAt: instantFixture(t, 1_776_000_000_000_000_000)}
}

func gitCommitRecordFromRequest(t testing.TB, request AppendGitCommitRequest) GitCommitRecord {
	t.Helper()
	return GitCommitRecord{ID: request.ID, ProjectID: request.ProjectID, TaskID: request.TaskID, Repository: request.Repository, Parent: request.Parent, Result: request.Result, Summary: request.Summary, TaskRevision: revisionAfter(t, request.ExpectedRevision), CreatedAt: instantFixture(t, 1_776_000_000_000_000_000)}
}

func revisionAfter(t testing.TB, value Revision) Revision {
	t.Helper()
	raw, err := value.Uint64()
	if err != nil {
		t.Fatalf("Revision.Uint64() error = %v, want nil", err)
	}
	return mustRevision(t, raw+1)
}

func listTaskRequestsEqual(left, right ListTasksRequest) bool {
	leftJSON, leftErr := left.MarshalJSON()
	rightJSON, rightErr := right.MarshalJSON()
	return leftErr == nil && rightErr == nil && string(leftJSON) == string(rightJSON)
}

func writeTaskManagerTestFailure(writer http.ResponseWriter) {
	http.Error(writer, core.ErrTaskManagerContract.Error(), http.StatusBadRequest)
}
