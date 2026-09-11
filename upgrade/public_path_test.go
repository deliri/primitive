package upgrade

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/release"
	"github.com/deliri/primitive/v2026/temporal"
)

type publicUpgradeFixture struct {
	installed release.VerifiedManifest
	build     core.BuildIdentity
	prepared  release.PreparedRelease
}

func publicFixture(t testing.TB) publicUpgradeFixture {
	t.Helper()
	offering := upgradeOffering(t, 4)
	installed := newReleaseFixtureForOffering(t, offering, core.NewReleaseVersion(1, 0, 0), []byte("installed"), 1)
	candidate := newReleaseFixtureForOffering(t, offering, core.NewReleaseVersion(2, 0, 0), []byte("candidate"), 2)
	build := artifactForTest(t, []byte("installed"), 1).Build()
	cached, err := release.NewCachedLatest(candidate.verifiedLatest)
	if err != nil {
		t.Fatalf("NewCachedLatest error = %v, want nil", err)
	}
	evidence := latestTimeEvidenceAt(t, 3000)
	selection, err := release.EvaluateInstalled(release.EvaluateInstalledRequest{Installed: build, Evaluate: release.EvaluateRequest{InstalledManifest: installed.verified, Latest: cached, Time: evidence}})
	if err != nil {
		t.Fatalf("EvaluateInstalled error = %v, want nil", err)
	}
	available, ok := selection.Available()
	if !ok {
		t.Fatal("Selection.Available = false, want candidate")
	}
	preparation, err := available.Prepare(evidence)
	if err != nil {
		t.Fatalf("Available.Prepare error = %v, want nil", err)
	}
	ready, ok := preparation.Ready()
	if !ok {
		t.Fatal("Preparation.Ready = false, want authenticated release")
	}
	return publicUpgradeFixture{installed: installed.verified, build: build, prepared: ready}
}

