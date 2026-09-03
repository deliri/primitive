package github

import (
	"bytes"
	"encoding/base64"
	json "encoding/json/v2"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

type externalIngressFuzzContract[Door, FuzzTarget any] struct {
	Door Door
	Fuzz FuzzTarget
}

func TestGitHubExternalIngressHasSemanticFuzzTargets(t *testing.T) {
	t.Parallel()

	_ = externalIngressFuzzContract[func(string) (Repository, error), func(*testing.F)]{
		Door: ParseRepository, Fuzz: FuzzParseRepositorySemanticClosure,
	}
	_ = externalIngressFuzzContract[func([]byte) ([]Tag, error), func(*testing.F)]{
		Door: decodeTagPage, Fuzz: FuzzDecodeGitHubTagPageSemanticClosure,
	}
	_ = externalIngressFuzzContract[func([]byte) (core.BuildCommit, error), func(*testing.F)]{
		Door: decodeHead, Fuzz: FuzzDecodeGitHubHeadSemanticClosure,
	}
	_ = externalIngressFuzzContract[func(FileRequest, []byte) (FileObservation, error), func(*testing.F)]{
		Door: fileObservation, Fuzz: FuzzGitHubFileResponseSemanticClosure,
	}
	_ = externalIngressFuzzContract[func([]byte, temporal.Instant) (installationToken, error), func(*testing.F)]{
		Door: decodeInstallationToken, Fuzz: FuzzDecodeGitHubInstallationAccessSemanticClosure,
	}
	_ = externalIngressFuzzContract[func(io.Reader, uint64, TreeVisitor) (uint64, error), func(*testing.F)]{
		Door: decodeTree, Fuzz: FuzzDecodeGitHubTreeSemanticClosure,
	}
}

func FuzzDecodeGitHubTagPageSemanticClosure(f *testing.F) {
	commit := parsedCommit(f)
	seed, err := json.Marshal([]tagWire{{Name: "v1.2.3", Commit: tagCommitWire{SHA: commit.String()}}})
	if err != nil {
		f.Fatalf("json.Marshal(tag seed) error = %v, want nil", err)
	}
	f.Add(seed)
	f.Add([]byte(`[]`))
	f.Add([]byte{})
	f.Fuzz(func(t *testing.T, payload []byte) {
		got, gotErr := decodeTagPage(payload)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrGitHubResponse) {
				t.Fatalf("decodeTagPage(rejected) error = %v, want %v", gotErr, core.ErrGitHubResponse)
			}
			return
		}
		wire := make([]tagWire, len(got))
		for index, tag := range got {
			if err := tag.Validate(); err != nil {
				t.Fatalf("decodeTagPage(accepted)[%d].Validate() error = %v, want nil", index, err)
			}
			wire[index] = tagWire{Name: tag.Name.String(), Commit: tagCommitWire{SHA: tag.Commit.String()}}
		}
		canonical, err := json.Marshal(wire)
		if err != nil {
			t.Fatalf("json.Marshal(accepted tags) error = %v, want nil", err)
		}
		roundTrip, err := decodeTagPage(canonical)
		if err != nil || len(roundTrip) != len(got) {
			t.Fatalf("decodeTagPage(canonical) = (%v, %v), want %d valid tags", roundTrip, err, len(got))
		}
		for index := range got {
			if roundTrip[index] != got[index] {
				t.Fatalf("decodeTagPage(canonical)[%d] = %v, want %v", index, roundTrip[index], got[index])
			}
		}
	})
}

func FuzzDecodeGitHubHeadSemanticClosure(f *testing.F) {
	commit := parsedCommit(f)
	seed, err := json.Marshal([]headWire{{SHA: commit.String()}})
	if err != nil {
		f.Fatalf("json.Marshal(head seed) error = %v, want nil", err)
	}
	f.Add(seed)
	f.Add([]byte{})
	f.Add([]byte(`[]`))
	f.Fuzz(func(t *testing.T, payload []byte) {
		got, gotErr := decodeHead(payload)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrGitHubResponse) || got != (core.BuildCommit{}) {
				t.Fatalf("decodeHead(rejected) = (%v, %v), want zero and %v", got, gotErr, core.ErrGitHubResponse)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("decodeHead(accepted).Validate() error = %v, want nil", err)
		}
		canonical, err := json.Marshal([]headWire{{SHA: got.String()}})
		if err != nil {
			t.Fatalf("json.Marshal(accepted head) error = %v, want nil", err)
		}
		roundTrip, err := decodeHead(canonical)
		if err != nil || roundTrip != got {
			t.Fatalf("decodeHead(canonical) = (%v, %v), want (%v, nil)", roundTrip, err, got)
		}
	})
}

