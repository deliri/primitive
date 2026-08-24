package main

import (
	"bytes"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/taskmanager"
)

type commandInputScenario struct {
	stdin             io.Reader
	wantErr           error
	wantJob           jobDocument
	root              core.AbsolutePath
	jobPath           string
	wantConfiguration configurationDocument
}

func TestTaskCommandFileIngressLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		build func(*testing.T) commandInputScenario
		name  string
	}{
		{name: "positive regular files return exact typed documents", build: regularFileScenario},
		{name: "negative directory returns zero documents", build: directoryFileScenario},
		{name: "negative absent file returns zero documents", build: absentFileScenario},
		{name: "neutral confined symbolic link returns exact typed documents", build: symbolicLinkScenario},
		{name: "negative escaping symbolic link returns zero documents", build: escapingSymbolicLinkScenario},
		{name: "negative oversized JSON returns zero documents", build: oversizedFileScenario},
		{name: "neutral stdin returns the same bounded typed job", build: standardInputScenario},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			scenario := tc.build(t)
			gotConfiguration, gotJob, gotErr := loadInputs(
				t.Context(), scenario.root, scenario.jobPath, scenario.stdin,
			)
			if !errors.Is(gotErr, scenario.wantErr) {
				t.Fatalf("loadInputs() error = %v, want %v", gotErr, scenario.wantErr)
			}
			gotConfigurationJSON := commandConfigurationJSON(t, gotConfiguration)
			wantConfigurationJSON := commandConfigurationJSON(t, scenario.wantConfiguration)
			if !bytes.Equal(gotConfigurationJSON, wantConfigurationJSON) {
				t.Fatalf("loadInputs() configuration = %s, want %s", gotConfigurationJSON, wantConfigurationJSON)
			}
			gotJobJSON := commandJobJSON(t, gotJob)
			wantJobJSON := commandJobJSON(t, scenario.wantJob)
			if !bytes.Equal(gotJobJSON, wantJobJSON) {
				t.Fatalf("loadInputs() job = %s, want %s", gotJobJSON, wantJobJSON)
			}
		})
	}
}

func regularFileScenario(t *testing.T) commandInputScenario {
	t.Helper()
	root, configuration, job := commandInputFixture(t)
	writeCommandInputFixtures(t, root, configuration, job)
	return commandInputScenario{
		root: root, jobPath: defaultJobFileName, stdin: bytes.NewReader(nil),
		wantConfiguration: configuration, wantJob: job,
	}
}

func directoryFileScenario(t *testing.T) commandInputScenario {
	t.Helper()
	root, configuration, job := commandInputFixture(t)
	writeCommandInputFixtures(t, root, configuration, job)
	target := commandPath(t, root, "directory-job")
	if err := os.Mkdir(target.String(), 0o700); err != nil {
		t.Fatalf("os.Mkdir(directory-job) error = %v, want nil", err)
	}
	return commandInputScenario{
		root: root, jobPath: "directory-job", stdin: bytes.NewReader(nil),
		wantErr: core.ErrFilestoreSource,
	}
}

func absentFileScenario(t *testing.T) commandInputScenario {
	t.Helper()
	root, configuration, job := commandInputFixture(t)
	writeCommandInputFixtures(t, root, configuration, job)
	return commandInputScenario{
		root: root, jobPath: "absent-job.json", stdin: bytes.NewReader(nil),
		wantErr: core.ErrFilestoreSource,
	}
}

func symbolicLinkScenario(t *testing.T) commandInputScenario {
	t.Helper()
	root, configuration, job := commandInputFixture(t)
	writeCommandInputFixtures(t, root, configuration, job)
	link := commandPath(t, root, "linked-job.json")
	if err := os.Symlink(defaultJobFileName, link.String()); err != nil {
		t.Fatalf("os.Symlink(job) error = %v, want nil", err)
	}
	return commandInputScenario{
		root: root, jobPath: "linked-job.json", stdin: bytes.NewReader(nil),
		wantConfiguration: configuration, wantJob: job,
	}
}