func TestPublicBootstrapStagePromotionLayerTriad(t *testing.T) {
	t.Parallel()
	fixture := publicFixture(t)
	for _, mode := range []string{"promote verified candidate", "discard failed product trial", "repeat stage retains exact candidate", "canceled stage creates no candidate", "foreign payload produces no target", "changed selector refuses authenticated handoff"} {
		t.Run(mode, func(t *testing.T) {
			t.Parallel()
			root, dir := stageRootForTest(t, t.TempDir())
			installed, err := Bootstrap(t.Context(), BootstrapRequest{Root: root, Directory: dir, Build: fixture.build, Manifest: fixture.installed, Source: bytes.NewReader([]byte("installed"))})
			if err != nil || installed.Artifact().Build() != fixture.build {
				t.Fatalf("Bootstrap = (%v,%v), want authenticated installed build", installed, err)
			}
			payload := []byte("candidate")
			if mode == "foreign payload produces no target" {
				payload = []byte("wrongdata")
			}
			transport := &stageDownloadTransport{payload: payload}
			request := StageRequest{Root: root, Directory: dir, Prepared: fixture.prepared, Source: stageDownloadSourceForTest(t, stageDownloadSourceFixture{objectName: "bucket/candidate", transport: transport})}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			if mode == "canceled stage creates no candidate" {
				cancel()
			}
			if mode == "changed selector refuses authenticated handoff" {
				altered := artifactForTest(t, []byte("another installed"), 1)
				changed := selectionDocument{Revision: selectionRevisionCurrent, Slot: SlotA, Artifact: altered}
				data, err := encodeSelection(changed)
				if err != nil {
					t.Fatalf("encode changed selector error = %v, want nil", err)
				}
				if err := root.WriteFile(selectionFilename, data, documentMode); err != nil {
					t.Fatalf("write changed selector error = %v, want nil", err)
				}
			}
			target, err := Stage(ctx, request)
			var wantErr error
			switch mode {
			case "canceled stage creates no candidate":
				wantErr = context.Canceled
			case "foreign payload produces no target":
				wantErr = core.ErrUpgradeDownload
			case "changed selector refuses authenticated handoff":
				wantErr = core.ErrUpgradeConflict
			}
			if wantErr != nil {
				if !errors.Is(err, wantErr) || target != (TrialTarget{}) {
					t.Fatalf("Stage = (%v,%v), want zero target and %v", target, err, wantErr)
				}
				if _, err := root.Stat(SlotB.String()); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("candidate slot stat error = %v, want absent", err)
				}
				return
			}
			if err != nil || target.Candidate() != artifactForTest(t, []byte("candidate"), 2) {
				t.Fatalf("Stage = (%v,%v), want exact authenticated candidate", target, err)
			}
			primary, err := ResolvePrimary(t.Context(), ResolveRequest{Root: root, Directory: dir})
			if err != nil || primary != installed {
				t.Fatalf("ResolvePrimary(staged) = (%v,%v), want unchanged installed", primary, err)
			}
			if mode == "repeat stage retains exact candidate" {
				again, err := Stage(t.Context(), request)
				if err != nil || again != target {
					t.Fatalf("Stage(replay) = (%v,%v), want identical target", again, err)
				}
			}
			if mode == "discard failed product trial" {
				promotion, err := CompleteTrial(TrialReport{Target: target, Observed: target.Candidate().Build(), Outcome: TrialFailed, Observation: temporal.InstantFromNanoseconds(4000)})
				if !errors.Is(err, core.ErrUpgradeTrial) || promotion != (Promotion{}) {
					t.Fatalf("CompleteTrial(failed) = (%v,%v), want zero promotion and trial refusal", promotion, err)
				}
				if err := DiscardTrial(t.Context(), DiscardTrialRequest{Root: root, Directory: dir, Target: target}); err != nil {
					t.Fatalf("DiscardTrial error = %v, want nil", err)
				}
				got, err := ResolvePrimary(t.Context(), ResolveRequest{Root: root, Directory: dir})
				if err != nil || got != installed {
					t.Fatalf("ResolvePrimary(discarded) = (%v,%v), want installed", got, err)
				}
			} else {
				promotion, err := CompleteTrial(TrialReport{Target: target, Observed: target.Candidate().Build(), Outcome: TrialPassed, Observation: temporal.InstantFromNanoseconds(4000)})
				if err != nil {
					t.Fatalf("CompleteTrial(passed) error = %v, want nil", err)
				}
				got, err := Promote(t.Context(), PromoteRequest{Root: root, Directory: dir, Promotion: promotion})
				if err != nil || got.Artifact() != target.Candidate() {
					t.Fatalf("Promote = (%v,%v), want exact candidate", got, err)
				}
				if _, err := root.Stat(SlotA.String()); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("old slot stat error = %v, want absent", err)
				}
			}
		})
	}
}

func TestBootstrapTypedNilReaderRejectedAtWall(t *testing.T) {
	t.Parallel()
	fixture := publicFixture(t)
	root, dir := stageRootForTest(t, t.TempDir())
	var source *bytes.Reader
	request := BootstrapRequest{Root: root, Directory: dir, Build: fixture.build, Manifest: fixture.installed, Source: source}
	if err := request.Validate(); !errors.Is(err, core.ErrUpgradeContract) {
		t.Fatalf("BootstrapRequest.Validate(typed nil) = %v, want contract refusal", err)
	}
	got, err := Bootstrap(t.Context(), request)
	if !errors.Is(err, core.ErrUpgradeContract) || got != (Primary{}) {
		t.Fatalf("Bootstrap(typed nil) = (%v,%v), want zero and contract refusal", got, err)
	}
	if _, err := root.Stat(SlotA.String()); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("typed nil slot stat error = %v, want absent", err)
	}
}

