package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

type taskCommandStructRole uint8

const (
	taskCommandStructRoleUnknown taskCommandStructRole = iota
	taskCommandStructRoleProtocolIngress
	taskCommandStructRoleProtocolProjection
	taskCommandStructRoleInternalFlow
	taskCommandStructRoleCapability
	taskCommandStructRoleLimit
)

func (r taskCommandStructRole) Validate() bool {
	return r > taskCommandStructRoleUnknown && r < taskCommandStructRoleLimit
}

func TestTaskCommandDataFlowStructInventory(t *testing.T) {
	t.Parallel()

	want := map[string]taskCommandStructRole{
		"googleSecretReference":          taskCommandStructRoleProtocolIngress,
		"evidenceStorageReference":       taskCommandStructRoleProtocolIngress,
		"configurationDocument":          taskCommandStructRoleProtocolIngress,
		"listProjectsInput":              taskCommandStructRoleProtocolIngress,
		"createProjectInput":             taskCommandStructRoleProtocolIngress,
		"listPhasesInput":                taskCommandStructRoleProtocolIngress,
		"createPhaseInput":               taskCommandStructRoleProtocolIngress,
		"listTasksInput":                 taskCommandStructRoleProtocolIngress,
		"createTaskInput":                taskCommandStructRoleProtocolIngress,
		"getTaskInput":                   taskCommandStructRoleProtocolIngress,
		"taskChangeInput":                taskCommandStructRoleProtocolIngress,
		"updateTaskInput":                taskCommandStructRoleProtocolIngress,
		"completeTaskInput":              taskCommandStructRoleProtocolIngress,
		"listEvidenceInput":              taskCommandStructRoleProtocolIngress,
		"appendEvidenceInput":            taskCommandStructRoleProtocolIngress,
		"appendGitCommitInput":           taskCommandStructRoleProtocolIngress,
		"listGitCommitsInput":            taskCommandStructRoleProtocolIngress,
		"jobDocument":                    taskCommandStructRoleProtocolIngress,
		"invocation":                     taskCommandStructRoleProtocolIngress,
		"commandInputRequest":            taskCommandStructRoleInternalFlow,
		"commandResult":                  taskCommandStructRoleProtocolProjection,
		"schemaDocument":                 taskCommandStructRoleProtocolProjection,
		"commandTextErrorRequest":        taskCommandStructRoleInternalFlow,
		"executionRequest":               taskCommandStructRoleInternalFlow,
		"taskEvidenceAppendRequest":      taskCommandStructRoleInternalFlow,
		"taskEvidenceUploadPlan":         taskCommandStructRoleInternalFlow,
		"taskEvidenceUploadRequest":      taskCommandStructRoleInternalFlow,
		"taskEvidencePreparationRequest": taskCommandStructRoleInternalFlow,
		"taskEvidenceObjectNameRequest":  taskCommandStructRoleInternalFlow,
		"taskEvidenceUploadReceipt":      taskCommandStructRoleInternalFlow,
		"commandStreams":                 taskCommandStructRoleCapability,
		"taskEvidenceSource":             taskCommandStructRoleCapability,
	}
	for name, role := range want {
		if !role.Validate() {
			t.Fatalf("struct inventory role for %s = %d, want a published role", name, role)
		}
	}

	got := taskCommandProductionStructNames(t)
	wantNames := make([]string, 0, len(want))
	for name := range want {
		wantNames = append(wantNames, name)
	}
	sort.Strings(wantNames)
	if strings.Join(got, "\n") != strings.Join(wantNames, "\n") {
		t.Fatalf("production struct inventory = %v, want classified %v", got, wantNames)
	}
}

func taskCommandProductionStructNames(t testing.TB) []string {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("os.ReadDir(command package) error = %v, want nil", err)
	}
	var names []string
	files := token.NewFileSet()
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file, parseErr := parser.ParseFile(files, filepath.Clean(entry.Name()), nil, 0)
		if parseErr != nil {
			t.Fatalf("parser.ParseFile(%q) error = %v, want nil", entry.Name(), parseErr)
		}
		for _, declaration := range file.Decls {
			general, ok := declaration.(*ast.GenDecl)
			if !ok || general.Tok != token.TYPE {
				continue
			}
			for _, specification := range general.Specs {
				typed, ok := specification.(*ast.TypeSpec)
				if !ok {
					continue
				}
				if _, ok := typed.Type.(*ast.StructType); ok {
					names = append(names, typed.Name.Name)
				}
			}
		}
	}
	sort.Strings(names)
	return names
}