func escapingSymbolicLinkScenario(t *testing.T) commandInputScenario {
	t.Helper()
	root, configuration, job := commandInputFixture(t)
	writeCommandInputFixtures(t, root, configuration, job)
	externalRoot, err := core.ParseAbsolutePath(t.TempDir())
	if err != nil {
		t.Fatalf("core.ParseAbsolutePath(external tempdir) error = %v, want nil", err)
	}
	external := commandPath(t, externalRoot, "external-job.json")
	if err := os.WriteFile(external.String(), commandJobJSON(t, job), 0o600); err != nil {
		t.Fatalf("os.WriteFile(external job) error = %v, want nil", err)
	}
	link := commandPath(t, root, "escaping-job.json")
	if err := os.Symlink(external.String(), link.String()); err != nil {
		t.Fatalf("os.Symlink(external job) error = %v, want nil", err)
	}
	return commandInputScenario{
		root: root, jobPath: "escaping-job.json", stdin: bytes.NewReader(nil),
		wantErr: core.ErrFilestoreSource,
	}
}

func oversizedFileScenario(t *testing.T) commandInputScenario {
	t.Helper()
	root, configuration, _ := commandInputFixture(t)
	writeCommandFile(t, root, configurationFileName, commandConfigurationJSON(t, configuration))
	writeCommandFile(t, root, defaultJobFileName, bytes.Repeat([]byte{' '}, commandDocumentMaxBytes+1))
	return commandInputScenario{
		root: root, jobPath: defaultJobFileName, stdin: bytes.NewReader(nil),
		wantErr: core.ErrJSONContract,
	}
}

func standardInputScenario(t *testing.T) commandInputScenario {
	t.Helper()
	root, configuration, job := commandInputFixture(t)
	writeCommandFile(t, root, configurationFileName, commandConfigurationJSON(t, configuration))
	return commandInputScenario{
		root: root, jobPath: standardInputPath, stdin: bytes.NewReader(commandJobJSON(t, job)),
		wantConfiguration: configuration, wantJob: job,
	}
}

func commandInputFixture(t testing.TB) (core.AbsolutePath, configurationDocument, jobDocument) {
	t.Helper()
	root, err := core.ParseAbsolutePath(t.TempDir())
	if err != nil {
		t.Fatalf("core.ParseAbsolutePath(tempdir) error = %v, want nil", err)
	}
	authority, err := core.ParseHTTPEndpoint("https://admin.example.com")
	if err != nil {
		t.Fatalf("core.ParseHTTPEndpoint() error = %v, want nil", err)
	}
	projectID := commandUUIDFixture(t, "019ff548-29cb-7451-869e-aa644c0947e6")
	configuration := configurationDocument{
		Revision: commandDocumentRevisionV1, Authority: authority, Username: commandIdentityFixture(t),
		PasswordSecret: googleSecretReference{Project: "example-task-project", Secret: "task-manager-admin-password"},
		ProjectID:      &projectID,
	}
	job := jobDocument{
		Revision: commandDocumentRevisionV1, Operation: operationListProjects,
		ListProjects: &listProjectsInput{Lifecycle: taskmanager.ProjectLifecycleActive, Order: taskmanager.PageOrderDescending, Limit: 17},
	}
	return root, configuration, job
}

func writeCommandInputFixtures(
	t testing.TB,
	root core.AbsolutePath,
	configuration configurationDocument,
	job jobDocument,
) {
	t.Helper()
	writeCommandFile(t, root, configurationFileName, commandConfigurationJSON(t, configuration))
	writeCommandFile(t, root, defaultJobFileName, commandJobJSON(t, job))
}

func writeCommandFile(t testing.TB, root core.AbsolutePath, name string, data []byte) {
	t.Helper()
	path := commandPath(t, root, name)
	if err := os.WriteFile(path.String(), data, 0o600); err != nil {
		t.Fatalf("os.WriteFile(%s) error = %v, want nil", name, err)
	}
}

func commandPath(t testing.TB, root core.AbsolutePath, name string) core.AbsolutePath {
	t.Helper()
	path, err := root.Resolve(name)
	if err != nil {
		t.Fatalf("root.Resolve(%s) error = %v, want nil", name, err)
	}
	return path
}

func commandConfigurationJSON(t testing.TB, value configurationDocument) []byte {
	t.Helper()
	if value == (configurationDocument{}) {
		return nil
	}
	encoded, err := value.MarshalJSON()
	if err != nil {
		t.Fatalf("configuration MarshalJSON() error = %v, want nil", err)
	}
	return encoded
}

func commandJobJSON(t testing.TB, value jobDocument) []byte {
	t.Helper()
	if value.payloadCount() == 0 {
		return nil
	}
	encoded, err := value.MarshalJSON()
	if err != nil {
		t.Fatalf("job MarshalJSON() error = %v, want nil", err)
	}
	return encoded
}
