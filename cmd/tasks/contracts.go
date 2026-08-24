package main

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
	"github.com/deliri/primitive/v2026/id"
	"github.com/deliri/primitive/v2026/secretstore"
	"github.com/deliri/primitive/v2026/taskmanager"
)

const (
	configurationFileName                      = "task_config.json"
	commandDocumentMaxBytes                    = 64 << 10
	configurationMaxBytes                      = 16 << 10
	operationOutsidePublishedDomainDetail      = "operation is outside the published domain"
	invocationModeOutsidePublishedDomainDetail = "invocation mode is outside the published domain"
	schemaProjectionInvalidDetail              = "schema projection is invalid"
	jsonResultWriteFailedDetail                = "JSON result write failed"
	createProjectNameField                     = "create_project.name"
	createProjectDescriptionField              = "create_project.description"
	createPhaseNameField                       = "create_phase.name"
	createPhaseDescriptionField                = "create_phase.description"
	createTaskTitleField                       = "create_task.title"
	createTaskDescriptionField                 = "create_task.description"
	updateTaskChangeTitleField                 = "update_task.change.title"
	updateTaskChangeDescriptionField           = "update_task.change.description"
)

type commandDocumentRevision uint8

const (
	commandDocumentRevisionUnknown commandDocumentRevision = iota
	commandDocumentRevisionV1
	commandDocumentRevisionLimit
)

func (r commandDocumentRevision) Validate() error {
	if r != commandDocumentRevisionV1 {
		return commandError("revision must be the published revision", nil)
	}
	return nil
}

func (r commandDocumentRevision) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return strconv.AppendUint(nil, uint64(r), 10), nil
}

func (r *commandDocumentRevision) UnmarshalJSON(data []byte) error {
	if r == nil {
		return errors.Join(core.ErrJSONContract, commandError("revision receiver is nil", nil))
	}
	value, err := strconv.ParseUint(string(data), 10, 8)
	if err != nil || strconv.FormatUint(value, 10) != string(data) {
		return errors.Join(core.ErrJSONContract, commandError("revision is not canonical", err))
	}
	candidate := commandDocumentRevision(value)
	if err := candidate.Validate(); err != nil {
		return errors.Join(core.ErrJSONContract, err)
	}
	*r = candidate
	return nil
}

type operation uint8

const (
	operationUnknown operation = iota
	operationListProjects
	operationGetProject
	operationCreateProject
	operationListPhases
	operationCreatePhase
	operationListTasks
	operationGetTask
	operationCreateTask
	operationUpdateTask
	operationCompleteTask
	operationListEvidence
	operationAppendEvidence
	operationListGitCommits
	operationAppendGitCommit
	operationLimit
)

func operationNames() [operationLimit]string {
	return [...]string{
		operationListProjects:    "list_projects",
		operationGetProject:      "get_project",
		operationCreateProject:   "create_project",
		operationListPhases:      "list_phases",
		operationCreatePhase:     "create_phase",
		operationListTasks:       "list_tasks",
		operationGetTask:         "get_task",
		operationCreateTask:      "create_task",
		operationUpdateTask:      "update_task",
		operationCompleteTask:    "complete_task",
		operationListEvidence:    "list_evidence",
		operationAppendEvidence:  "append_evidence",
		operationListGitCommits:  "list_git_commits",
		operationAppendGitCommit: "append_git_commit",
	}
}

func (o operation) Validate() error {
	if o <= operationUnknown || o >= operationLimit || operationNames()[o] == "" {
		return commandError(operationOutsidePublishedDomainDetail, nil)
	}
	return nil
}

func (o operation) String() string {
	if o.Validate() != nil {
		return core.UnknownEnumDiagnostic
	}
	return operationNames()[o]
}

func (o operation) MarshalJSON() ([]byte, error) {
	if err := o.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONString(o.String())
}

