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

func executeJob(
	ctx context.Context,
	client taskmanager.Client,
	configuration configurationDocument,
	job jobDocument,
) (commandResult, error) {
	if err := errors.Join(client.Validate(), configuration.Validate(), job.Validate()); err != nil {
		return commandResult{}, commandError("command execution input is invalid", err)
	}
	executor := jobExecutors()[job.Operation]
	if executor == nil {
		return commandResult{}, commandError("operation is outside the published domain", nil)
	}
	return executor(executionRequest{
		ctx: ctx, client: client, configuration: configuration, job: job,
	})
}

type executionRequest struct {
	configuration configurationDocument
	job           jobDocument
	ctx           context.Context
	client        taskmanager.Client
}

type jobExecutor func(executionRequest) (commandResult, error)

func jobExecutors() [operationLimit]jobExecutor {
	return [...]jobExecutor{
		operationListProjects: func(r executionRequest) (commandResult, error) {
			return executeListProjects(r.ctx, r.client, r.job)
		},
		operationGetProject: func(r executionRequest) (commandResult, error) {
			return executeGetProject(r.ctx, r.client, r.configuration, r.job)
		},
		operationCreateProject: func(r executionRequest) (commandResult, error) {
			return executeCreateProject(r.ctx, r.client, r.job)
		},
		operationListPhases: func(r executionRequest) (commandResult, error) {
			return executeListPhases(r.ctx, r.client, r.configuration, r.job)
		},
		operationCreatePhase: func(r executionRequest) (commandResult, error) {
			return executeCreatePhase(r.ctx, r.client, r.configuration, r.job)
		},
		operationListTasks: func(r executionRequest) (commandResult, error) {
			return executeListTasks(r.ctx, r.client, r.configuration, r.job)
		},
		operationGetTask: func(r executionRequest) (commandResult, error) {
			return executeGetTask(r.ctx, r.client, r.configuration, r.job)
		},
		operationCreateTask: func(r executionRequest) (commandResult, error) {
			return executeCreateTask(r.ctx, r.client, r.configuration, r.job)
		},
		operationUpdateTask: func(r executionRequest) (commandResult, error) {
			return executeUpdateTask(r.ctx, r.client, r.configuration, r.job)
		},
		operationCompleteTask: func(r executionRequest) (commandResult, error) {
			return executeCompleteTask(r.ctx, r.client, r.configuration, r.job)
		},
		operationListEvidence: func(r executionRequest) (commandResult, error) {
			return executeListEvidence(r.ctx, r.client, r.configuration, r.job)
		},
		operationAppendEvidence: func(r executionRequest) (commandResult, error) {
			return executeAppendEvidence(r.ctx, r.client, r.configuration, r.job)
		},
		operationListGitCommits: func(r executionRequest) (commandResult, error) {
			return executeListGitCommits(r.ctx, r.client, r.configuration, r.job)
		},
		operationAppendGitCommit: func(r executionRequest) (commandResult, error) {
			return executeAppendGitCommit(r.ctx, r.client, r.configuration, r.job)
		},
	}
}

func executeGetProject(
	ctx context.Context,
	client taskmanager.Client,
	configuration configurationDocument,
	job jobDocument,
) (commandResult, error) {
	projectID, err := configuration.projectID()
	if err != nil {
		return commandResult{}, err
	}
	project, err := client.GetProject(ctx, taskmanager.GetProjectRequest{ProjectID: projectID})
	if err != nil {
		return commandResult{}, err
	}
	return checkedResult(commandResult{
		Revision: commandDocumentRevisionV1, Operation: job.Operation, Project: &project,
	})
}

func executeListProjects(ctx context.Context, client taskmanager.Client, job jobDocument) (commandResult, error) {
	input := *job.ListProjects
	page, err := client.ListProjects(ctx, taskmanager.ListProjectsRequest{
		Lifecycle: input.Lifecycle, Order: input.Order, Limit: input.Limit, After: input.After,
	})
	if err != nil {
		return commandResult{}, err
	}
	return checkedResult(commandResult{
		Revision: commandDocumentRevisionV1, Operation: job.Operation, Projects: &page,
	})
}

