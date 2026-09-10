package filestore_test

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

func TestActivationRequestClosesEveryPreEffectBoundary(t *testing.T) {
	t.Parallel()

	root := requireTestRoot(t, t.TempDir())
	for _, size := range []uint64{0, 1, 2, 32<<10 - 1, 32 << 10, 32<<10 + 1, 64<<10 - 1, 64 << 10, 64<<10 + 1, uint64(^uint64(0) >> 1)} {
		expected, err := core.NewByteLength(size)
		if err != nil {
			t.Fatalf("core.NewByteLength(%d) error = %v, want nil", size, err)
		}
		request := filestore.ActivationRequest{
			Temporary: filestore.Location{Root: root, Path: mustRelativePath(t, ".stage")},
			Target:    mustRelativePath(t, "target"), ExpectedBytes: expected,
			Mode: 0o600, Install: filestore.InstallReplace,
		}
		if err := request.Validate(); err != nil {
			t.Fatalf("ActivationRequest.Validate(%d) error = %v, want nil", size, err)
		}
		stage := request.StageDestination()
		if err := stage.Validate(); err != nil || stage.Temporary != request.Temporary ||
			stage.ExpectedBytes != expected || stage.Mode != request.Mode {
			t.Fatalf("ActivationRequest.StageDestination(%d) = (%v, %v), want exact plan projection", size, stage, err)
		}
	}

	valid := filestore.ActivationRequest{
		Temporary: filestore.Location{Root: root, Path: mustRelativePath(t, ".stage")},
		Target:    mustRelativePath(t, "target"), ExpectedBytes: mustActivationLength(t, 1),
		Mode: 0o600, Install: filestore.InstallReplace,
	}
	cases := []struct {
		mutate func(*filestore.ActivationRequest)
		name   string
	}{
		{name: "zero request", mutate: func(request *filestore.ActivationRequest) { *request = filestore.ActivationRequest{} }},
		{name: "missing root", mutate: func(request *filestore.ActivationRequest) { request.Temporary.Root = nil }},
		{name: "zero temporary", mutate: func(request *filestore.ActivationRequest) { request.Temporary.Path = core.RelativePath{} }},
		{name: "root temporary", mutate: func(request *filestore.ActivationRequest) { request.Temporary.Path = mustRelativePath(t, ".") }},
		{name: "zero target", mutate: func(request *filestore.ActivationRequest) { request.Target = core.RelativePath{} }},
		{name: "root target", mutate: func(request *filestore.ActivationRequest) { request.Target = mustRelativePath(t, ".") }},
		{name: "target equals temporary", mutate: func(request *filestore.ActivationRequest) { request.Target = request.Temporary.Path }},
		{name: "zero mode", mutate: func(request *filestore.ActivationRequest) { request.Mode = 0 }},
		{name: "typed mode bits", mutate: func(request *filestore.ActivationRequest) { request.Mode = fs.ModeDir | 0o700 }},
		{name: "zero install", mutate: func(request *filestore.ActivationRequest) { request.Install = filestore.InstallUnknown }},
		{name: "install above domain", mutate: func(request *filestore.ActivationRequest) { request.Install = filestore.InstallMode(255) }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			request := valid
			tc.mutate(&request)
			if err := request.Validate(); !errors.Is(err, core.ErrFilestoreContract) {
				t.Fatalf("ActivationRequest.Validate(%s) error = %v, want errors.Is %v", tc.name, err, core.ErrFilestoreContract)
			}
		})
	}
}

type activationAgreementClass uint8

const (
	activationAgreementBoundary activationAgreementClass = iota + 1
	activationAgreementContradiction
	activationAgreementRefusal
	activationAgreementNeutral
)

type activationAgreementChange uint8

const (
	activationAgreementForeignRoot activationAgreementChange = 1 << iota
	activationAgreementForeignPath
	activationAgreementForeignExtent
	activationAgreementForeignMode
	activationAgreementAbsentReceipt
	activationAgreementDomain
)

