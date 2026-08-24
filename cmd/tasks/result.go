package main

import (
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/taskmanager"
)

type commandResult struct {
	Tasks        *taskmanager.TaskPage        `json:"tasks,omitempty"`
	Projects     *taskmanager.ProjectPage     `json:"projects,omitempty"`
	Project      *taskmanager.ProjectDetail   `json:"project,omitempty"`
	Phases       *taskmanager.PhasePage       `json:"phases,omitempty"`
	Phase        *taskmanager.PhaseSummary    `json:"phase,omitempty"`
	Task         *taskmanager.TaskSummary     `json:"task,omitempty"`
	TaskDetail   *taskmanager.TaskDetail      `json:"task_detail,omitempty"`
	EvidencePage *taskmanager.EvidencePage    `json:"evidence_page,omitempty"`
	Evidence     *taskmanager.EvidenceRecord  `json:"evidence,omitempty"`
	GitCommits   *taskmanager.GitCommitPage   `json:"git_commits,omitempty"`
	GitCommit    *taskmanager.GitCommitRecord `json:"git_commit,omitempty"`
	Operation    operation                    `json:"operation"`
	Revision     commandDocumentRevision      `json:"revision"`
}

func (r commandResult) Validate() error {
	if err := r.Revision.Validate(); err != nil {
		return err
	}
	if err := r.Operation.Validate(); err != nil {
		return err
	}
	if r.payloadCount() != 1 {
		return commandError("result must contain exactly one payload", nil)
	}
	validator := resultPayloadValidators()[r.Operation]
	if validator == nil {
		return commandError("result operation is outside the published domain", nil)
	}
	return validator(r)
}

type resultPayloadValidator func(commandResult) error

func resultPayloadValidators() [operationLimit]resultPayloadValidator {
	return [...]resultPayloadValidator{
		operationListProjects:  func(r commandResult) error { return validatePayload(r.Projects) },
		operationGetProject:    func(r commandResult) error { return validatePayload(r.Project) },
		operationCreateProject: func(r commandResult) error { return validatePayload(r.Project) },
		operationListPhases:    func(r commandResult) error { return validatePayload(r.Phases) },
		operationCreatePhase:   func(r commandResult) error { return validatePayload(r.Phase) },
		operationListTasks:     func(r commandResult) error { return validatePayload(r.Tasks) },
		operationGetTask:       func(r commandResult) error { return validatePayload(r.TaskDetail) },
		operationCreateTask:    func(r commandResult) error { return validatePayload(r.TaskDetail) },
		operationUpdateTask:    func(r commandResult) error { return validatePayload(r.TaskDetail) },
		operationCompleteTask:  func(r commandResult) error { return validatePayload(r.Task) },
		operationListEvidence:  func(r commandResult) error { return validatePayload(r.EvidencePage) },
		operationAppendEvidence: func(r commandResult) error {
			return validatePayload(r.Evidence)
		},
		operationListGitCommits: func(r commandResult) error { return validatePayload(r.GitCommits) },
		operationAppendGitCommit: func(r commandResult) error {
			return validatePayload(r.GitCommit)
		},
	}
}

func (r commandResult) payloadCount() int {
	values := [...]bool{
		r.Projects != nil, r.Project != nil, r.Phases != nil, r.Phase != nil,
		r.Tasks != nil, r.Task != nil, r.TaskDetail != nil, r.EvidencePage != nil,
		r.Evidence != nil, r.GitCommits != nil, r.GitCommit != nil,
	}
	count := 0
	for _, present := range values {
		if present {
			count++
		}
	}
	return count
}

func (r commandResult) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, err
	}
	type wire commandResult
	return core.MarshalCanonicalJSONDocument(wire(r))
}

type schemaDocument struct {
	ConfigurationFile           string                  `json:"configuration_file"`
	Operations                  []operation             `json:"operations"`
	ConfigurationMaximumBytes   uint64                  `json:"configuration_maximum_bytes"`
	JobMaximumBytes             uint64                  `json:"job_maximum_bytes"`
	TitleMaximumRunes           uint64                  `json:"title_maximum_runes"`
	DescriptionMaximumRunes     uint64                  `json:"description_maximum_runes"`
	EvidenceSummaryMaximumRunes uint64                  `json:"evidence_summary_maximum_runes"`
	RepositoryMaximumRunes      uint64                  `json:"repository_maximum_runes"`
	CommitSummaryMaximumRunes   uint64                  `json:"commit_summary_maximum_runes"`
	PageLimitMaximum            taskmanager.PageLimit   `json:"page_limit_maximum"`
	Revision                    commandDocumentRevision `json:"revision"`
}

func currentSchema() schemaDocument {
	operations := make([]operation, 0, operationLimit-operationListProjects)
	for candidate := operationListProjects; candidate < operationLimit; candidate++ {
		operations = append(operations, candidate)
	}
	return schemaDocument{
		Revision:                    commandDocumentRevisionV1,
		ConfigurationFile:           configurationFileName,
		ConfigurationMaximumBytes:   configurationMaxBytes,
		JobMaximumBytes:             commandDocumentMaxBytes,
		PageLimitMaximum:            taskmanager.PageLimitMaximum,
		TitleMaximumRunes:           taskmanager.TitleMaximumRunes,
		DescriptionMaximumRunes:     taskmanager.DescriptionMaximumRunes,
		EvidenceSummaryMaximumRunes: taskmanager.EvidenceSummaryMaximumRunes,
		RepositoryMaximumRunes:      taskmanager.RepositoryMaximumRunes,
		CommitSummaryMaximumRunes:   taskmanager.CommitSummaryMaximumRunes,
		Operations:                  operations,
	}
}

func (d schemaDocument) Validate() error {
	if err := d.Revision.Validate(); err != nil {
		return commandError(schemaProjectionInvalidDetail, err)
	}
	facts := [...]bool{
		d.ConfigurationFile == configurationFileName,
		d.ConfigurationMaximumBytes == configurationMaxBytes,
		d.JobMaximumBytes == commandDocumentMaxBytes,
		d.PageLimitMaximum == taskmanager.PageLimitMaximum,
		d.TitleMaximumRunes == taskmanager.TitleMaximumRunes,
		d.DescriptionMaximumRunes == taskmanager.DescriptionMaximumRunes,
		d.EvidenceSummaryMaximumRunes == taskmanager.EvidenceSummaryMaximumRunes,
		d.RepositoryMaximumRunes == taskmanager.RepositoryMaximumRunes,
		d.CommitSummaryMaximumRunes == taskmanager.CommitSummaryMaximumRunes,
		len(d.Operations) == int(operationLimit-operationListProjects),
	}
	for _, valid := range facts {
		if !valid {
			return commandError(schemaProjectionInvalidDetail, nil)
		}
	}
	for index, candidate := range d.Operations {
		if candidate != operation(index)+operationListProjects {
			return commandError("schema operation ordering is invalid", nil)
		}
	}
	return nil
}

func (d schemaDocument) MarshalJSON() ([]byte, error) {
	if err := d.Validate(); err != nil {
		return nil, err
	}
	type wire schemaDocument
	return core.MarshalCanonicalJSONDocument(wire(d))
}

var (
	_ core.ValidatedJSONMarshaler = commandResult{}
	_ core.ValidatedJSONMarshaler = schemaDocument{}
)
