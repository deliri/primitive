package taskmanager

import (
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

const (
	// ProtocolRevisionV1 is the sole published task-manager socket revision.
	ProtocolRevisionV1     uint16 = 1
	protocolRevisionV1Path        = "/v1"
	taskManagerPath               = protocolRevisionV1Path + "/task-manager"
	listProjectsPath              = taskManagerPath + "/projects/list"
	getProjectPath                = taskManagerPath + "/projects/get"
	createProjectPath             = taskManagerPath + "/projects/create"
	listPhasesPath                = taskManagerPath + "/phases/list"
	createPhasePath               = taskManagerPath + "/phases/create"
	listTasksPath                 = taskManagerPath + "/tasks/list"
	getTaskPath                   = taskManagerPath + "/tasks/get"
	createTaskPath                = taskManagerPath + "/tasks/create"
	updateTaskPath                = taskManagerPath + "/tasks/update"
	listEvidencePath              = taskManagerPath + "/evidence/list"
	appendEvidencePath            = taskManagerPath + "/evidence/append"
	listGitCommitsPath            = taskManagerPath + "/git-commits/list"
	appendGitCommitPath           = taskManagerPath + "/git-commits/append"
	completeTaskPath              = taskManagerPath + "/tasks/complete"
)

// Route is the closed task-manager HTTP socket domain.
type Route uint8

const (
	RouteUnknown Route = iota
	RouteListProjects
	RouteGetProject
	RouteCreateProject
	RouteListPhases
	RouteCreatePhase
	RouteListTasks
	RouteGetTask
	RouteCreateTask
	RouteUpdateTask
	RouteListEvidence
	RouteAppendEvidence
	RouteListGitCommits
	RouteAppendGitCommit
	RouteCompleteTask
	routeLimit
)

type routeFact struct {
	path      string
	semantics exchange.RouteSemantics
}

func routeFacts() [routeLimit]routeFact {
	return [...]routeFact{
		RouteUnknown: {},
		RouteListProjects: {
			path: listProjectsPath,
			semantics: exchange.RouteSemantics{
				Method: exchange.MethodPost,
				Replay: exchange.ReplaySingleAttempt,
			},
		},
		RouteGetProject:    singleRouteFact(getProjectPath, exchange.MethodPost),
		RouteCreateProject: mutationRouteFact(createProjectPath, exchange.MethodPost),
		RouteListPhases:    singleRouteFact(listPhasesPath, exchange.MethodPost),
		RouteCreatePhase:   mutationRouteFact(createPhasePath, exchange.MethodPost),
		RouteListTasks:     singleRouteFact(listTasksPath, exchange.MethodPost),
		RouteGetTask:       singleRouteFact(getTaskPath, exchange.MethodPost),
		RouteCreateTask:    mutationRouteFact(createTaskPath, exchange.MethodPost),
		RouteUpdateTask: {
			path: updateTaskPath,
			semantics: exchange.RouteSemantics{
				Method: exchange.MethodPatch,
				Replay: exchange.ReplayIdempotencyKey,
			},
		},
		RouteListEvidence:    singleRouteFact(listEvidencePath, exchange.MethodPost),
		RouteAppendEvidence:  mutationRouteFact(appendEvidencePath, exchange.MethodPost),
		RouteListGitCommits:  singleRouteFact(listGitCommitsPath, exchange.MethodPost),
		RouteAppendGitCommit: mutationRouteFact(appendGitCommitPath, exchange.MethodPost),
		RouteCompleteTask:    mutationRouteFact(completeTaskPath, exchange.MethodPatch),
	}
}

func singleRouteFact(path string, method exchange.Method) routeFact {
	return routeFact{path: path, semantics: exchange.RouteSemantics{
		Method: method, Replay: exchange.ReplaySingleAttempt,
	}}
}

func mutationRouteFact(path string, method exchange.Method) routeFact {
	return routeFact{path: path, semantics: exchange.RouteSemantics{
		Method: method, Replay: exchange.ReplayIdempotencyKey,
	}}
}

func (r Route) Validate() error {
	if r <= RouteUnknown || r >= routeLimit || routeFacts()[r].path == "" {
		return contractError()
	}
	if err := routeFacts()[r].semantics.Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

func (r Route) IsValid() bool { return r.Validate() == nil }

func (r Route) String() string {
	path, _ := r.Path()
	return path
}

// Path projects the exact mounted path owned by r.
func (r Route) Path() (string, error) {
	if err := r.Validate(); err != nil {
		return "", err
	}
	return routeFacts()[r].path, nil
}

// Semantics projects the exact method and replay contract owned by r.
func (r Route) Semantics() (exchange.RouteSemantics, error) {
	if err := r.Validate(); err != nil {
		return exchange.RouteSemantics{}, err
	}
	return routeFacts()[r].semantics, nil
}

// OffWireEnum declares that Route selects a local socket and never enters JSON.
func (Route) OffWireEnum() {}

var _ core.OffWireEnum = RouteUnknown
