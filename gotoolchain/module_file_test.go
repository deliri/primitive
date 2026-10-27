package gotoolchain

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/hostfacts"
	"github.com/deliri/primitive/v2026/testserial"
)

// This is a new boundary with no prior executable API. Compilation failure is
// not a red result: actual native-file contracts and semantic mutations must
// establish the ratchet before publication.
func TestObserveModuleFileUsesNativeGoWithoutEditingOrResolvingDependencies(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, body, version string
		presence            ModuleGoDeclaration
		replacements        uint64
		refusal             bool
	}{
		{name: "pinned release", body: "go 1.27.2\n", version: "go1.27.2", presence: ModuleGoDeclarationPresent},
		{name: "language version", body: "go 1.27\n", version: "go1.27", presence: ModuleGoDeclarationPresent},
		{name: "prerelease version", body: "go 1.27rc1\n", version: "go1.27rc1", presence: ModuleGoDeclarationPresent},
		{name: "absent Go directive", presence: ModuleGoDeclarationAbsent},
		{name: "default toolchain is not a Go version", body: "go 1.27.2\ntoolchain default\n", version: "go1.27.2", presence: ModuleGoDeclarationPresent},
		{name: "requirements need no network resolution", body: "go 1.27.2\nrequire example.invalid/unavailable v1.0.0\n", version: "go1.27.2", presence: ModuleGoDeclarationPresent},
		{name: "replacement block counts actual entries", body: "go 1.27.2\nreplace (\n example.invalid/a => ../absent-a\n example.invalid/b v1.0.0 => example.invalid/c v1.2.3\n)\n", version: "go1.27.2", presence: ModuleGoDeclarationPresent, replacements: 2},
		{name: "comment cannot fabricate replacement", body: "go 1.27.2\n// replace a => ../b\n", version: "go1.27.2", presence: ModuleGoDeclarationPresent},
		{name: "duplicate Go directive is native refusal", body: "go 1.27.2\ngo 1.27.2\n", refusal: true},
		{name: "custom toolchain suffix is not Go directive grammar", body: "go 1.27.2-custom\n", refusal: true},
		{name: "broken replacement is native refusal", body: "go 1.27.2\nreplace example.invalid/a =>\n", refusal: true},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			directory := t.TempDir()
			before := []byte("module example.com/native-module-file\n\n" + row.body)
			path := filepath.Join(directory, "go.mod")
			if err := os.WriteFile(path, before, 0600); err != nil {
				t.Fatal(err)
			}
			capability, request := installationFixtureCapability(t, directory, []string{"GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off"})
			got, err := capability.ObserveModuleFile(t.Context(), request)
			if row.refusal {
				var exit CommandExitError
				if got != (ModuleFileObservation{}) || !errors.Is(err, core.ErrGoToolchainExecution) || !errors.As(err, &exit) || exit.Process.ExitCode == 0 {
					t.Fatalf("native mod refusal%+v/%v", got, err)
				}
			} else if err != nil || got.Validate() != nil || got.GoDeclaration != row.presence || got.GoVersion.String() != row.version || got.Replacements != row.replacements {
				t.Fatalf("native module%+v/%v want%s/%s/%d", got, err, row.presence, row.version, row.replacements)
			}
			after, readErr := os.ReadFile(path)
			if readErr != nil || !bytes.Equal(before, after) {
				t.Fatalf("native observation changedgo.mod=%t readerror%v", !bytes.Equal(before, after), readErr)
			}
			// An independent native command must agree on success/refusal; the
			// semantic oracle above is bound to authored directives, not JSON output.
			native := exec.CommandContext(t.Context(), capability.command.String(), "mod", "edit", "-json")
			native.Dir = directory
			native.Env, _ = capability.environment.Strings()
			_, nativeErr := native.Output()
			if (nativeErr != nil) != row.refusal {
				t.Fatalf("native Go parityerror%v refusal%t", nativeErr, row.refusal)
			}
			if row.refusal {
				// The refusal carries Go's own diagnostic: the same stderr suffix
				// an independent native run writes, and Error() names it.
				var exit CommandExitError
				var nativeExit *exec.ExitError
				if !errors.As(err, &exit) || !errors.As(nativeErr, &nativeExit) {
					t.Fatalf("refusal types=%v/%v", err, nativeErr)
				}
				want := nativeExit.Stderr[max(0, len(nativeExit.Stderr)-commandStderrTailBytes):]
				if len(want) == 0 || !bytes.Equal(exit.Stderr.Data, want) || exit.Stderr.Validate() != nil || !strings.Contains(err.Error(), strings.TrimSpace(string(want))) {
					t.Fatalf("refusal stderr=%q error=%q, want native suffix %q", exit.Stderr.Data, err, want)
				}
			}
		})
	}
}