func (o *operation) UnmarshalJSON(data []byte) error {
	if o == nil {
		return errors.Join(core.ErrJSONContract, commandError("operation receiver is nil", nil))
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	for candidate := operationListProjects; candidate < operationLimit; candidate++ {
		if operationNames()[candidate] == value {
			*o = candidate
			return nil
		}
	}
	return errors.Join(core.ErrJSONContract, commandError(operationOutsidePublishedDomainDetail, nil))
}

type googleSecretReference struct {
	Project string `json:"project"`
	Secret  string `json:"secret"`
}

func (r googleSecretReference) Validate() error {
	_, projectErr := secretstore.ParseGoogleProjectID(r.Project)
	_, secretErr := secretstore.ParseGoogleSecretID(r.Secret)
	if err := errors.Join(projectErr, secretErr); err != nil {
		return commandError("password_secret is invalid", err)
	}
	return nil
}

func (r googleSecretReference) accessRequest() (secretstore.AccessRequest, error) {
	if err := r.Validate(); err != nil {
		return secretstore.AccessRequest{}, err
	}
	project, _ := secretstore.ParseGoogleProjectID(r.Project)
	secret, _ := secretstore.ParseGoogleSecretID(r.Secret)
	request := secretstore.AccessRequest{
		Project: project, Secret: secret, Version: secretstore.GoogleVersionSelectorLatest,
	}
	if err := request.Validate(); err != nil {
		return secretstore.AccessRequest{}, commandError("password_secret cannot be projected", err)
	}
	return request, nil
}

type configurationDocument struct {
	ProjectID      *id.UUIDv7                          `json:"project_id,omitempty"`
	PasswordSecret googleSecretReference               `json:"password_secret"`
	Username       exchange.BasicAuthorizationIdentity `json:"username"`
	Authority      core.HTTPEndpoint                   `json:"authority"`
	Revision       commandDocumentRevision             `json:"revision"`
}

func (d configurationDocument) Validate() error {
	if err := errors.Join(d.Revision.Validate(), d.Authority.Validate(), d.Username.Validate(), d.PasswordSecret.Validate()); err != nil {
		return commandError("task configuration is invalid", err)
	}
	url := d.Authority.HTTPURL()
	if url.Scheme != core.SchemeHTTPS || url.Path != "" || url.RawPath != "" ||
		url.RawQuery != "" || url.ForceQuery || strings.TrimSpace(d.Username.String()) != d.Username.String() {
		return commandError("task configuration authority or username is invalid", nil)
	}
	if d.ProjectID != nil {
		if err := d.ProjectID.Validate(); err != nil {
			return commandError("project_id is invalid", err)
		}
	}
	return nil
}

func (d configurationDocument) projectID() (id.UUIDv7, error) {
	if err := d.Validate(); err != nil {
		return id.UUIDv7{}, err
	}
	if d.ProjectID == nil {
		return id.UUIDv7{}, commandError("project_id is required for this operation", nil)
	}
	return *d.ProjectID, nil
}

func (d configurationDocument) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	type wire configurationDocument
	return core.MarshalCanonicalJSONDocument(wire(d))
}

type listProjectsInput struct {
	After     *taskmanager.ProjectCursor   `json:"after,omitempty"`
	Limit     taskmanager.PageLimit        `json:"limit"`
	Lifecycle taskmanager.ProjectLifecycle `json:"lifecycle"`
	Order     taskmanager.PageOrder        `json:"order"`
}

func (i listProjectsInput) Validate() error {
	return taskmanager.ListProjectsRequest{Lifecycle: i.Lifecycle, Order: i.Order, Limit: i.Limit, After: i.After}.Validate()
}

type createProjectInput struct {
	Name        string                       `json:"name"`
	Description string                       `json:"description"`
	Lifecycle   taskmanager.ProjectLifecycle `json:"lifecycle"`
}

func (i createProjectInput) Validate() error {
	_, titleErr := commandTitle(createProjectNameField, i.Name)
	_, descriptionErr := commandDescription(createProjectDescriptionField, i.Description)
	return errors.Join(titleErr, descriptionErr, i.Lifecycle.Validate())
}

type listPhasesInput struct {
	After *taskmanager.PhaseCursor `json:"after,omitempty"`
	Limit taskmanager.PageLimit    `json:"limit"`
	Order taskmanager.PageOrder    `json:"order"`
}

func (i listPhasesInput) Validate() error {
	var afterErr error
	if i.After != nil {
		afterErr = i.After.Validate()
	}
	return errors.Join(i.Limit.Validate(), i.Order.Validate(), afterErr)
}

type createPhaseInput struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Position    uint64 `json:"position"`
}