func FuzzGitHubFileResponseSemanticClosure(f *testing.F) {
	request := FileRequest{
		Repository: parsedRepository(f, "owner/repository"), Commit: parsedCommit(f),
		Path: parsedPath(f, "source/main.go"), MaximumBytes: byteCountFixture(f, 1024),
	}
	content := []byte("package main")
	seed, err := json.Marshal(contentsWire{
		Path: request.Path.String(), Size: uint64(len(content)), Type: "file",
		Encoding: "base64", Content: base64.StdEncoding.EncodeToString(content),
	})
	if err != nil {
		f.Fatalf("json.Marshal(file seed) error = %v, want nil", err)
	}
	f.Add(seed)
	f.Add([]byte{})
	f.Add([]byte(`{}`))
	f.Fuzz(func(t *testing.T, payload []byte) {
		got, gotErr := fileObservation(request, payload)
		if gotErr != nil {
			zero := got.Repository == (Repository{}) && got.Commit == (core.BuildCommit{}) &&
				got.Path == (core.SourcePath{}) && got.Length == (core.ByteLength{}) &&
				got.SHA256 == (core.SHA256Digest{}) && len(got.Content) == 0
			if !errors.Is(gotErr, core.ErrGitHubResponse) || !zero {
				t.Fatalf("fileObservation(rejected) = (%v, %v), want zero and %v", got, gotErr, core.ErrGitHubResponse)
			}
			return
		}
		if err := got.Validate(); err != nil {
			t.Fatalf("fileObservation(accepted).Validate() error = %v, want nil", err)
		}
		canonical, err := json.Marshal(contentsWire{
			Path: got.Path.String(), Size: got.Length.Uint64(), Type: "file",
			Encoding: "base64", Content: base64.StdEncoding.EncodeToString(got.Content),
		})
		if err != nil {
			t.Fatalf("json.Marshal(accepted file) error = %v, want nil", err)
		}
		roundTrip, err := fileObservation(request, canonical)
		if err != nil || roundTrip.Repository != got.Repository || roundTrip.Commit != got.Commit ||
			roundTrip.Path != got.Path || roundTrip.Length != got.Length || roundTrip.SHA256 != got.SHA256 || !bytes.Equal(roundTrip.Content, got.Content) {
			t.Fatalf("fileObservation(canonical) = (%v, %v), want exact accepted observation", roundTrip, err)
		}
	})
}

func FuzzDecodeGitHubInstallationAccessSemanticClosure(f *testing.F) {
	now, err := temporal.NewInstant(time.Date(2026, time.September, 3, 10, 0, 0, 0, time.UTC))
	if err != nil {
		f.Fatalf("temporal.NewInstant(fixed) error = %v, want nil", err)
	}
	expiresAt, err := now.Add(durationMinutesFixture(f, 60))
	if err != nil {
		f.Fatalf("fixed expiry construction error = %v, want nil", err)
	}
	expiresText, err := expiresAt.RFC3339()
	if err != nil {
		f.Fatalf("fixed expiry projection error = %v, want nil", err)
	}
	seed, err := json.Marshal(struct {
		Token     string `json:"token"`
		ExpiresAt string `json:"expires_at"`
	}{Token: "ghs_fixed", ExpiresAt: expiresText})
	if err != nil {
		f.Fatalf("json.Marshal(installation access seed) error = %v, want nil", err)
	}
	f.Add(seed)
	f.Add([]byte{})
	f.Add([]byte(`{}`))
	f.Fuzz(func(t *testing.T, payload []byte) {
		got, gotErr := decodeInstallationToken(payload, now)
		if gotErr != nil {
			if !errors.Is(gotErr, core.ErrGitHubResponse) || got != (installationToken{}) {
				t.Fatalf("decodeInstallationToken(rejected) = (%v, %v), want zero and %v", got, gotErr, core.ErrGitHubResponse)
			}
			return
		}
		usable, usableErr := got.usable(now)
		if usableErr != nil || !usable {
			t.Fatalf("decodeInstallationToken(accepted).usable() = (%t, %v), want true and nil", usable, usableErr)
		}
	})
}

func durationMinutesFixture(t testing.TB, minutes uint64) temporal.Duration {
	t.Helper()
	value, err := temporal.DurationFromMinutes(minutes)
	if err != nil {
		t.Fatalf("temporal.DurationFromMinutes(%d) error = %v, want nil", minutes, err)
	}
	return value
}