func TestNativeModuleFileProducerProjectionMatrix(t *testing.T) {
	t.Parallel()
	for _, version := range []struct {
		name, directive, token string
		presence               ModuleGoDeclaration
		refusal                bool
	}{
		{name: "absent", presence: ModuleGoDeclarationAbsent},
		{name: "release", directive: "go 1.27.2\n", token: "go1.27.2", presence: ModuleGoDeclarationPresent},
		{name: "language", directive: "go 1.27\n", token: "go1.27", presence: ModuleGoDeclarationPresent},
		{name: "older release", directive: "go 1.26.99\n", token: "go1.26.99", presence: ModuleGoDeclarationPresent},
		{name: "prerelease", directive: "go 1.27rc1\n", token: "go1.27rc1", presence: ModuleGoDeclarationPresent},
		{name: "custom suffix", directive: "go 1.27.2-custom\n", refusal: true},
		{name: "duplicate", directive: "go 1.27.2\ngo 1.27\n", refusal: true},
		{name: "leading zero", directive: "go 1.027\n", refusal: true},
	} {
		for _, replacement := range []struct {
			name, text string
			count      uint64
			refusal    bool
		}{
			{name: "none"},
			{name: "local", text: "replace example.invalid/a => ../absent-a\n", count: 1},
			{name: "remote", text: "replace example.invalid/a v1.0.0 => example.invalid/b v1.2.3\n", count: 1},
			{name: "block", text: "replace (\nexample.invalid/a => ../absent-a\nexample.invalid/b => ../absent-b\n)\n", count: 2},
			{name: "comment", text: "// replace example.invalid/a => ../absent-a\n"},
			{name: "unresolved requirement", text: "require example.invalid/unavailable v1.0.0\n"},
			{name: "broken", text: "replace example.invalid/a =>\n", refusal: true},
		} {
			t.Run(version.name+"/"+replacement.name, func(t *testing.T) {
				t.Parallel()
				directory := t.TempDir()
				source := []byte("module example.com/native-matrix\n" + version.directive + replacement.text)
				path := filepath.Join(directory, "go.mod")
				if err := os.WriteFile(path, source, 0600); err != nil {
					t.Fatal(err)
				}
				capability, request := installationFixtureCapability(t, directory, []string{"GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off"})
				observed, err := capability.ObserveModuleFile(t.Context(), request)
				if version.refusal || replacement.refusal {
					var exit CommandExitError
					if observed != (ModuleFileObservation{}) || !errors.Is(err, core.ErrGoToolchainExecution) || !errors.As(err, &exit) || exit.Process.ExitCode == 0 {
						t.Fatalf("native matrix refusal%s/%s got%+v/%v", version.name, replacement.name, observed, err)
					}
				} else if err != nil || observed.Validate() != nil || observed.GoDeclaration != version.presence || observed.GoVersion.String() != version.token || observed.Replacements != replacement.count {
					t.Fatalf("native matrix%s/%s got%+v/%v want%s/%s/%d", version.name, replacement.name, observed, err, version.presence, version.token, replacement.count)
				}
				after, err := os.ReadFile(path)
				if err != nil || !bytes.Equal(source, after) {
					t.Fatalf("native matrix modified source%t error%v", !bytes.Equal(source, after), err)
				}
			})
		}
	}
}

func TestModuleFileProjectionRefusesMalformedNativeRecords(t *testing.T) {
	t.Parallel()
	for _, row := range []struct {
		name, body, version string
		replacements        uint64
		refuse              bool
	}{
		{name: "missing Go remains absence", body: `{}`},
		{name: "empty Go remains absence", body: `{"Go":""}`},
		{name: "Go field has native value", body: `{"Go":"1.27.2"}`, version: "go1.27.2"},
		{name: "null replacements are empty", body: `{"Replace":null}`},
		{name: "empty replacements are empty", body: `{"Replace":[]}`},
		{name: "two replacement objects are counted", body: `{"Replace":[{"Old":{"Path":"a"}},{"New":{"Path":"b"}}]}`, replacements: 2},
		{name: "unknown nested values do not alter projection", body: `{"Go":"1.27.2","Require":[{"Path":"go","Data":[1,true,null,{}]}]}`, version: "go1.27.2"},
		{name: "empty source refuses", refuse: true},
		{name: "nonobject root refuses", body: `[]`, refuse: true},
		{name: "wrong Go kind refuses", body: `{"Go":27}`, refuse: true},
		{name: "invalid Go token refuses", body: `{"Go":"1.027"}`, refuse: true},
		{name: "wrong replacement kind refuses", body: `{"Replace":{}}`, refuse: true},
		{name: "nonobject replacement refuses", body: `{"Replace":[1]}`, refuse: true},
		{name: "truncated replacement refuses", body: `{"Replace":[{`, refuse: true},
		{name: "duplicate Go refuses", body: `{"Go":"1.27","Go":"1.27.2"}`, refuse: true},
		{name: "trailing document refuses", body: `{} {}`, refuse: true},
		{name: "malformed ignored input refuses", body: `{"Require":[{"Path":}]}`, refuse: true},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			got, err := readModuleFileObservation(t.Context(), strings.NewReader(row.body))
			if row.refuse {
				if got != (ModuleFileObservation{}) || !errors.Is(err, core.ErrGoToolchainOutput) {
					t.Fatalf("native JSON refusal%+v/%v", got, err)
				}
				return
			}
			want := ModuleGoDeclarationAbsent
			if row.version != "" {
				want = ModuleGoDeclarationPresent
			}
			if err != nil || got.Validate() != nil || got.GoDeclaration != want || got.GoVersion.String() != row.version || got.Replacements != row.replacements {
				t.Fatalf("native JSON%+v/%v want%s/%s/%d", got, err, want, row.version, row.replacements)
			}
		})
	}
}

