package main

import (
	"context"
	"errors"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/id"
	"github.com/deliri/primitive/v2026/keygen"
	"github.com/deliri/primitive/v2026/taskmanager"
	"github.com/deliri/primitive/v2026/temporal"
)

func executeJob(request executionRequest) (commandResult, error) {
	if err := request.Validate(); err != nil {
		return commandResult{}, commandError("command execution input is invalid", err)
	}
	executor := jobExecutors()[request.job.Operation]
	if executor == nil {
		return commandResult{}, commandError("operation is outside the published domain", nil)
	}
	return executor(request)
}

type executionRequest struct {
	configuration    configurationDocument
	job              jobDocument
	ctx              context.Context
	workingDirectory core.AbsolutePath
	client           taskmanager.Client
}

func (r executionRequest) Validate() error {
	if r.ctx == nil {
		return core.ErrNilContext
	}
	return errors.Join(
		r.workingDirectory.Validate(), r.client.Validate(), r.configuration.Validate(), r.job.Validate(),
	)
}

type jobExecutor func(executionRequest) (commandResult, error)

func jobExecutors() [operationLimit]jobExecutor {
	return [...]jobExecutor{
		operationListProjects:    executeListProjects,
		operationGetProject:      executeGetProject,
		operationCreateProject:   executeCreateProject,
		operationListPhases:      executeListPhases,
		operationCreatePhase:     executeCreatePhase,
		operationListTasks:       executeListTasks,
		operationGetTask:         executeGetTask,
		operationCreateTask:      executeCreateTask,
		operationUpdateTask:      executeUpdateTask,
		operationCompleteTask:    executeCompleteTask,
		operationListEvidence:    executeListEvidence,
		operationAppendEvidence:  executeAppendEvidence,
		operationListGitCommits:  executeListGitCommits,
		operationAppendGitCommit: executeAppendGitCommit,
	}
}

func executeGetProject(request executionRequest) (commandResult, error) {
	projectID, err := request.configuration.projectID()
	if err != nil {
		return commandResult{}, err
	}
	project, err := request.client.GetProject(request.ctx, taskmanager.GetProjectRequest{ProjectID: projectID})
	if err != nil {
		return commandResult{}, err
	}
	return checkedResult(commandResult{
		Revision: commandDocumentRevisionV2, Operation: request.job.Operation, Project: &project,
	})
}

func executeListProjects(request executionRequest) (commandResult, error) {
	input := *request.job.ListProjects
	page, err := request.client.ListProjects(request.ctx, taskmanager.ListProjectsRequest{
		Lifecycle: input.Lifecycle, Order: input.Order, Limit: input.Limit, After: input.After,
	})
	if err != nil {
		return commandResult{}, err
	}
	return checkedResult(commandResult{
		Revision: commandDocumentRevisionV2, Operation: request.job.Operation, Projects: &page,
	})
}

func executeCreateProject(request executionRequest) (commandResult, error) {
	input := *request.job.CreateProject
	projectID, err := freshID()
	if err != nil {
		return commandResult{}, err
	}
	mutationID, err := freshID()
	if err != nil {
		return commandResult{}, err
	}
	name, _ := commandTitle(createProjectNameField, input.Name)
	description, _ := commandDescription(createProjectDescriptionField, input.Description)
	project, err := request.client.CreateProject(request.ctx, taskmanager.CreateProjectRequest{
		ID: projectID, MutationID: mutationID, Name: name, Description: description, Lifecycle: input.Lifecycle,
	})
	if err != nil {
		return commandResult{}, err
	}
	if project.Summary.ID != projectID {
		return commandResult{}, commandError("create_project response identity is not request-bound", nil)
	}
	return checkedResult(commandResult{
		Revision: commandDocumentRevisionV2, Operation: request.job.Operation, Project: &project,
	})
}

func executeListPhases(request executionRequest) (commandResult, error) {
	projectID, err := request.configuration.projectID()
	if err != nil {
		return commandResult{}, err
	}
	input := *request.job.ListPhases
	page, err := request.client.ListPhases(request.ctx, taskmanager.ListPhasesRequest{
		ProjectID: projectID, Order: input.Order, Limit: input.Limit, After: input.After,
	})
	if err != nil {
		return commandResult{}, err
	}
	return checkedResult(commandResult{
		Revision: commandDocumentRevisionV2, Operation: request.job.Operation, Phases: &page,
	})
}