func (i createPhaseInput) Validate() error {
	_, titleErr := commandTitle(createPhaseNameField, i.Name)
	_, descriptionErr := commandDescription(createPhaseDescriptionField, i.Description)
	return errors.Join(titleErr, descriptionErr)
}

type listTasksInput struct {
	PhaseID    *id.UUIDv7                 `json:"phase_id,omitempty"`
	After      *taskmanager.TaskCursor    `json:"after,omitempty"`
	Collection taskmanager.TaskCollection `json:"collection"`
	Order      taskmanager.PageOrder      `json:"order"`
	Limit      taskmanager.PageLimit      `json:"limit"`
}

func (i listTasksInput) Validate() error {
	var phaseErr, afterErr error
	if i.PhaseID != nil {
		phaseErr = i.PhaseID.Validate()
	}
	if i.After != nil {
		afterErr = i.After.Validate()
	}
	return errors.Join(phaseErr, afterErr, i.Collection.Validate(), i.Order.Validate(), i.Limit.Validate())
}

type createTaskInput struct {
	Title       string                `json:"title"`
	Description string                `json:"description"`
	PhaseID     id.UUIDv7             `json:"phase_id"`
	Kind        taskmanager.TaskKind  `json:"kind"`
	State       taskmanager.TaskState `json:"state"`
}

type getTaskInput struct {
	TaskID id.UUIDv7 `json:"task_id"`
}

func (i getTaskInput) Validate() error { return i.TaskID.Validate() }

func (i createTaskInput) Validate() error {
	_, titleErr := commandTitle(createTaskTitleField, i.Title)
	_, descriptionErr := commandDescription(createTaskDescriptionField, i.Description)
	return errors.Join(i.PhaseID.Validate(), i.Kind.Validate(), i.State.Validate(), titleErr, descriptionErr)
}

type taskChangeInput struct {
	Title       *string                `json:"title,omitempty"`
	Description *string                `json:"description,omitempty"`
	Kind        *taskmanager.TaskKind  `json:"kind,omitempty"`
	State       *taskmanager.TaskState `json:"state,omitempty"`
}

func (i taskChangeInput) Validate() error {
	if i.Title == nil && i.Description == nil && i.Kind == nil && i.State == nil {
		return commandError("update_task.change must contain at least one field", nil)
	}
	var titleErr, descriptionErr, kindErr, stateErr error
	if i.Title != nil {
		_, titleErr = commandTitle(updateTaskChangeTitleField, *i.Title)
	}
	if i.Description != nil {
		_, descriptionErr = commandDescription(updateTaskChangeDescriptionField, *i.Description)
	}
	if i.Kind != nil {
		kindErr = i.Kind.Validate()
	}
	if i.State != nil {
		stateErr = i.State.Validate()
	}
	return errors.Join(titleErr, descriptionErr, kindErr, stateErr)
}

type updateTaskInput struct {
	Change           taskChangeInput      `json:"change"`
	ExpectedRevision taskmanager.Revision `json:"expected_revision"`
	TaskID           id.UUIDv7            `json:"task_id"`
}

func (i updateTaskInput) Validate() error {
	return errors.Join(i.TaskID.Validate(), i.ExpectedRevision.Validate(), i.Change.Validate())
}

type completeTaskInput struct {
	TaskID           id.UUIDv7            `json:"task_id"`
	ExpectedRevision taskmanager.Revision `json:"expected_revision"`
}

type listEvidenceInput struct {
	After  *taskmanager.EvidenceCursor `json:"after,omitempty"`
	Limit  taskmanager.PageLimit       `json:"limit"`
	TaskID id.UUIDv7                   `json:"task_id"`
	Order  taskmanager.PageOrder       `json:"order"`
}

func (i listEvidenceInput) Validate() error {
	var afterErr error
	if i.After != nil {
		afterErr = i.After.Validate()
	}
	return errors.Join(i.TaskID.Validate(), afterErr, i.Order.Validate(), i.Limit.Validate())
}

func (i completeTaskInput) Validate() error {
	return errors.Join(i.TaskID.Validate(), i.ExpectedRevision.Validate())
}