func executeCreateProject(ctx context.Context, client taskmanager.Client, job jobDocument) (commandResult, error) {
	input := *job.CreateProject
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
	project, err := client.CreateProject(ctx, taskmanager.CreateProjectRequest{
		ID: projectID, MutationID: mutationID, Name: name, Description: description, Lifecycle: input.Lifecycle,
	})
	if err != nil {
		return commandResult{}, err
	}
	if project.Summary.ID != projectID {
		return commandResult{}, commandError("create_project response identity is not request-bound", nil)
	}
	return checkedResult(commandResult{
		Revision: commandDocumentRevisionV1, Operation: job.Operation, Project: &project,
	})
}

func executeListPhases(
	ctx context.Context,
	client taskmanager.Client,
	configuration configurationDocument,
	job jobDocument,
) (commandResult, error) {
	projectID, err := configuration.projectID()
	if err != nil {
		return commandResult{}, err
	}
	input := *job.ListPhases
	page, err := client.ListPhases(ctx, taskmanager.ListPhasesRequest{
		ProjectID: projectID, Order: input.Order, Limit: input.Limit, After: input.After,
	})
	if err != nil {
		return commandResult{}, err
	}
	return checkedResult(commandResult{
		Revision: commandDocumentRevisionV1, Operation: job.Operation, Phases: &page,
	})
}

func executeCreatePhase(
	ctx context.Context,
	client taskmanager.Client,
	configuration configurationDocument,
	job jobDocument,
) (commandResult, error) {
	projectID, err := configuration.projectID()
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
	input := *job.CreatePhase
	name, _ := commandTitle(createPhaseNameField, input.Name)
	description, _ := commandDescription(createPhaseDescriptionField, input.Description)
	phase, err := client.CreatePhase(ctx, taskmanager.CreatePhaseRequest{
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
		Revision: commandDocumentRevisionV1, Operation: job.Operation, Phase: &phase,
	})
}

func executeListTasks(
	ctx context.Context,
	client taskmanager.Client,
	configuration configurationDocument,
	job jobDocument,
) (commandResult, error) {
	projectID, err := configuration.projectID()
	if err != nil {
		return commandResult{}, err
	}
	input := *job.ListTasks
	page, err := client.ListTasks(ctx, taskmanager.ListTasksRequest{
		ProjectID: projectID, PhaseID: input.PhaseID, After: input.After,
		Collection: input.Collection, Order: input.Order, Limit: input.Limit,
	})
	if err != nil {
		return commandResult{}, err
	}
	return checkedResult(commandResult{
		Revision: commandDocumentRevisionV1, Operation: job.Operation, Tasks: &page,
	})
}

func executeGetTask(
	ctx context.Context,
	client taskmanager.Client,
	configuration configurationDocument,
	job jobDocument,
) (commandResult, error) {
	projectID, err := configuration.projectID()
	if err != nil {
		return commandResult{}, err
	}
	input := *job.GetTask
	task, err := client.GetTask(ctx, taskmanager.GetTaskRequest{ProjectID: projectID, TaskID: input.TaskID})
	if err != nil {
		return commandResult{}, err
	}
	return checkedResult(commandResult{
		Revision: commandDocumentRevisionV1, Operation: job.Operation, TaskDetail: &task,
	})
}

func executeCreateTask(
	ctx context.Context,
	client taskmanager.Client,
	configuration configurationDocument,
	job jobDocument,
) (commandResult, error) {
	projectID, err := configuration.projectID()
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
	input := *job.CreateTask
	title, _ := commandTitle(createTaskTitleField, input.Title)
	description, _ := commandDescription(createTaskDescriptionField, input.Description)
	task, err := client.CreateTask(ctx, taskmanager.CreateTaskRequest{
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
		Revision: commandDocumentRevisionV1, Operation: job.Operation, TaskDetail: &task,
	})
}