func executeCreatePhase(request executionRequest) (commandResult, error) {
	projectID, err := request.configuration.projectID()
	if err != nil {
		return commandResult{}, err
	}
	phaseID, err := freshID()
	if err != nil {
		return commandResult{}, err
	}
	mutationID, err := freshID()
	if err != nil {
		return commandResult{}, err
	}
	input := *request.job.CreatePhase
	name, _ := commandTitle(createPhaseNameField, input.Name)
	description, _ := commandDescription(createPhaseDescriptionField, input.Description)
	phase, err := request.client.CreatePhase(request.ctx, taskmanager.CreatePhaseRequest{
		ID: phaseID, ProjectID: projectID, MutationID: mutationID,
		Name: name, Description: description, Position: input.Position,
	})
	if err != nil {
		return commandResult{}, err
	}
	if phase.ID != phaseID || phase.ProjectID != projectID {
		return commandResult{}, commandError("create_phase response is not request-bound", nil)
	}
	return checkedResult(commandResult{
		Revision: commandDocumentRevisionV2, Operation: request.job.Operation, Phase: &phase,
	})
}

func executeListTasks(request executionRequest) (commandResult, error) {
	projectID, err := request.configuration.projectID()
	if err != nil {
		return commandResult{}, err
	}
	input := *request.job.ListTasks
	page, err := request.client.ListTasks(request.ctx, taskmanager.ListTasksRequest{
		ProjectID: projectID, PhaseID: input.PhaseID, After: input.After,
		Collection: input.Collection, Order: input.Order, Limit: input.Limit,
	})
	if err != nil {
		return commandResult{}, err
	}
	return checkedResult(commandResult{
		Revision: commandDocumentRevisionV2, Operation: request.job.Operation, Tasks: &page,
	})
}

func executeGetTask(request executionRequest) (commandResult, error) {
	projectID, err := request.configuration.projectID()
	if err != nil {
		return commandResult{}, err
	}
	input := *request.job.GetTask
	task, err := request.client.GetTask(request.ctx, taskmanager.GetTaskRequest{ProjectID: projectID, TaskID: input.TaskID})
	if err != nil {
		return commandResult{}, err
	}
	return checkedResult(commandResult{
		Revision: commandDocumentRevisionV2, Operation: request.job.Operation, TaskDetail: &task,
	})
}

func executeCreateTask(request executionRequest) (commandResult, error) {
	projectID, err := request.configuration.projectID()
	if err != nil {
		return commandResult{}, err
	}
	taskID, err := freshID()
	if err != nil {
		return commandResult{}, err
	}
	mutationID, err := freshID()
	if err != nil {
		return commandResult{}, err
	}
	input := *request.job.CreateTask
	title, _ := commandTitle(createTaskTitleField, input.Title)
	description, _ := commandDescription(createTaskDescriptionField, input.Description)
	task, err := request.client.CreateTask(request.ctx, taskmanager.CreateTaskRequest{
		ID: taskID, ProjectID: projectID, PhaseID: input.PhaseID, MutationID: mutationID,
		Title: title, Description: description, Kind: input.Kind, State: input.State,
	})
	if err != nil {
		return commandResult{}, err
	}
	if task.Summary.ID != taskID || task.Summary.ProjectID != projectID || task.Summary.PhaseID != input.PhaseID {
		return commandResult{}, commandError("create_task response is not request-bound", nil)
	}
	return checkedResult(commandResult{
		Revision: commandDocumentRevisionV2, Operation: request.job.Operation, TaskDetail: &task,
	})
}

func executeUpdateTask(request executionRequest) (commandResult, error) {
	projectID, err := request.configuration.projectID()
	if err != nil {
		return commandResult{}, err
	}
	mutationID, err := freshID()
	if err != nil {
		return commandResult{}, err
	}
	input := *request.job.UpdateTask
	change, err := input.Change.taskChange()
	if err != nil {
		return commandResult{}, err
	}
	task, err := request.client.UpdateTask(request.ctx, taskmanager.UpdateTaskRequest{
		ProjectID: projectID, TaskID: input.TaskID, MutationID: mutationID,
		ExpectedRevision: input.ExpectedRevision, Change: change,
	})
	if err != nil {
		return commandResult{}, err
	}
	if task.Summary.ID != input.TaskID || task.Summary.ProjectID != projectID {
		return commandResult{}, commandError("update_task response is not request-bound", nil)
	}
	return checkedResult(commandResult{
		Revision: commandDocumentRevisionV2, Operation: request.job.Operation, TaskDetail: &task,
	})
}

