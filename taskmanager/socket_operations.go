package taskmanager

import (
	"context"
	"net/http"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

// GetProject returns one directly addressed project without collection history.
func (c Client) GetProject(ctx context.Context, request GetProjectRequest) (ProjectDetail, error) {
	detail, err := sendSingle[GetProjectRequest, ProjectDetail](ctx, c, RouteGetProject, request)
	if err != nil {
		return ProjectDetail{}, err
	}
	if detail.Summary.ID != request.ProjectID {
		return ProjectDetail{}, contractError()
	}
	return detail, nil
}

// CreateProject creates one project through the paired idempotent socket.
func (c Client) CreateProject(ctx context.Context, request CreateProjectRequest) (ProjectDetail, error) {
	result, err := sendMutation[CreateProjectRequest, ProjectDetail](ctx, c, RouteCreateProject, request)
	if err != nil {
		return ProjectDetail{}, err
	}
	if result.Summary.ID != request.ID || result.Summary.Name != request.Name ||
		result.Summary.Lifecycle != request.Lifecycle || result.Description != request.Description {
		return ProjectDetail{}, contractError()
	}
	return result, nil
}

// ListPhases returns one bounded page for one directly addressed project.
func (c Client) ListPhases(ctx context.Context, request ListPhasesRequest) (PhasePage, error) {
	page, err := sendSingle[ListPhasesRequest, PhasePage](ctx, c, RouteListPhases, request)
	if err != nil {
		return PhasePage{}, err
	}
	if page.ProjectID != request.ProjectID || page.Order != request.Order {
		return PhasePage{}, contractError()
	}
	if len(page.Items) > int(request.Limit) {
		return PhasePage{}, contractError()
	}
	return page, nil
}

// CreatePhase creates one project-local phase through the paired socket.
func (c Client) CreatePhase(ctx context.Context, request CreatePhaseRequest) (PhaseSummary, error) {
	result, err := sendMutation[CreatePhaseRequest, PhaseSummary](ctx, c, RouteCreatePhase, request)
	if err != nil {
		return PhaseSummary{}, err
	}
	if result.ID != request.ID || result.ProjectID != request.ProjectID || result.Name != request.Name ||
		result.Description != request.Description || result.Position != request.Position {
		return PhaseSummary{}, contractError()
	}
	return result, nil
}

// ListTasks returns one explicitly selected active or completed task page.
func (c Client) ListTasks(ctx context.Context, request ListTasksRequest) (TaskPage, error) {
	page, err := sendSingle[ListTasksRequest, TaskPage](ctx, c, RouteListTasks, request)
	if err != nil {
		return TaskPage{}, err
	}
	if page.ProjectID != request.ProjectID || page.Collection != request.Collection || page.Order != request.Order {
		return TaskPage{}, contractError()
	}
	if len(page.Items) > int(request.Limit) {
		return TaskPage{}, contractError()
	}
	if request.PhaseID != nil {
		for _, item := range page.Items {
			if item.PhaseID != *request.PhaseID {
				return TaskPage{}, contractError()
			}
		}
	}
	return page, nil
}

// GetTask returns one directly addressed task without embedding proof history.
func (c Client) GetTask(ctx context.Context, request GetTaskRequest) (TaskDetail, error) {
	detail, err := sendSingle[GetTaskRequest, TaskDetail](ctx, c, RouteGetTask, request)
	if err != nil {
		return TaskDetail{}, err
	}
	if detail.Summary.ID != request.TaskID || detail.Summary.ProjectID != request.ProjectID {
		return TaskDetail{}, contractError()
	}
	return detail, nil
}

// CreateTask creates one phase-local task through the paired socket.
func (c Client) CreateTask(ctx context.Context, request CreateTaskRequest) (TaskDetail, error) {
	result, err := sendMutation[CreateTaskRequest, TaskDetail](ctx, c, RouteCreateTask, request)
	if err != nil {
		return TaskDetail{}, err
	}
	if result.Summary.ID != request.ID || result.Summary.ProjectID != request.ProjectID ||
		result.Summary.PhaseID != request.PhaseID || result.Summary.Title != request.Title ||
		result.Summary.Kind != request.Kind || result.Summary.State != request.State ||
		result.Description != request.Description {
		return TaskDetail{}, contractError()
	}
	return result, nil
}

// ListEvidence returns one bounded evidence page for one task.
func (c Client) ListEvidence(ctx context.Context, request ListEvidenceRequest) (EvidencePage, error) {
	page, err := sendSingle[ListEvidenceRequest, EvidencePage](ctx, c, RouteListEvidence, request)
	if err != nil {
		return EvidencePage{}, err
	}
	if page.ProjectID != request.ProjectID || page.TaskID != request.TaskID || page.Order != request.Order || len(page.Items) > int(request.Limit) {
		return EvidencePage{}, contractError()
	}
	return page, nil
}

// AppendEvidence records one immutable external proof reference.
func (c Client) AppendEvidence(ctx context.Context, request AppendEvidenceRequest) (EvidenceRecord, error) {
	result, err := sendMutation[AppendEvidenceRequest, EvidenceRecord](ctx, c, RouteAppendEvidence, request)
	if err != nil {
		return EvidenceRecord{}, err
	}
	if result.ID != request.ID || result.ProjectID != request.ProjectID || result.TaskID != request.TaskID ||
		result.Kind != request.Kind || result.Summary != request.Summary || result.Location != request.Location ||
		result.Digest != request.Digest || !revisionAdvances(request.ExpectedRevision, result.TaskRevision) {
		return EvidenceRecord{}, contractError()
	}
	return result, nil
}

// ListGitCommits returns one bounded Git proof page for one task.
func (c Client) ListGitCommits(ctx context.Context, request ListGitCommitsRequest) (GitCommitPage, error) {
	page, err := sendSingle[ListGitCommitsRequest, GitCommitPage](ctx, c, RouteListGitCommits, request)
	if err != nil {
		return GitCommitPage{}, err
	}
	if page.ProjectID != request.ProjectID || page.TaskID != request.TaskID || page.Order != request.Order || len(page.Items) > int(request.Limit) {
		return GitCommitPage{}, contractError()
	}
	return page, nil
}

// AppendGitCommit records exact parent/result Git identity.
func (c Client) AppendGitCommit(ctx context.Context, request AppendGitCommitRequest) (GitCommitRecord, error) {
	result, err := sendMutation[AppendGitCommitRequest, GitCommitRecord](ctx, c, RouteAppendGitCommit, request)
	if err != nil {
		return GitCommitRecord{}, err
	}
	if result.ID != request.ID || result.ProjectID != request.ProjectID || result.TaskID != request.TaskID ||
		result.Repository != request.Repository || result.Parent != request.Parent || result.Result != request.Result ||
		result.Summary != request.Summary || !revisionAdvances(request.ExpectedRevision, result.TaskRevision) {
		return GitCommitRecord{}, contractError()
	}
	return result, nil
}

// CompleteTask atomically transitions one exact task revision to completed.
func (c Client) CompleteTask(ctx context.Context, request CompleteTaskRequest) (TaskSummary, error) {
	result, err := sendMutation[CompleteTaskRequest, TaskSummary](ctx, c, RouteCompleteTask, request)
	if err != nil {
		return TaskSummary{}, err
	}
	if result.ID != request.TaskID || result.ProjectID != request.ProjectID ||
		result.State != TaskStateCompleted || !revisionAdvances(request.ExpectedRevision, result.Revision) {
		return TaskSummary{}, contractError()
	}
	return result, nil
}

func revisionAdvances(expected, result Revision) bool {
	expectedValue, expectedErr := expected.Uint64()
	resultValue, resultErr := result.Uint64()
	return expectedErr == nil && resultErr == nil && resultValue > expectedValue
}

func sendSingle[
	Request core.ValidatedJSONMarshaler,
	Response core.Validatable,
](ctx context.Context, client Client, route Route, request Request) (Response, error) {
	var zero Response
	semantics, err := clientSemantics(route, exchange.IdempotencyKey{})
	if err != nil {
		return zero, err
	}
	response, err := exchange.SendJSON[Request, Response](exchange.JSONCall[Request]{
		Context: ctx, Client: client.http,
		Request: exchange.JSONRequest[Request]{
			Target: client.targets[route], Body: request, Semantics: semantics,
			Headers: client.headers, ExpectedStatus: core.HTTPStatusOK(),
		},
		Policy: client.policy,
	})
	if err != nil {
		return zero, contractError(err)
	}
	return response.Body, nil
}

func sendMutation[
	Request interface {
		core.ValidatedJSONMarshaler
		exchange.IdempotencyBound
	},
	Response core.Validatable,
](ctx context.Context, client Client, route Route, request Request) (Response, error) {
	var zero Response
	key, err := request.IdempotencyKey()
	if err != nil {
		return zero, err
	}
	semantics, err := clientSemantics(route, key)
	if err != nil {
		return zero, err
	}
	response, err := exchange.SendReplayBoundJSON[Request, Response](exchange.JSONCall[Request]{
		Context: ctx, Client: client.http,
		Request: exchange.JSONRequest[Request]{
			Target: client.targets[route], Body: request, Semantics: semantics,
			Headers: client.headers, ExpectedStatus: core.HTTPStatusOK(),
		},
		Policy: client.policy,
	})
	if err != nil {
		return zero, contractError(err)
	}
	return response.Body, nil
}

func receiveSingle[
	Body any,
	BodyPtr interface {
		*Body
		core.Validatable
	},
](request *http.Request, route Route) (exchange.Received[BodyPtr], error) {
	var zero exchange.Received[BodyPtr]
	semantics, err := serverRoute(request, route)
	if err != nil {
		return zero, err
	}
	policy, err := socketServerPolicy()
	if err != nil {
		return zero, err
	}
	received, err := exchange.ReceiveJSON[Body, BodyPtr](exchange.JSONReceiveCall{
		Request: request, Route: semantics, Policy: policy,
	})
	if err != nil {
		return zero, contractError(err)
	}
	return received, nil
}

func receiveMutation[
	Body any,
	BodyPtr interface {
		*Body
		exchange.IdempotencyBound
	},
](request *http.Request, route Route) (exchange.Received[BodyPtr], error) {
	var zero exchange.Received[BodyPtr]
	semantics, err := serverRoute(request, route)
	if err != nil {
		return zero, err
	}
	policy, err := socketServerPolicy()
	if err != nil {
		return zero, err
	}
	received, err := exchange.ReceiveReplayBoundJSON[Body, BodyPtr](exchange.JSONReceiveCall{
		Request: request, Route: semantics, Policy: policy,
	})
	if err != nil {
		return zero, contractError(err)
	}
	return received, nil
}

func ReceiveCreateProject(request *http.Request) (exchange.Received[*CreateProjectRequest], error) {
	return receiveMutation[CreateProjectRequest, *CreateProjectRequest](request, RouteCreateProject)
}

func ReceiveGetProject(request *http.Request) (exchange.Received[*GetProjectRequest], error) {
	return receiveSingle[GetProjectRequest, *GetProjectRequest](request, RouteGetProject)
}

func ReceiveListPhases(request *http.Request) (exchange.Received[*ListPhasesRequest], error) {
	return receiveSingle[ListPhasesRequest, *ListPhasesRequest](request, RouteListPhases)
}

func ReceiveCreatePhase(request *http.Request) (exchange.Received[*CreatePhaseRequest], error) {
	return receiveMutation[CreatePhaseRequest, *CreatePhaseRequest](request, RouteCreatePhase)
}

func ReceiveListTasks(request *http.Request) (exchange.Received[*ListTasksRequest], error) {
	return receiveSingle[ListTasksRequest, *ListTasksRequest](request, RouteListTasks)
}

func ReceiveGetTask(request *http.Request) (exchange.Received[*GetTaskRequest], error) {
	return receiveSingle[GetTaskRequest, *GetTaskRequest](request, RouteGetTask)
}

func ReceiveCreateTask(request *http.Request) (exchange.Received[*CreateTaskRequest], error) {
	return receiveMutation[CreateTaskRequest, *CreateTaskRequest](request, RouteCreateTask)
}

func ReceiveListEvidence(request *http.Request) (exchange.Received[*ListEvidenceRequest], error) {
	return receiveSingle[ListEvidenceRequest, *ListEvidenceRequest](request, RouteListEvidence)
}

func ReceiveAppendEvidence(request *http.Request) (exchange.Received[*AppendEvidenceRequest], error) {
	return receiveMutation[AppendEvidenceRequest, *AppendEvidenceRequest](request, RouteAppendEvidence)
}

func ReceiveListGitCommits(request *http.Request) (exchange.Received[*ListGitCommitsRequest], error) {
	return receiveSingle[ListGitCommitsRequest, *ListGitCommitsRequest](request, RouteListGitCommits)
}

func ReceiveAppendGitCommit(request *http.Request) (exchange.Received[*AppendGitCommitRequest], error) {
	return receiveMutation[AppendGitCommitRequest, *AppendGitCommitRequest](request, RouteAppendGitCommit)
}

func ReceiveCompleteTask(request *http.Request) (exchange.Received[*CompleteTaskRequest], error) {
	return receiveMutation[CompleteTaskRequest, *CompleteTaskRequest](request, RouteCompleteTask)
}

func WriteProjectDetail(writer http.ResponseWriter, detail ProjectDetail) error {
	return writeJSON(writer, detail)
}

func WritePhasePage(writer http.ResponseWriter, page PhasePage) error {
	return writeJSON(writer, page)
}

func WritePhaseSummary(writer http.ResponseWriter, summary PhaseSummary) error {
	return writeJSON(writer, summary)
}

func WriteTaskPage(writer http.ResponseWriter, page TaskPage) error {
	return writeJSON(writer, page)
}

func WriteTaskDetail(writer http.ResponseWriter, detail TaskDetail) error {
	return writeJSON(writer, detail)
}

func WriteEvidencePage(writer http.ResponseWriter, page EvidencePage) error {
	return writeJSON(writer, page)
}

func WriteEvidenceRecord(writer http.ResponseWriter, record EvidenceRecord) error {
	return writeJSON(writer, record)
}

func WriteGitCommitPage(writer http.ResponseWriter, page GitCommitPage) error {
	return writeJSON(writer, page)
}

func WriteGitCommitRecord(writer http.ResponseWriter, record GitCommitRecord) error {
	return writeJSON(writer, record)
}