type moduleFileFailingReader struct{}

func (moduleFileFailingReader) Read([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestModuleFileAdmissionAndNativeCauses(t *testing.T) {
	t.Parallel()
	capability, request := installationFixtureCapability(t, t.TempDir(), []string{"GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off"})
	ctx, cancel := context.WithCancelCause(t.Context())
	cancel(io.ErrClosedPipe)
	got, err := capability.ObserveModuleFile(ctx, request)
	if got != (ModuleFileObservation{}) || !errors.Is(err, context.Canceled) || !errors.Is(err, io.ErrClosedPipe) || !errors.Is(err, core.ErrGoToolchainExecution) {
		t.Fatalf("module cancelled ingress%+v/%v", got, err)
	}
	expired, release := context.WithDeadlineCause(t.Context(), time.Time{}, io.ErrClosedPipe)
	defer release()
	for _, row := range []struct {
		name        string
		ctx         context.Context
		want, cause error
	}{
		{"nil context refuses module execution", nil, core.ErrGoToolchainContract, nil},
		{"expired deadline retains module cause", expired, context.DeadlineExceeded, io.ErrClosedPipe},
	} {
		t.Run(row.name, func(t *testing.T) {
			t.Parallel()
			got, err := capability.ObserveModuleFile(row.ctx, request)
			if got != (ModuleFileObservation{}) || !errors.Is(err, row.want) || (row.cause != nil && (!errors.Is(err, row.cause) || !errors.Is(err, core.ErrGoToolchainExecution))) {
				t.Fatalf("module context got%+v/%v wantzero/%v and%v", got, err, row.want, row.cause)
			}
		})
	}
	if got, err := (Capability{}).ObserveModuleFile(t.Context(), request); got != (ModuleFileObservation{}) || !errors.Is(err, core.ErrGoToolchainContract) {
		t.Fatalf("module invalid capability%+v/%v", got, err)
	}
	if got, err := capability.ObserveModuleFile(t.Context(), ObservationRequest{}); got != (ModuleFileObservation{}) || !errors.Is(err, core.ErrGoToolchainContract) {
		t.Fatalf("module invalid request%+v/%v", got, err)
	}
	if got, err := readModuleFileObservation(t.Context(), moduleFileFailingReader{}); got != (ModuleFileObservation{}) || !errors.Is(err, io.ErrClosedPipe) || !errors.Is(err, core.ErrGoToolchainOutput) {
		t.Fatalf("module source cause%+v/%v", got, err)
	}
	if got, err := readModuleFileObservation(ctx, strings.NewReader(`{}`)); got != (ModuleFileObservation{}) || !errors.Is(err, io.ErrClosedPipe) || !errors.Is(err, context.Canceled) {
		t.Fatalf("module projection cancel%+v/%v", got, err)
	}
	version, err := ParseToolchainVersion("go1.27.2")
	if err != nil {
		t.Fatal(err)
	}
	for _, value := range []ModuleGoDeclaration{ModuleGoDeclarationUnknown, ModuleGoDeclarationAbsent, ModuleGoDeclarationPresent, 3, 255} {
		want := value == ModuleGoDeclarationAbsent || value == ModuleGoDeclarationPresent
		if value.IsValid() != want || (value.Validate() == nil) != want || value.String() == "" {
			t.Fatalf("module declaration enum%d validity%t error%v name%q", value, value.IsValid(), value.Validate(), value.String())
		}
		value.OffWireEnum()
	}
	for _, row := range []struct {
		name        string
		observation ModuleFileObservation
		valid       bool
	}{
		{name: "unknown observation refuses"},
		{name: "absence with version refuses", observation: ModuleFileObservation{GoDeclaration: ModuleGoDeclarationAbsent, GoVersion: version}},
		{name: "present without version refuses", observation: ModuleFileObservation{GoDeclaration: ModuleGoDeclarationPresent}},
		{name: "absence is valid", observation: ModuleFileObservation{GoDeclaration: ModuleGoDeclarationAbsent}, valid: true},
		{name: "presence is valid", observation: ModuleFileObservation{GoDeclaration: ModuleGoDeclarationPresent, GoVersion: version}, valid: true},
	} {
		t.Run(row.name, func(t *testing.T) {
			if (row.observation.Validate() == nil) != row.valid {
				t.Fatalf("module observation%+v validation%v valid%t", row.observation, row.observation.Validate(), row.valid)
			}
			row.observation.goToolchainProtocolFact()
		})
	}
}

type moduleFilePopulationReader struct {
	remaining, offset, completed int
	baseline, peak               int64
}

const moduleFilePopulationRecord = `{"Path":"example.com/large","Version":"v1.0.0"},`

func (r *moduleFilePopulationReader) Read(data []byte) (int, error) {
	if len(data) == 0 {
		return 0, nil
	}
	if r.remaining == 0 {
		return 0, io.EOF
	}
	n := copy(data, moduleFilePopulationRecord[r.offset:])
	r.offset += n
	if r.offset == len(moduleFilePopulationRecord) {
		r.offset = 0
		r.remaining--
		r.completed++
		if r.completed%1024 == 0 {
			observed, err := hostfacts.ObserveCollectedGoHeap(context.Background())
			if err != nil {
				return n, err
			}
			r.peak = max(r.peak, int64(observed.Uint64())-r.baseline)
		}
	}
	return n, nil
}

func TestModuleFileProjectionDoesNotCaptureDirectivePopulation(t *testing.T) {
	testserial.Declare(t, core.TestIsolationDeclaration{Hazard: core.TestIsolationHazardRuntimeAllocation, Scope: core.TestIsolationScopePackageProcess})
	for _, population := range []int{64, 4097, 65537} {
		t.Run(fmt.Sprint(population), func(t *testing.T) {
			before, err := hostfacts.ObserveCollectedGoHeap(t.Context())
			if err != nil {
				t.Fatal(err)
			}
			source := &moduleFilePopulationReader{remaining: population - 1, baseline: int64(before.Uint64())}
			got, err := readModuleFileObservation(t.Context(), io.MultiReader(strings.NewReader(`{"Go":"1.27.2","Require":[`), source, strings.NewReader(`{"Path":"example.com/final"}],"Replace":[{},{}]}`)))
			if err != nil || got.GoVersion.String() != "go1.27.2" || got.Replacements != 2 || source.remaining != 0 || source.completed != population-1 || source.peak > 256<<10 {
				t.Fatalf("native module population%d result%+v/%v unread%d peak%d want complete fixed projection", population, got, err, source.remaining, source.peak)
			}
			t.Logf("native module population%d live projection growth%d bytes; Go command parsing is outside this observation", population, source.peak)
		})
	}
}

func FuzzObserveNativeModuleFileSemanticProjection(f *testing.F) {
	f.Add(uint8(0))
	f.Add(uint8(7))
	f.Fuzz(func(t *testing.T, mode uint8) {
		body := "module example.com/fuzz-native-module\n"
		version := "go1.27.2"
		presence := ModuleGoDeclarationPresent
		count := uint64(mode & 3)
		if mode&4 != 0 {
			presence = ModuleGoDeclarationAbsent
			version = ""
		} else {
			body += "go 1.27.2\n"
		}
		for i := range count {
			body += fmt.Sprintf("replace example.invalid/module%d => ../missing%d\n", i, i)
		}
		refusal := mode&8 != 0
		if refusal {
			body += "go 1.27.2-custom\n"
		}
		directory := t.TempDir()
		if err := os.WriteFile(filepath.Join(directory, "go.mod"), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		capability, request := installationFixtureCapability(t, directory, []string{"GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off"})
		got, err := capability.ObserveModuleFile(t.Context(), request)
		if refusal {
			var exit CommandExitError
			if got != (ModuleFileObservation{}) || !errors.As(err, &exit) || exit.Process.ExitCode == 0 {
				t.Fatalf("native module fuzz refusal mode%d result%+v/%v", mode, got, err)
			}
			return
		}
		if err != nil || got.Validate() != nil || got.GoVersion.String() != version || got.GoDeclaration != presence || got.Replacements != count {
			t.Fatalf("native module fuzz mode%d result%+v/%v want%s/%s/%d", mode, got, err, presence, version, count)
		}
	})
}