type appendEvidenceInput struct {
	Summary          string                   `json:"summary"`
	Location         core.HTTPEndpoint        `json:"location"`
	ExpectedRevision taskmanager.Revision     `json:"expected_revision"`
	Digest           core.SHA256Digest        `json:"digest"`
	TaskID           id.UUIDv7                `json:"task_id"`
	Kind             taskmanager.EvidenceKind `json:"kind"`
}

func (i appendEvidenceInput) Validate() error {
	_, summaryErr := commandEvidenceSummary(i.Summary)
	url := i.Location.HTTPURL()
	var locationPolicyErr error
	if url.Scheme != core.SchemeHTTPS {
		locationPolicyErr = commandError("append_evidence.location must use HTTPS", nil)
	}
	return errors.Join(
		i.TaskID.Validate(), i.Kind.Validate(), summaryErr, i.Location.Validate(),
		locationPolicyErr, i.Digest.Validate(), i.ExpectedRevision.Validate(),
	)
}

type appendGitCommitInput struct {
	Repository       string               `json:"repository"`
	Summary          string               `json:"summary"`
	ExpectedRevision taskmanager.Revision `json:"expected_revision"`
	Parent           core.BuildCommit     `json:"parent"`
	Result           core.BuildCommit     `json:"result"`
	TaskID           id.UUIDv7            `json:"task_id"`
}

type listGitCommitsInput struct {
	After  *taskmanager.GitCommitCursor `json:"after,omitempty"`
	Limit  taskmanager.PageLimit        `json:"limit"`
	TaskID id.UUIDv7                    `json:"task_id"`
	Order  taskmanager.PageOrder        `json:"order"`
}

func (i listGitCommitsInput) Validate() error {
	var afterErr error
	if i.After != nil {
		afterErr = i.After.Validate()
	}
	return errors.Join(i.TaskID.Validate(), afterErr, i.Order.Validate(), i.Limit.Validate())
}

func (i appendGitCommitInput) Validate() error {
	_, repositoryErr := commandRepository(i.Repository)
	_, summaryErr := commandCommitSummary(i.Summary)
	if i.Parent == i.Result {
		return commandError("append_git_commit parent and result must differ", nil)
	}
	return errors.Join(
		i.TaskID.Validate(), repositoryErr, i.Parent.Validate(), i.Result.Validate(),
		summaryErr, i.ExpectedRevision.Validate(),
	)
}

type jobDocument struct {
	ListTasks       *listTasksInput         `json:"list_tasks,omitempty"`
	CompleteTask    *completeTaskInput      `json:"complete_task,omitempty"`
	ListProjects    *listProjectsInput      `json:"list_projects,omitempty"`
	CreateProject   *createProjectInput     `json:"create_project,omitempty"`
	ListPhases      *listPhasesInput        `json:"list_phases,omitempty"`
	CreatePhase     *createPhaseInput       `json:"create_phase,omitempty"`
	AppendGitCommit *appendGitCommitInput   `json:"append_git_commit,omitempty"`
	CreateTask      *createTaskInput        `json:"create_task,omitempty"`
	ListGitCommits  *listGitCommitsInput    `json:"list_git_commits,omitempty"`
	UpdateTask      *updateTaskInput        `json:"update_task,omitempty"`
	GetTask         *getTaskInput           `json:"get_task,omitempty"`
	ListEvidence    *listEvidenceInput      `json:"list_evidence,omitempty"`
	AppendEvidence  *appendEvidenceInput    `json:"append_evidence,omitempty"`
	Revision        commandDocumentRevision `json:"revision"`
	Operation       operation               `json:"operation"`
}

func (d jobDocument) Validate() error {
	if err := errors.Join(d.Revision.Validate(), d.Operation.Validate()); err != nil {
		return commandError("job document is invalid", err)
	}
	if d.payloadCount() != operationPayloadCounts()[d.Operation] {
		return commandError("job document payload count does not match its operation", nil)
	}
	return d.validateSelectedPayload()
}

func (d jobDocument) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	type wire jobDocument
	return core.MarshalCanonicalJSONDocument(wire(d))
}

