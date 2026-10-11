//go:build linux

package runnercontrol_test

import (
	json "encoding/json/v2"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runnercontrol"
)

type nativeSystemdPathProperty struct {
	Type string   `json:"type"`
	Data []string `json:"data"`
}

func TestCompiledSystemdPathsMatchNativeBusProperties(t *testing.T) {
	t.Parallel()
	spellings := [...]string{"root space", "two  spaces", `quote"path`, `single'path`, `back\slash`, `mixed "\ path`, "東京 space", "path\twith\ttabs"}
	for index := range 64 {
		t.Run(fmt.Sprint(index), func(t *testing.T) {
			t.Parallel()
			capability := experimentObservationRequestFixture(t).Capability
			root := filepath.Join(t.TempDir(), spellings[index%8]+fmt.Sprint(index))
			if err := os.Mkdir(root, 0700); err != nil {
				t.Fatal(err)
			}
			capability.Execution.Subject.SourceRoot = mustProfileAbsolutePath(t, root)
			capability.Execution.Process.WorkingDirectory = capability.Execution.Subject.SourceRoot
			plan, err := runnercontrol.CompileSubjectProcess(capability)
			if err != nil {
				t.Fatal(err)
			}
			arguments := planArguments(t, plan)
			unit := fmt.Sprintf("primitivepathproof%dcase%d.service", os.Getpid(), index)
			nativeArgs := []string{"--user", "--unit=" + unit, "--property=RemainAfterExit=yes"}
			for _, argument := range arguments {
				if strings.HasPrefix(argument, core.SystemdReadOnlyPathsArgument) || strings.HasPrefix(argument, core.SystemdReadWritePathsArgument) || strings.HasPrefix(argument, core.SystemdInaccessiblePathsArgument) {
					nativeArgs = append(nativeArgs, argument)
				}
			}
			nativeArgs = append(nativeArgs, "/usr/bin/true")
			// The native manager must parse the actual compiled argv. This proof
			// observes its path properties; process namespace availability is a
			// separate execution contract, not approximated by a Go unquote helper.
			output, err := exec.CommandContext(t.Context(), "systemd-run", nativeArgs...).CombinedOutput()
			if err != nil {
				t.Fatalf("native systemd admission: %v: %s", err, output)
			}
			defer func() {
				output, err := exec.Command("systemctl", "--user", "stop", unit).CombinedOutput()
				if err != nil {
					t.Errorf("native unit cleanup: %v: %s", err, output)
				}
			}()
			object := "/org/freedesktop/systemd1/unit/" + strings.TrimSuffix(unit, ".service") + "_2eservice"
			for _, property := range [...]struct {
				name  string
				paths []string
			}{
				{name: "ReadOnlyPaths", paths: []string{root}},
				{name: "ReadWritePaths", paths: []string{capability.Execution.Workspace.Root.String()}},
				{name: "InaccessiblePaths", paths: []string{capability.Execution.Subject.ControlSocket.String(), capability.Execution.Subject.HostCredentials.String(), capability.Execution.Subject.SigningState.String(), capability.Execution.Subject.ExecutableState.String()}},
			} {
				output, err := exec.CommandContext(t.Context(), "busctl", "--user", "--json=short", "get-property", "org.freedesktop.systemd1", object, "org.freedesktop.systemd1.Service", property.name).CombinedOutput()
				if err != nil {
					t.Fatalf("native bus observation: %v: %s", err, output)
				}
				var observed nativeSystemdPathProperty
				if err := json.Unmarshal(output, &observed); err != nil {
					t.Fatal(err)
				}
				if observed.Type != "as" || !slices.Equal(observed.Data, property.paths) {
					t.Fatalf("native %s=%q, signed paths=%q", property.name, observed.Data, property.paths)
				}
			}
		})
	}
}
