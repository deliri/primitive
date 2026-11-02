package gotoolchain

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"golang.org/x/mod/module"
	modzip "golang.org/x/mod/zip"
)

type moduleClosureFixture struct {
	capability Capability
	directory  core.AbsolutePath
	cache      string
	sums       []byte
}

func newModuleClosureFixture(t *testing.T) moduleClosureFixture {
	t.Helper()
	root := t.TempDir()
	proxy := filepath.Join(root, "proxy")
	for _, row := range []struct{ path, version, requirements string }{
		{"example.com/a", "v1.0.0", "require example.com/m v1.0.0\n"},
		{"example.com/b", "v1.0.0", "require example.com/m v1.1.0\n"},
		{"example.com/m", "v1.0.0", "require example.com/z v1.0.0\n"},
		{"example.com/m", "v1.1.0", ""},
		{"example.com/z", "v1.0.0", ""},
	} {
		content := []byte("module " + row.path + "\n\ngo 1.16\n" + row.requirements)
		input := filepath.Join(root, "input", strings.TrimPrefix(row.path, "example.com/"), row.version)
		if err := os.MkdirAll(input, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(input, "go.mod"), content, 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(input, "dependency.go"), []byte("package dependency\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		output := filepath.Join(proxy, row.path, "@v")
		if err := os.MkdirAll(output, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(output, row.version+".mod"), content, 0o644); err != nil {
			t.Fatal(err)
		}
		metadata, err := json.Marshal(struct {
			Version string
			Time    time.Time
		}{row.version, time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(output, row.version+".info"), metadata, 0o644); err != nil {
			t.Fatal(err)
		}
		archive, err := os.Create(filepath.Join(output, row.version+".zip"))
		if err != nil {
			t.Fatal(err)
		}
		writeErr := modzip.CreateFromDir(archive, module.Version{Path: row.path, Version: row.version}, input)
		if err := errors.Join(writeErr, archive.Close()); err != nil {
			t.Fatal(err)
		}
	}
	directory := filepath.Join(root, "subject")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "go.mod"), []byte("module example.com/subject\n\ngo 1.16\nrequire (\n example.com/a v1.0.0\n example.com/b v1.0.0\n)\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cache := filepath.Join(root, "cache")
	capability, scope := installationFixtureCapability(t, directory, []string{
		"GOPROXY=file://" + filepath.ToSlash(proxy), "GOSUMDB=off", "GOVCS=*:off", "GOTOOLCHAIN=local", "GOENV=off", "GOFLAGS=", "GOMODCACHE=" + cache,
	})
	fixture := moduleClosureFixture{capability: capability, directory: scope.WorkingDirectory, cache: cache}
	fixture.goCommand(t, "mod", "download", "all")
	// Native module directories are read-only. These are exclusively owned
	// fixture directories; let TempDir cleanup remove them on every exit.
	if err := filepath.WalkDir(cache, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return os.Chmod(path, 0o755)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	var err error
	fixture.sums, err = os.ReadFile(filepath.Join(directory, "go.sum"))
	if err != nil {
		t.Fatal(err)
	}
	// Observation is offline. Removing metadata below must refuse, rather
	// than silently repairing it from our fixture's otherwise valid proxy.
	fixture.capability, _ = installationFixtureCapability(t, directory, []string{"GOPROXY=off", "GOSUMDB=off", "GOVCS=*:off", "GOTOOLCHAIN=local", "GOENV=off", "GOFLAGS=", "GOMODCACHE=" + cache})
	return fixture
}

func (f moduleClosureFixture) goCommand(t *testing.T, args ...string) []byte {
	t.Helper()
	command := exec.CommandContext(t.Context(), f.capability.command.String(), args...)
	command.Dir = f.directory.String()
	var err error
	command.Env, err = f.capability.environment.Strings()
	if err != nil {
		t.Fatal(err)
	}
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("native go %q output=%s error=%v, want success", args, output, err)
	}
	return output
}

func (f moduleClosureFixture) request() ModuleClosureRequest {
	return ModuleClosureRequest{WorkingDirectory: f.directory, GoSum: bytes.NewReader(f.sums), VisitSelected: func(context.Context, SelectedModuleObservation) error { return nil }, VisitGoMod: func(context.Context, GoModMetadataObservation) error { return nil }}
}

func TestModuleClosureNativeMVSIncludesLosingMetadata(t *testing.T) {
	t.Parallel()
	f := newModuleClosureFixture(t)
	beforeMod, err := os.ReadFile(filepath.Join(f.directory.String(), "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	var selected []SelectedModuleObservation
	var metadata []GoModMetadataObservation
	request := f.request()
	request.VisitSelected = func(_ context.Context, row SelectedModuleObservation) error {
		selected = append(selected, row)
		return nil
	}
	request.VisitGoMod = func(_ context.Context, row GoModMetadataObservation) error {
		metadata = append(metadata, row)
		return nil
	}
	got, err := f.capability.ObserveModuleClosure(t.Context(), request)
	if err != nil || got.Validate() != nil || got.Selected != 4 || got.AvailableGoMod != 5 || got.ModuleCache.String() != f.cache {
		t.Fatalf("closure=%+v error=%v, want 4 selected and all 5 native metadata entries", got, err)
	}
	var identities []string
	for _, row := range selected {
		if err := row.Validate(); err != nil {
			t.Fatal(err)
		}
		identities = append(identities, row.Module.String()+"@"+row.Version.String())
	}
	slices.Sort(identities)
	want := []string{"example.com/a@v1.0.0", "example.com/b@v1.0.0", "example.com/m@v1.1.0", "example.com/z@v1.0.0"}
	if !slices.Equal(identities, want) {
		t.Fatalf("selection=%q, want %q", identities, want)
	}
	loser := false
	for _, row := range metadata {
		if row.Module.String() == "example.com/m" && row.Version.String() == "v1.0.0" {
			loser = true
		}
	}
	if !loser {
		t.Fatalf("metadata=%+v, want nonselected m@v1.0.0 go.mod", metadata)
	}
	afterMod, modErr := os.ReadFile(filepath.Join(f.directory.String(), "go.mod"))
	afterSum, sumErr := os.ReadFile(filepath.Join(f.directory.String(), "go.sum"))
	if errors.Join(modErr, sumErr) != nil || !bytes.Equal(beforeMod, afterMod) || !bytes.Equal(f.sums, afterSum) {
		t.Fatalf("module files changed or unreadable: mod=%v sum=%v, want exact frozen inputs", modErr, sumErr)
	}
	// No transported ZIP is needed by native verification; both native verify
	// and the capability must still bind the extracted bytes to frozen h1.
	if err := filepath.WalkDir(f.cache, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.HasSuffix(path, ".zip") {
			return os.Remove(path)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	f.goCommand(t, "mod", "verify")
	if got, err := f.capability.ObserveModuleClosure(t.Context(), f.request()); err != nil || got.Selected != 4 {
		t.Fatalf("zip-free observation=%+v/%v, want same verified selection", got, err)
	}
}

func TestModuleClosureNativeRefusesChangedSelectedAuthority(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"tree", "go_mod", "info", "ziphash", "partial", "missing_info", "missing_tree", "missing_sum", "foreign_sum"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newModuleClosureFixture(t)
			prefix := filepath.Join(f.cache, "cache", "download", "example.com", "a", "@v", "v1.0.0")
			var err error
			switch name {
			case "tree":
				path := filepath.Join(f.cache, "example.com", "a@v1.0.0", "dependency.go")
				if err = os.Chmod(path, 0o644); err == nil {
					err = os.WriteFile(path, []byte("package changed\n"), 0o644)
				}
			case "go_mod":
				err = os.WriteFile(prefix+".mod", []byte("module example.com/wrong\n"), 0o644)
			case "info":
				err = os.WriteFile(prefix+".info", []byte(`{"Version":"v1.9.0"}`), 0o644)
			case "ziphash":
				err = os.WriteFile(prefix+".ziphash", []byte("h1:"+strings.Repeat("A", 43)+"="), 0o644)
			case "partial":
				err = os.WriteFile(prefix+".partial", nil, 0o644)
			case "missing_info":
				err = os.Remove(prefix + ".info")
			case "missing_tree":
				err = os.RemoveAll(filepath.Join(f.cache, "example.com", "a@v1.0.0"))
			case "missing_sum":
				f.sums = nil
			case "foreign_sum":
				f.sums = bytes.Replace(f.sums, []byte("example.com/a v1.0.0 h1:"), []byte("example.com/a v1.0.0 h1:Z"), 1)
			}
			if err != nil {
				t.Fatal(err)
			}
			got, err := f.capability.ObserveModuleClosure(t.Context(), f.request())
			if err == nil || got != (ModuleClosureObservation{}) {
				t.Fatalf("changed %s observation=%+v/%v, want zero refusal", name, got, err)
			}
		})
	}
}

func TestModuleClosureNativeCallbackCancellationAndAbsentHistory(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"selected_refusal", "metadata_refusal", "canceled", "absent_history"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newModuleClosureFixture(t)
			request := f.request()
			cause := errors.New("module callback stopped")
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			switch name {
			case "selected_refusal":
				request.VisitSelected = func(context.Context, SelectedModuleObservation) error { return cause }
			case "metadata_refusal":
				request.VisitGoMod = func(context.Context, GoModMetadataObservation) error { return cause }
			case "canceled":
				cancel()
				cause = context.Canceled
			case "absent_history":
				// This old checksum is authentic but is no selected/graph input.
				// The observer may omit it; closure completeness remains the
				// consumer's separate isolated offline replay contract.
				index, err := readModuleSums(t.Context(), bytes.NewReader(f.sums))
				if err != nil {
					t.Fatal(err)
				}
				request.GoSum = strings.NewReader(string(f.sums) + fmt.Sprintf("example.com/history v1.0.0/go.mod %s\n", index.expected[index.order[0]].String()))
			}
			got, err := f.capability.ObserveModuleClosure(ctx, request)
			if name == "absent_history" {
				if err != nil || got.AvailableGoMod != 5 {
					t.Fatalf("absent history=%+v/%v, want 5 actual available files", got, err)
				}
			} else if !errors.Is(err, cause) || got != (ModuleClosureObservation{}) {
				t.Fatalf("callback/cancel=%+v/%v, want zero preserving %v", got, err, cause)
			}
		})
	}
}

func TestModuleClosureFrozenExpectedHashControlsNativeAuthority(t *testing.T) {
	t.Parallel()
	for _, suffix := range []string{"", "/go.mod"} {
		t.Run("frozen"+strings.ReplaceAll(suffix, "/", "_"), func(t *testing.T) {
			t.Parallel()
			f := newModuleClosureFixture(t)
			// A native cache verify can pass while the caller's independently
			// frozen expected checksum differs. Neither native .ziphash nor
			// the on-disk module files may substitute for that expected h1.
			f.goCommand(t, "mod", "verify")
			prefix := "example.com/a v1.0.0" + suffix + " "
			lines := strings.Split(string(f.sums), "\n")
			changed := false
			for i, line := range lines {
				if strings.HasPrefix(line, prefix) {
					lines[i] = prefix + moduleClosureTestSum(t, "independently frozen other content\n")
					changed = true
				}
			}
			if !changed {
				t.Fatalf("native sums=%s, want exact selected checksum row %q", f.sums, prefix)
			}
			request := f.request()
			request.GoSum = strings.NewReader(strings.Join(lines, "\n"))
			got, err := f.capability.ObserveModuleClosure(t.Context(), request)
			if !errors.Is(err, core.ErrGoToolchainOutput) || got != (ModuleClosureObservation{}) {
				t.Fatalf("foreign frozen h1=%+v/%v, want zero output refusal", got, err)
			}
		})
	}
}

func TestModuleClosureSelectedProjectionRejectsForeignNativeFacts(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"directory", "go_mod", "missing_version", "foreign_path", "foreign_hash", "major_mismatch"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			f := newModuleClosureFixture(t)
			data := f.goCommand(t, "list", "-mod=readonly", "-m", "-json", "example.com/a")
			cursor := moduleJSONCursor{ctx: t.Context(), decoder: jsontext.NewDecoder(bytes.NewReader(data))}
			wire, err := readModuleSelection(cursor)
			if err != nil {
				t.Fatal(err)
			}
			index, err := readModuleSums(t.Context(), bytes.NewReader(f.sums))
			if err != nil {
				t.Fatal(err)
			}
			cache, err := core.ParseAbsolutePath(f.cache)
			if err != nil {
				t.Fatal(err)
			}
			switch name {
			case "directory":
				wire.directory = f.directory.String()
			case "go_mod":
				wire.goMod = filepath.Join(f.directory.String(), "go.mod")
			case "missing_version":
				wire.version = ""
			case "foreign_path":
				wire.path = "example.com/foreign"
			case "foreign_hash":
				wire.sum = moduleClosureTestSum(t, "foreign frozen tree\n")
			case "major_mismatch":
				wire.version = "v2.0.0"
			}
			got, err := projectSelectedModule(t.Context(), wire, moduleSelectionVisit{cache: cache, sums: index})
			if err == nil || got != (SelectedModuleObservation{}) {
				t.Fatalf("foreign %s=%+v/%v, want zero refused native projection", name, got, err)
			}
		})
	}
}