func (d jobDocument) payloadCount() int {
	values := [...]bool{
		d.ListProjects != nil, d.CreateProject != nil,
		d.ListPhases != nil, d.CreatePhase != nil, d.ListTasks != nil,
		d.GetTask != nil, d.CreateTask != nil, d.UpdateTask != nil,
		d.CompleteTask != nil, d.ListEvidence != nil, d.AppendEvidence != nil,
		d.ListGitCommits != nil, d.AppendGitCommit != nil,
	}
	count := 0
	for _, present := range values {
		if present {
			count++
		}
	}
	return count
}

func (d jobDocument) validateSelectedPayload() error {
	if err := d.Operation.Validate(); err != nil {
		return err
	}
	validator := jobPayloadValidators()[d.Operation]
	if validator == nil {
		return commandError(operationOutsidePublishedDomainDetail, nil)
	}
	return validator(d)
}

type jobPayloadValidator func(jobDocument) error

func jobPayloadValidators() [operationLimit]jobPayloadValidator {
	return [...]jobPayloadValidator{
		operationListProjects:    func(d jobDocument) error { return validatePayload(d.ListProjects) },
		operationGetProject:      validateNoJobPayload,
		operationCreateProject:   func(d jobDocument) error { return validatePayload(d.CreateProject) },
		operationListPhases:      func(d jobDocument) error { return validatePayload(d.ListPhases) },
		operationCreatePhase:     func(d jobDocument) error { return validatePayload(d.CreatePhase) },
		operationListTasks:       func(d jobDocument) error { return validatePayload(d.ListTasks) },
		operationGetTask:         func(d jobDocument) error { return validatePayload(d.GetTask) },
		operationCreateTask:      func(d jobDocument) error { return validatePayload(d.CreateTask) },
		operationUpdateTask:      func(d jobDocument) error { return validatePayload(d.UpdateTask) },
		operationCompleteTask:    func(d jobDocument) error { return validatePayload(d.CompleteTask) },
		operationListEvidence:    func(d jobDocument) error { return validatePayload(d.ListEvidence) },
		operationAppendEvidence:  func(d jobDocument) error { return validatePayload(d.AppendEvidence) },
		operationListGitCommits:  func(d jobDocument) error { return validatePayload(d.ListGitCommits) },
		operationAppendGitCommit: func(d jobDocument) error { return validatePayload(d.AppendGitCommit) },
	}
}

func operationPayloadCounts() [operationLimit]int {
	counts := [operationLimit]int{}
	for candidate := operationListProjects; candidate < operationLimit; candidate++ {
		counts[candidate] = 1
	}
	counts[operationGetProject] = 0
	return counts
}

func validateNoJobPayload(jobDocument) error { return nil }

func validateJobConfiguration(configuration configurationDocument, job jobDocument) error {
	if err := errors.Join(configuration.Validate(), job.Validate()); err != nil {
		return commandError("configuration and job are invalid", err)
	}
	return configurationValidators()[job.Operation](configuration, job)
}

type configurationValidator func(configurationDocument, jobDocument) error

func configurationValidators() [operationLimit]configurationValidator {
	return [...]configurationValidator{
		operationListProjects:    validateUnscopedConfiguration,
		operationGetProject:      validateProjectConfiguration,
		operationCreateProject:   validateUnscopedConfiguration,
		operationListPhases:      validatePhasePageConfiguration,
		operationCreatePhase:     validateProjectConfiguration,
		operationListTasks:       validateTaskPageConfiguration,
		operationGetTask:         validateProjectConfiguration,
		operationCreateTask:      validateProjectConfiguration,
		operationUpdateTask:      validateProjectConfiguration,
		operationCompleteTask:    validateProjectConfiguration,
		operationListEvidence:    validateEvidencePageConfiguration,
		operationAppendEvidence:  validateProjectConfiguration,
		operationListGitCommits:  validateGitPageConfiguration,
		operationAppendGitCommit: validateProjectConfiguration,
	}
}

func validateUnscopedConfiguration(configurationDocument, jobDocument) error { return nil }

func validateProjectConfiguration(configuration configurationDocument, _ jobDocument) error {
	_, err := configuration.projectID()
	return err
}

func validatePhasePageConfiguration(configuration configurationDocument, job jobDocument) error {
	projectID, err := configuration.projectID()
	if err != nil || job.ListPhases.After == nil || job.ListPhases.After.ProjectID == projectID {
		return err
	}
	return commandError("list_phases.after belongs to another project", nil)
}

