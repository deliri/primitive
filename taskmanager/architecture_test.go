package taskmanager

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestProductionStructDataFlowInventory(t *testing.T) {
	t.Parallel()

	files, gotGlobErr := filepath.Glob("*.go")
	if gotGlobErr != nil {
		t.Fatalf("filepath.Glob() error = %v, want nil", gotGlobErr)
	}
	set := token.NewFileSet()
	found := 0
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, gotParseErr := parser.ParseFile(set, path, nil, 0)
		if gotParseErr != nil {
			t.Fatalf("parser.ParseFile(%q) error = %v, want nil", path, gotParseErr)
		}
		ast.Inspect(file, func(node ast.Node) bool {
			spec, ok := node.(*ast.TypeSpec)
			if !ok {
				return true
			}
			if _, ok := spec.Type.(*ast.StructType); !ok {
				return true
			}
			role, classified := productionStructRole(spec.Name.Name)
			if !classified || !role.IsValid() {
				t.Errorf("production struct %s has role %v classified %t, want a precise data-flow role", spec.Name.Name, role, classified)
			}
			found++
			return false
		})
	}
	if found == 0 {
		t.Fatalf("production struct inventory found %d structs, want at least %d", found, 1)
	}
}

type dataFlowRole uint8

const (
	dataFlowRoleUnknown dataFlowRole = iota
	dataFlowRoleOptimisticRevision
	dataFlowRoleClosedDomain
	dataFlowRoleContinuation
	dataFlowRoleEntityIngress
	dataFlowRoleCollectionIngress
	dataFlowRoleEntityProjection
	dataFlowRoleCollectionProjection
	dataFlowRolePartialMutation
	dataFlowRoleMutationIngress
	dataFlowRoleDecodeCarrier
	dataFlowRoleProofProjection
	dataFlowRoleSocketConfiguration
	dataFlowRoleSocketCapability
	dataFlowRoleInternalFlow
	dataFlowRoleLimit
)

func (r dataFlowRole) IsValid() bool {
	return r > dataFlowRoleUnknown && r < dataFlowRoleLimit
}

type optimisticRevisionInventory struct{ Revision Revision }
type closedDomainInventory struct {
	enumFact  enumFact[ProjectLifecycle]
	routeFact routeFact
}
type continuationInventory struct {
	ProjectCursor   ProjectCursor
	PhaseCursor     PhaseCursor
	TaskCursor      TaskCursor
	EvidenceCursor  EvidenceCursor
	GitCommitCursor GitCommitCursor
}
type entityIngressInventory struct {
	GetProjectRequest GetProjectRequest
	GetTaskRequest    GetTaskRequest
}
type collectionIngressInventory struct {
	ListProjectsRequest   ListProjectsRequest
	ListPhasesRequest     ListPhasesRequest
	ListTasksRequest      ListTasksRequest
	ListEvidenceRequest   ListEvidenceRequest
	ListGitCommitsRequest ListGitCommitsRequest
}
type entityProjectionInventory struct {
	TaskSummary    TaskSummary
	ProjectDetail  ProjectDetail
	ProjectSummary ProjectSummary
	PhaseSummary   PhaseSummary
	TaskDetail     TaskDetail
}
type collectionProjectionInventory struct {
	ProjectPage   ProjectPage
	PhasePage     PhasePage
	TaskPage      TaskPage
	EvidencePage  EvidencePage
	GitCommitPage GitCommitPage
}
type partialMutationInventory struct{ TaskChange TaskChange }
type mutationIngressInventory struct {
	CreateProjectRequest   CreateProjectRequest
	UpdateTaskRequest      UpdateTaskRequest
	CreatePhaseRequest     CreatePhaseRequest
	CreateTaskRequest      CreateTaskRequest
	AppendEvidenceRequest  AppendEvidenceRequest
	AppendGitCommitRequest AppendGitCommitRequest
	CompleteTaskRequest    CompleteTaskRequest
}
type decodeCarrierInventory struct{ updateTaskRequestWire updateTaskRequestWire }
type proofProjectionInventory struct {
	EvidenceRecord  EvidenceRecord
	GitCommitRecord GitCommitRecord
}
type socketConfigurationInventory struct{ ClientConfiguration ClientConfiguration }
type socketCapabilityInventory struct{ Client Client }
type internalFlowInventory struct{ proofPageHeader proofPageHeader }

var (
	_ = closedDomainInventory{}.enumFact
	_ = closedDomainInventory{}.routeFact
	_ = decodeCarrierInventory{}.updateTaskRequestWire
	_ = internalFlowInventory{}.proofPageHeader
)

type roleInventory struct {
	structs reflect.Type
	role    dataFlowRole
}

func productionStructInventories() []roleInventory {
	return []roleInventory{
		{role: dataFlowRoleOptimisticRevision, structs: reflect.TypeFor[optimisticRevisionInventory]()},
		{role: dataFlowRoleClosedDomain, structs: reflect.TypeFor[closedDomainInventory]()},
		{role: dataFlowRoleContinuation, structs: reflect.TypeFor[continuationInventory]()},
		{role: dataFlowRoleEntityIngress, structs: reflect.TypeFor[entityIngressInventory]()},
		{role: dataFlowRoleCollectionIngress, structs: reflect.TypeFor[collectionIngressInventory]()},
		{role: dataFlowRoleEntityProjection, structs: reflect.TypeFor[entityProjectionInventory]()},
		{role: dataFlowRoleCollectionProjection, structs: reflect.TypeFor[collectionProjectionInventory]()},
		{role: dataFlowRolePartialMutation, structs: reflect.TypeFor[partialMutationInventory]()},
		{role: dataFlowRoleMutationIngress, structs: reflect.TypeFor[mutationIngressInventory]()},
		{role: dataFlowRoleDecodeCarrier, structs: reflect.TypeFor[decodeCarrierInventory]()},
		{role: dataFlowRoleProofProjection, structs: reflect.TypeFor[proofProjectionInventory]()},
		{role: dataFlowRoleSocketConfiguration, structs: reflect.TypeFor[socketConfigurationInventory]()},
		{role: dataFlowRoleSocketCapability, structs: reflect.TypeFor[socketCapabilityInventory]()},
		{role: dataFlowRoleInternalFlow, structs: reflect.TypeFor[internalFlowInventory]()},
	}
}

func productionStructRole(name string) (dataFlowRole, bool) {
	for _, inventory := range productionStructInventories() {
		if _, found := inventory.structs.FieldByName(name); found {
			return inventory.role, true
		}
	}
	return dataFlowRoleUnknown, false
}
