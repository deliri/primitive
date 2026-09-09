package distribution_test

import (
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/distribution"
	"github.com/deliri/primitive/v2026/filestore"
	"github.com/deliri/primitive/v2026/release"
	"io"
	"net/http"
	"testing"
)

type forbiddenStageTransport struct{ calls int }

func (w *forbiddenStageTransport) RoundTrip(*http.Request) (*http.Response, error) {
	w.calls++
	return nil, io.ErrClosedPipe
}

func preparedDistributionRelease(t testing.TB) release.PreparedRelease {
	t.Helper()
	installed := newReleaseFixture(t, core.NewReleaseVersion(2026, 0, 54), 1)
	candidate := newReleaseFixture(t, core.NewReleaseVersion(2026, 0, 55), 2)
	cache, err := release.NewCachedLatest(candidate.latest)
	if err != nil {
		t.Fatalf("NewCachedLatest()=%v, want nil", err)
	}
	selection, err := release.EvaluateInstalled(release.EvaluateInstalledRequest{
		Installed: installed.builds[2],
		Evaluate:  release.EvaluateRequest{InstalledManifest: installed.manifest, Latest: cache, Time: distributionLatestTimeEvidence(t, 3_000)},
	})
	if err != nil {
		t.Fatalf("EvaluateInstalled()=%v, want nil", err)
	}
	available, ok := selection.Available()
	if !ok {
		t.Fatalf("Selection.Available()=%t, want true", ok)
	}
	preparation, err := available.Prepare(distributionLatestTimeEvidence(t, 3_000))
	if err != nil {
		t.Fatalf("Prepare()=%v, want nil", err)
	}
	got, ok := preparation.Ready()
	if !ok {
		t.Fatalf("Preparation.Ready()=%t, want true", ok)
	}
	return got
}

func TestUpgradeStageProjectionLayerTriad(t *testing.T) {
	t.Parallel()
	f := newUpgradeExchangeFixture(t)
	grant, err := distribution.VerifyUpgradeGrant(upgradeGrantExpectation(f, f.grantDoc))
	if err != nil {
		t.Fatalf("VerifyUpgradeGrant()=%v, want nil", err)
	}
	prepared := preparedDistributionRelease(t)
	cases := []struct {
		name                    string
		foreign, closed, absent bool
		wantErr                 error
	}{
		{name: "same directory returns exact unexecuted capability"},
		{name: "foreign directory refuses all stage output", foreign: true, wantErr: core.ErrDistributionContract},
		{name: "closed root refuses all stage output", closed: true, wantErr: core.ErrDistributionContract},
		{name: "absent root refuses all stage output", absent: true, wantErr: core.ErrDistributionContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			absolute, err := core.ParseAbsolutePath(dir)
			if err != nil {
				t.Fatalf("ParseAbsolutePath()=%v, want nil", err)
			}
			root, err := filestore.OpenRoot(t.Context(), absolute)
			if err != nil {
				t.Fatalf("OpenRoot()=%v, want nil", err)
			}
			if tc.closed {
				if err := root.Close(); err != nil {
					t.Fatalf("Close()=%v, want nil", err)
				}
			} else {
				t.Cleanup(func() {
					if err := root.Close(); err != nil {
						t.Errorf("Close()=%v, want nil", err)
					}
				})
			}
			inputRoot := root
			if tc.absent {
				inputRoot = nil
			}
			directory := absolute
			if tc.foreign {
				other := t.TempDir()
				directory, err = core.ParseAbsolutePath(other)
				if err != nil {
					t.Fatalf("ParseAbsolutePath(other)=%v, want nil", err)
				}
			}
			transport := &forbiddenStageTransport{}
			request := distribution.UpgradeStageRequest{Root: inputRoot, Directory: directory, Grant: grant, Prepared: prepared, Client: objectstoreClient(t, transport), Policy: objectstorePolicy(t)}
			validationErr := request.Validate()
			got, gotErr := distribution.PrepareUpgradeStage(request)
			if !errors.Is(gotErr, tc.wantErr) || !errors.Is(validationErr, tc.wantErr) || transport.calls != 0 {
				t.Fatalf("Validate/PrepareUpgradeStage()=(%v,%v,%d calls), want (%v,%v,0)", validationErr, gotErr, transport.calls, tc.wantErr, tc.wantErr)
			}
			if tc.wantErr != nil {
				if got.Root != nil || got.Directory != (core.AbsolutePath{}) || got.Prepared != (release.PreparedRelease{}) || got.Source.Observer != nil || got.Source.Client.Validate() == nil {
					t.Fatalf("refused stage=%v, want zero request", got)
				}
			} else {
				if got.Root != root || got.Directory != absolute || got.Prepared != prepared || got.Source.Policy != request.Policy || got.Validate() != nil {
					t.Fatalf("stage=%v, want exact valid handoff", got)
				}
			}
		})
	}
}