func executeUpdateTask(
	ctx context.Context,
	client taskmanager.Client,
	configuration configurationDocument,
	job jobDocument,
) (commandResult, error) {
	projectID, err := configuration.projectID()
	if err != nil {
		return commandResult{}, err
	}
	mutationID, err := freshID()
	if err != nil {
		return commandResult{}, err
	}
	input := *job.UpdateTask
	change, err := input.Change.taskChange()
	if err != nil {
		return commandResult{}, err
	}
	task, err := client.UpdateTask(ctx, taskmanager.UpdateTaskRequest{
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
		Revision: commandDocumentRevisionV1, Operation: job.Operation, TaskDetail: &task,
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

func executeCompleteTask(
	ctx context.Context,
	client taskmanager.Client,
	configuration configurationDocument,
	job jobDocument,
) (commandResult, error) {
	projectID, err := configuration.projectID()
	if err != nil {
		return commandResult{}, err
	}
	mutationID, err := freshID()
	if err != nil {
		return commandResult{}, err
	}
	input := *job.CompleteTask
	task, err := client.CompleteTask(ctx, taskmanager.CompleteTaskRequest{
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
		Revision: commandDocumentRevisionV1, Operation: job.Operation, Task: &task,
	})
}

func executeAppendEvidence(
	ctx context.Context,
	client taskmanager.Client,
	configuration configurationDocument,
	job jobDocument,
) (commandResult, error) {
	projectID, err := configuration.projectID()
	if err != nil {
		return commandResult{}, err
	}
	evidenceID, err := freshID()
	if err != nil {
		return commandResult{}, err
	}
	mutationID, err := freshID()
	if err != nil {
		return commandResult{}, err
	}
	input := *job.AppendEvidence
	summary, _ := commandEvidenceSummary(input.Summary)
	evidence, err := client.AppendEvidence(ctx, taskmanager.AppendEvidenceRequest{
		ID: evidenceID, ProjectID: projectID, TaskID: input.TaskID, MutationID: mutationID,
		Kind: input.Kind, Summary: summary, Location: input.Location, Digest: input.Digest,
		ExpectedRevision: input.ExpectedRevision,
	})
	if err != nil {
		return commandResult{}, err
	}
	if evidence.ID != evidenceID || evidence.ProjectID != projectID || evidence.TaskID != input.TaskID {
		return commandResult{}, commandError("append_evidence response is not request-bound", nil)
	}
	return checkedResult(commandResult{
		Revision: commandDocumentRevisionV1, Operation: job.Operation, Evidence: &evidence,
	})
}

func executeListEvidence(
	ctx context.Context,
	client taskmanager.Client,
	configuration configurationDocument,
	job jobDocument,
) (commandResult, error) {
	projectID, err := configuration.projectID()
	if err != nil {
		return commandResult{}, err
	}
	input := *job.ListEvidence
	page, err := client.ListEvidence(ctx, taskmanager.ListEvidenceRequest{
		ProjectID: projectID, TaskID: input.TaskID, After: input.After, Order: input.Order, Limit: input.Limit,
	})
	if err != nil {
		return commandResult{}, err
	}
	return checkedResult(commandResult{
		Revision: commandDocumentRevisionV1, Operation: job.Operation, EvidencePage: &page,
	})
}

func executeAppendGitCommit(
	ctx context.Context,
	client taskmanager.Client,
	configuration configurationDocument,
	job jobDocument,
) (commandResult, error) {
	projectID, err := configuration.projectID()
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
	input := *job.AppendGitCommit
	repository, _ := commandRepository(input.Repository)
	summary, _ := commandCommitSummary(input.Summary)
	record, err := client.AppendGitCommit(ctx, taskmanager.AppendGitCommitRequest{
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
		Revision: commandDocumentRevisionV1, Operation: job.Operation, GitCommit: &record,
	})
}

func executeListGitCommits(
	ctx context.Context,
	client taskmanager.Client,
	configuration configurationDocument,
	job jobDocument,
) (commandResult, error) {
	projectID, err := configuration.projectID()
	if err != nil {
		return commandResult{}, err
	}
	input := *job.ListGitCommits
	page, err := client.ListGitCommits(ctx, taskmanager.ListGitCommitsRequest{
		ProjectID: projectID, TaskID: input.TaskID, After: input.After, Order: input.Order, Limit: input.Limit,
	})
	if err != nil {
		return commandResult{}, err
	}
	return checkedResult(commandResult{
		Revision: commandDocumentRevisionV1, Operation: job.Operation, GitCommits: &page,
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