func validateTaskPageConfiguration(configuration configurationDocument, job jobDocument) error {
	projectID, err := configuration.projectID()
	if err != nil || job.ListTasks.After == nil || job.ListTasks.After.ProjectID == projectID {
		return err
	}
	return commandError("list_tasks.after belongs to another project", nil)
}

func validateEvidencePageConfiguration(configuration configurationDocument, job jobDocument) error {
	projectID, err := configuration.projectID()
	if err != nil || job.ListEvidence.After == nil {
		return err
	}
	if job.ListEvidence.After.ProjectID != projectID || job.ListEvidence.After.TaskID != job.ListEvidence.TaskID {
		return commandError("list_evidence.after belongs to another task", nil)
	}
	return nil
}

func validateGitPageConfiguration(configuration configurationDocument, job jobDocument) error {
	projectID, err := configuration.projectID()
	if err != nil || job.ListGitCommits.After == nil {
		return err
	}
	if job.ListGitCommits.After.ProjectID != projectID || job.ListGitCommits.After.TaskID != job.ListGitCommits.TaskID {
		return commandError("list_git_commits.after belongs to another task", nil)
	}
	return nil
}

func validatePayload[T interface{ Validate() error }](payload *T) error {
	if payload == nil {
		return commandError("operation does not match its payload", nil)
	}
	if err := (*payload).Validate(); err != nil {
		return commandError("operation payload is invalid", err)
	}
	return nil
}

func commandTitle(field, value string) (taskmanager.Title, error) {
	value = strings.TrimSpace(value)
	parsed, err := taskmanager.ParseTitle(value)
	return parsed, commandTextError(commandTextErrorRequest{
		field: field, value: value, maximum: taskmanager.TitleMaximumRunes, cause: err,
	})
}

func commandDescription(field, value string) (taskmanager.Description, error) {
	value = strings.TrimSpace(value)
	parsed, err := taskmanager.ParseDescription(value)
	return parsed, commandTextError(commandTextErrorRequest{
		field: field, value: value, maximum: taskmanager.DescriptionMaximumRunes, cause: err,
	})
}

func commandEvidenceSummary(value string) (taskmanager.EvidenceSummary, error) {
	value = strings.TrimSpace(value)
	parsed, err := taskmanager.ParseEvidenceSummary(value)
	return parsed, commandTextError(commandTextErrorRequest{
		field: "append_evidence.summary", value: value, maximum: taskmanager.EvidenceSummaryMaximumRunes, cause: err,
	})
}

func commandRepository(value string) (taskmanager.RepositoryIdentity, error) {
	value = strings.TrimSpace(value)
	parsed, err := taskmanager.ParseRepositoryIdentity(value)
	return parsed, commandTextError(commandTextErrorRequest{
		field: "append_git_commit.repository", value: value, maximum: taskmanager.RepositoryMaximumRunes, cause: err,
	})
}

func commandCommitSummary(value string) (taskmanager.CommitSummary, error) {
	value = strings.TrimSpace(value)
	parsed, err := taskmanager.ParseCommitSummary(value)
	return parsed, commandTextError(commandTextErrorRequest{
		field: "append_git_commit.summary", value: value, maximum: taskmanager.CommitSummaryMaximumRunes, cause: err,
	})
}

type commandTextErrorRequest struct {
	cause   error
	field   string
	value   string
	maximum int
}

func commandTextError(request commandTextErrorRequest) error {
	if request.cause == nil {
		return nil
	}
	return commandError(
		fmt.Sprintf(
			"%s is invalid: runes=%d maximum=%d",
			request.field,
			utf8.RuneCountInString(request.value),
			request.maximum,
		),
		request.cause,
	)
}

func commandError(detail string, cause error) error {
	if cause == nil {
		return errors.Join(core.ErrTaskManagerContract, errors.New(detail))
	}
	return errors.Join(core.ErrTaskManagerContract, fmt.Errorf("%s: %w", detail, cause))
}

var (
	_ core.ValidatedJSONMarshaler = commandDocumentRevisionUnknown
	_ core.ValidatedJSONMarshaler = operationUnknown
	_ core.ValidatedJSONMarshaler = configurationDocument{}
	_ core.ValidatedJSONMarshaler = jobDocument{}
)