func (i taskChangeInput) taskChange() (taskmanager.TaskChange, error) {
	if err := i.Validate(); err != nil {
		return taskmanager.TaskChange{}, err
	}
	change := taskmanager.TaskChange{Kind: i.Kind, State: i.State}
	if i.Title != nil {
		value, _ := commandTitle(updateTaskChangeTitleField, *i.Title)
		change.Title = &value
	}
	if i.Description != nil {
		value, _ := commandDescription(updateTaskChangeDescriptionField, *i.Description)
		change.Description = &value
	}
	if err := change.Validate(); err != nil {
		return taskmanager.TaskChange{}, commandError("update_task.change cannot be projected", err)
	}
	return change, nil
}

func executeCompleteTask(request executionRequest) (commandResult, error) {
	projectID, err := request.configuration.projectID()
	if err != nil {
		return commandResult{}, err
	}
	mutationID, err := freshID()
	if err != nil {
		return commandResult{}, err
	}
	input := *request.job.CompleteTask
	task, err := request.client.CompleteTask(request.ctx, taskmanager.CompleteTaskRequest{
		ProjectID: projectID, TaskID: input.TaskID, MutationID: mutationID,
		ExpectedRevision: input.ExpectedRevision,
	})
	if err != nil {
		return commandResult{}, err
	}
	if task.ID != input.TaskID || task.ProjectID != projectID || task.State != taskmanager.TaskStateCompleted {
		return commandResult{}, commandError("complete_task response is not request-bound", nil)
	}
	return checkedResult(commandResult{
		Revision: commandDocumentRevisionV2, Operation: request.job.Operation, Task: &task,
	})
}

func executeAppendEvidence(request executionRequest) (commandResult, error) {
	if err := request.Validate(); err != nil {
		return commandResult{}, commandError("append_evidence execution input is invalid", err)
	}
	if request.job.Operation != operationAppendEvidence || request.job.AppendEvidence == nil {
		return commandResult{}, commandError("append_evidence execution input is not append_evidence", nil)
	}
	evidenceID, err := freshID()
	if err != nil {
		return commandResult{}, err
	}
	mutationID, err := freshID()
	if err != nil {
		return commandResult{}, err
	}
	input := *request.job.AppendEvidence
	uploaded, err := uploadTaskEvidence(request.ctx, taskEvidenceUploadRequest{
		WorkingDirectory: request.workingDirectory,
		Configuration:    request.configuration,
		Input:            input,
	})
	if err != nil {
		return commandResult{}, err
	}
	return appendUploadedTaskEvidence(request.ctx, taskEvidenceAppendRequest{
		Client: request.client, Configuration: request.configuration, Job: request.job,
		Uploaded: uploaded, EvidenceID: evidenceID, MutationID: mutationID,
	})
}

type taskEvidenceAppendRequest struct {
	Client        taskmanager.Client
	Configuration configurationDocument
	Job           jobDocument
	Uploaded      taskEvidenceUploadReceipt
	EvidenceID    id.UUIDv7
	MutationID    id.UUIDv7
}

func (r taskEvidenceAppendRequest) Validate() error {
	if err := errors.Join(
		r.Client.Validate(), r.Configuration.Validate(), r.Job.Validate(),
		r.Uploaded.Validate(), r.EvidenceID.Validate(), r.MutationID.Validate(),
	); err != nil {
		return err
	}
	if r.Job.Operation != operationAppendEvidence || r.Job.AppendEvidence == nil {
		return commandError("uploaded evidence append operation is invalid", nil)
	}
	return nil
}