// Exhaust every subset of the five independent agreement changes. Each row
// obtains its receipt from the real producer before the classifier runs; exact
// and empty stages are controls against both fabricated custody and refuse-all.
func TestActivationStageAgreementLayerTriad(t *testing.T) {
	t.Parallel()
	payloads := []struct {
		name string
		data []byte
	}{
		{name: "binary stage retains both bytes", data: []byte{0, 0xff}},
		{name: "empty stage is a real receipt without invented content"},
	}
	changes := []struct {
		bit  activationAgreementChange
		name string
	}{
		{activationAgreementForeignRoot, "foreign rooted capability"},
		{activationAgreementForeignPath, "foreign temporary name"},
		{activationAgreementForeignExtent, "foreign byte extent"},
		{activationAgreementForeignMode, "foreign permission mode"},
		{activationAgreementAbsentReceipt, "absent stage receipt"},
	}
	for _, payload := range payloads {
		t.Run(payload.name, func(t *testing.T) {
			t.Parallel()
			cases := make([]struct {
				name    string
				changes activationAgreementChange
				wantErr error
				class   activationAgreementClass
			}, 0, int(activationAgreementDomain))
			for bits := range activationAgreementDomain {
				name := "exact agreement alone authorizes activation"
				var labels []string
				for _, change := range changes {
					if bits&change.bit != 0 {
						labels = append(labels, change.name)
					}
				}
				class := activationAgreementBoundary
				if len(payload.data) == 0 {
					class = activationAgreementNeutral
				}
				if bits != 0 {
					class = activationAgreementContradiction
				}
				if bits&activationAgreementAbsentReceipt != 0 {
					class = activationAgreementRefusal
				}
				var wantErr error
				if bits != 0 {
					name = strings.Join(labels, " plus ") + " cannot authorize activation"
					wantErr = core.ErrFilestoreContract
				}
				cases = append(cases, struct {
					name    string
					changes activationAgreementChange
					wantErr error
					class   activationAgreementClass
				}{name, bits, wantErr, class})
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					t.Parallel()
					directory, foreignDirectory := t.TempDir(), t.TempDir()
					root, foreignRoot := requireTestRoot(t, directory), requireTestRoot(t, foreignDirectory)
					stagePath, target := mustRelativePath(t, ".planned"), mustRelativePath(t, "target")
					staged, err := filestore.Stage(t.Context(), filestore.StageRequest{
						Temporary: filestore.Location{Root: root, Path: stagePath}, Source: bytes.NewReader(payload.data), Mode: 0o600,
					})
					if err != nil {
						t.Fatalf("stage producer = %v, want nil", err)
					}
					if err := staged.Validate(); err != nil || staged.Path() != stagePath || staged.BytesWritten().Uint64() != uint64(len(payload.data)) {
						t.Fatalf("producer receipt = (%v,%v), want exact temporary and %d bytes", staged, err, len(payload.data))
					}
					plan := filestore.ActivationRequest{Temporary: filestore.Location{Root: root, Path: stagePath}, Target: target, ExpectedBytes: mustActivationLength(t, uint64(len(payload.data))), Mode: 0o600, Install: filestore.InstallCreate}
					input := staged
					if tc.changes&activationAgreementForeignRoot != 0 {
						plan.Temporary.Root = foreignRoot
					}
					if tc.changes&activationAgreementForeignPath != 0 {
						plan.Temporary.Path = mustRelativePath(t, ".different")
					}
					if tc.changes&activationAgreementForeignExtent != 0 {
						plan.ExpectedBytes = mustActivationLength(t, uint64(len(payload.data)+1))
					}
					if tc.changes&activationAgreementForeignMode != 0 {
						plan.Mode = 0o400
					}
					if tc.changes&activationAgreementAbsentReceipt != 0 {
						input = filestore.StagedFile{}
					}
					if err := plan.Validate(); err != nil {
						t.Fatalf("individually valid foreign agreement = %v, want nil", err)
					}
					if tc.class == activationAgreementRefusal {
						if err := input.Validate(); !errors.Is(err, core.ErrFilestoreContract) {
							t.Fatalf("absent producer receipt = %v, want typed refusal", err)
						}
					} else if err := input.Validate(); err != nil {
						t.Fatalf("admitted producer receipt = %v, want nil before agreement comparison", err)
					}
					got, gotErr := plan.CommitRequest(input)
					if !errors.Is(gotErr, tc.wantErr) {
						t.Fatalf("stage-to-activation binding = (%v,%v), want %v", got, gotErr, tc.wantErr)
					}
					sourceBytes, readErr := root.ReadFile(stagePath.String())
					info, statErr := root.Stat(stagePath.String())
					if readErr != nil || statErr != nil || !bytes.Equal(sourceBytes, payload.data) || info.Mode().Perm() != 0o600 {
						t.Fatalf("binding changed the real stage: read=%v stat=%v bytes=%x", readErr, statErr, sourceBytes)
					}
					for _, capability := range []*os.Root{root, foreignRoot} {
						if _, err := capability.Lstat(target.String()); !errors.Is(err, fs.ErrNotExist) {
							t.Fatalf("binding created a target: %v", err)
						}
					}
					if tc.wantErr != nil {
						if got != (filestore.CommitRequest{}) {
							t.Fatalf("refused binding retained executable request: %+v", got)
						}
						return
					}
					if err := got.Validate(); err != nil || got.Staged != staged || got.Target != target || got.Install != plan.Install {
						t.Fatalf("bound request = (%v,%v), want exact receipt, target and install intent", got, err)
					}
					if err := filestore.Commit(t.Context(), got); err != nil {
						t.Fatalf("bound activation = %v, want nil", err)
					}
					stored, err := root.ReadFile(target.String())
					if err != nil || !bytes.Equal(stored, payload.data) {
						t.Fatalf("activated bytes = (%x,%v), want exact %x", stored, err, payload.data)
					}
					if _, err := root.Lstat(stagePath.String()); !errors.Is(err, fs.ErrNotExist) {
						t.Fatalf("consumed stage = %v, want absent", err)
					}
					if entries, err := os.ReadDir(foreignDirectory); err != nil || len(entries) != 0 {
						t.Fatalf("foreign root entries = (%v,%v), want empty", entries, err)
					}
				})
			}
		})
	}
}

func mustActivationLength(t *testing.T, value uint64) core.ByteLength {
	t.Helper()
	length, err := core.NewByteLength(value)
	if err != nil {
		t.Fatalf("core.NewByteLength(%d) error = %v, want nil", value, err)
	}
	return length
}