// FuzzPublicBootstrapStream binds filesystem effects to the independently signed
// manifest. The only accepted executable is the exact genuinely signed seed.
func FuzzPublicBootstrapStream(f *testing.F) {
	fixture := publicFixture(f)
	f.Add([]byte("installed"))
	f.Add([]byte("foreign"))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		root, dir := stageRootForTest(t, t.TempDir())
		got, err := Bootstrap(t.Context(), BootstrapRequest{Root: root, Directory: dir, Build: fixture.build, Manifest: fixture.installed, Source: bytes.NewReader(data)})
		if bytes.Equal(data, []byte("installed")) {
			if err != nil || got.Artifact().Build() != fixture.build {
				t.Fatalf("Bootstrap signed bytes = (%v,%v), want exact build", got, err)
			}
			replay, replayErr := ResolvePrimary(t.Context(), ResolveRequest{Root: root, Directory: dir})
			if replayErr != nil || replay != got {
				t.Fatalf("ResolvePrimary = (%v,%v), want exact committed primary", replay, replayErr)
			}
		} else {
			if !errors.Is(err, core.ErrUpgradePersistence) || got != (Primary{}) {
				t.Fatalf("Bootstrap foreign bytes = (%v,%v), want zero and persistence refusal", got, err)
			}
			file, openErr := root.Open(".")
			if openErr != nil {
				t.Fatalf("Open root error = %v, want nil", openErr)
			}
			names, readErr := file.Readdirnames(1)
			closeErr := file.Close()
			if len(names) != 0 || !errors.Is(readErr, io.EOF) || closeErr != nil {
				t.Fatalf("refused bootstrap tree = (%v,%v,%v), want empty", names, readErr, closeErr)
			}
		}
	})
}

// The transport is local and synthetic; Stage, Objectstore download, filesystem
// installation, and authenticated artifact verification are production paths.
func FuzzStageDownloadedBytes(f *testing.F) {
	fixture := publicFixture(f)
	f.Add([]byte("candidate"))
	f.Add([]byte("wrongdata"))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, data []byte) {
		root, dir := stageRootForTest(t, t.TempDir())
		installed, err := Bootstrap(t.Context(), BootstrapRequest{Root: root, Directory: dir, Build: fixture.build, Manifest: fixture.installed, Source: bytes.NewReader([]byte("installed"))})
		if err != nil {
			t.Fatalf("Bootstrap seed error = %v, want nil", err)
		}
		source := stageDownloadSourceForTest(t, stageDownloadSourceFixture{objectName: "bucket/candidate", transport: &stageDownloadTransport{payload: data}})
		target, err := Stage(t.Context(), StageRequest{Root: root, Directory: dir, Prepared: fixture.prepared, Source: source})
		if bytes.Equal(data, []byte("candidate")) {
			if err != nil || target.Candidate() != artifactForTest(t, data, 2) {
				t.Fatalf("Stage signed bytes = (%v,%v), want exact signed candidate", target, err)
			}
			if err := requireTrialReceipt(t.Context(), root, target); err != nil {
				t.Fatalf("persisted candidate receipt error = %v, want nil", err)
			}
			if err := verifyArtifact(t.Context(), root, target.slot, target.Candidate()); err != nil {
				t.Fatalf("staged artifact verification error = %v, want nil", err)
			}
		} else {
			if target != (TrialTarget{}) || (!errors.Is(err, core.ErrUpgradeDownload) && !errors.Is(err, core.ErrUpgradeVerification)) {
				t.Fatalf("Stage foreign bytes = (%v,%v), want zero and typed refusal", target, err)
			}
			if _, err := root.Stat(SlotB.String()); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("refused stage slot error = %v, want absent", err)
			}
		}
		primary, err := ResolvePrimary(t.Context(), ResolveRequest{Root: root, Directory: dir})
		if err != nil || primary != installed {
			t.Fatalf("ResolvePrimary after stage = (%v,%v), want unchanged installed", primary, err)
		}
	})
}