func appendUploadedTaskEvidence(
	ctx context.Context,
	request taskEvidenceAppendRequest,
) (commandResult, error) {
	if ctx == nil {
		return commandResult{}, commandError("uploaded evidence append input is invalid", core.ErrNilContext)
	}
	if err := request.Validate(); err != nil {
		return commandResult{}, commandError("uploaded evidence append input is invalid", err)
	}
	projectID, err := request.Configuration.projectID()
	if err != nil {
		return commandResult{}, err
	}
	input := *request.Job.AppendEvidence
	summary, _ := commandEvidenceSummary(input.Summary)
	evidence, err := request.Client.AppendEvidence(ctx, taskmanager.AppendEvidenceRequest{
		ID: request.EvidenceID, ProjectID: projectID, TaskID: input.TaskID, MutationID: request.MutationID,
		Kind: input.Kind, Summary: summary, Location: request.Uploaded.Location, Digest: request.Uploaded.Digest,
		ExpectedRevision: input.ExpectedRevision,
	})
	if err != nil {
		return commandResult{}, err
	}
	if evidence.ID != request.EvidenceID || evidence.ProjectID != projectID || evidence.TaskID != input.TaskID {
		return commandResult{}, commandError("append_evidence response is not request-bound", nil)
	}
	return checkedResult(commandResult{
		Revision: commandDocumentRevisionV2, Operation: request.Job.Operation, Evidence: &evidence,
	})
}

func executeListEvidence(request executionRequest) (commandResult, error) {
	projectID, err := request.configuration.projectID()
	if err != nil {
		return commandResult{}, err
	}
	input := *request.job.ListEvidence
	page, err := request.client.ListEvidence(request.ctx, taskmanager.ListEvidenceRequest{
		ProjectID: projectID, TaskID: input.TaskID, After: input.After, Order: input.Order, Limit: input.Limit,
	})
	if err != nil {
		return commandResult{}, err
	}
	return checkedResult(commandResult{
		Revision: commandDocumentRevisionV2, Operation: request.job.Operation, EvidencePage: &page,
	})
}

func executeAppendGitCommit(request executionRequest) (commandResult, error) {
	projectID, err := request.configuration.projectID()
	if err != nil {
		return commandResult{}, err
	}
	recordID, err := freshID()
	if err != nil {
		return commandResult{}, err
	}
	mutationID, err := freshID()
	if err != nil {
		return commandResult{}, err
	}
	input := *request.job.AppendGitCommit
	repository, _ := commandRepository(input.Repository)
	summary, _ := commandCommitSummary(input.Summary)
	record, err := request.client.AppendGitCommit(request.ctx, taskmanager.AppendGitCommitRequest{
		ID: recordID, ProjectID: projectID, TaskID: input.TaskID, MutationID: mutationID,
		Repository: repository, Parent: input.Parent, Result: input.Result, Summary: summary,
		ExpectedRevision: input.ExpectedRevision,
	})
	if err != nil {
		return commandResult{}, err
	}
	if record.ID != recordID || record.ProjectID != projectID || record.TaskID != input.TaskID {
		return commandResult{}, commandError("append_git_commit response is not request-bound", nil)
	}
	return checkedResult(commandResult{
		Revision: commandDocumentRevisionV2, Operation: request.job.Operation, GitCommit: &record,
	})
}

func executeListGitCommits(request executionRequest) (commandResult, error) {
	projectID, err := request.configuration.projectID()
	if err != nil {
		return commandResult{}, err
	}
	input := *request.job.ListGitCommits
	page, err := request.client.ListGitCommits(request.ctx, taskmanager.ListGitCommitsRequest{
		ProjectID: projectID, TaskID: input.TaskID, After: input.After, Order: input.Order, Limit: input.Limit,
	})
	if err != nil {
		return commandResult{}, err
	}
	return checkedResult(commandResult{
		Revision: commandDocumentRevisionV2, Operation: request.job.Operation, GitCommits: &page,
	})
}

func checkedResult(result commandResult) (commandResult, error) {
	if err := result.Validate(); err != nil {
		return commandResult{}, err
	}
	return result, nil
}

func freshID() (id.UUIDv7, error) {
	observation, err := temporal.Observe()
	if err != nil {
		return id.UUIDv7{}, commandError("identity time observation failed", err)
	}
	size, err := core.NewByteCount(core.SecretMaterialMinimumBytes)
	if err != nil {
		return id.UUIDv7{}, commandError("identity entropy extent is invalid", err)
	}
	entropy, err := keygen.GenerateSecret(keygen.SecretRequest{Size: size})
	if err != nil {
		return id.UUIDv7{}, commandError("identity entropy generation failed", err)
	}
	defer func() { _ = entropy.Destroy() }()
	value, err := id.NewUUIDv7(id.Request{Entropy: entropy, Observation: observation})
	if err != nil {
		return id.UUIDv7{}, commandError("identity construction failed", err)
	}
	return value, nil
}

var (
	_ core.Validatable = executionRequest{}
	_ core.Validatable = taskEvidenceAppendRequest{}
)
