package filestore_test

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/filestore"
)

type acceptanceFixture struct {
	Destination *filestore.AppendDestination
	Root        *os.Root
	Request     filestore.AppendRequest
	Directory   string
}

func newAcceptanceFixture(t testing.TB, existing []byte) acceptanceFixture {
	t.Helper()
	dir := t.TempDir()
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := root.Close(); err != nil {
			t.Error(err)
		}
	})
	path, err := core.ParseRelativePath("first")
	if err != nil {
		t.Fatal(err)
	}
	request := filestore.AppendRequest{Location: filestore.Location{Root: root, Path: path}, Mode: 0600, Append: filestore.AppendCreate}
	if existing != nil {
		if err := os.WriteFile(dir+"/first", existing, 0600); err != nil {
			t.Fatal(err)
		}
		request.Append = filestore.AppendExisting
	}
	destination, err := filestore.OpenAppendDestination(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := destination.Close(); err != nil {
			t.Error(err)
		}
	})
	return acceptanceFixture{Destination: destination, Root: root, Request: request, Directory: dir}
}

func TestAppendAcceptanceObservesActualWritesAcrossNativeLifetimes(t *testing.T) {
	t.Parallel()
	for _, size := range []int{0, 1, 4095, 4096, 4097, 65537} {
		t.Run(fmt.Sprintf("bytes_%d", size), func(t *testing.T) {
			t.Parallel()
			fixture := newAcceptanceFixture(t, nil)
			d := fixture.Destination
			got, err := d.AcceptedBytes()
			if err != nil || got.Uint64() != 0 {
				t.Fatalf("initial acceptance=%v/%v,want zero", got, err)
			}
			payload := bytes.Repeat([]byte{'a'}, size)
			n, err := d.Write(payload)
			if err != nil || n != len(payload) {
				t.Fatalf("native write=%d/%v,want%d", n, err, len(payload))
			}
			if err := d.Sync(); err != nil {
				t.Fatal(err)
			}
			file, err := os.ReadFile(fixture.Directory + "/first")
			if err != nil || !bytes.Equal(file, payload) {
				t.Fatalf("native file=%d/error=%v,want exact%d", len(file), err, len(payload))
			}
			discarded := []byte("discard")
			added, err := d.Write(discarded)
			if err != nil {
				t.Fatal(err)
			}
			if count, err := d.DiscardBuffered(); err != nil || count != added {
				t.Fatalf("discard=%d/%v,want%d", count, err, added)
			}
			if err := d.Truncate(t.Context(), core.ByteLength{}); err != nil {
				t.Fatal(err)
			}
			got, err = d.AcceptedBytes()
			want := uint64(n + added)
			if err != nil || got.Uint64() != want {
				t.Fatalf("historical acceptance=%v/%v,want%d despite discard/truncate", got, err, want)
			}
			// A direct native file change is observable extent, not this owner's Write.
			if err := os.WriteFile(fixture.Directory+"/first", []byte("outside"), 0600); err != nil {
				t.Fatal(err)
			}
			got, err = d.AcceptedBytes()
			if err != nil || got.Uint64() != want {
				t.Fatalf("external extent manufactured acceptance=%v/%v,want%d", got, err, want)
			}
			path, err := core.ParseRelativePath("second")
			if err != nil {
				t.Fatal(err)
			}
			incoming := fixture.Request
			incoming.Location.Path = path
			if err := d.Rotate(t.Context(), incoming); err != nil {
				t.Fatal(err)
			}
			n, err = d.Write([]byte("tail"))
			if err != nil {
				t.Fatal(err)
			}
			want += uint64(n)
			if err := d.Close(); err != nil {
				t.Fatal(err)
			}
			got, err = d.AcceptedBytes()
			if err != nil || got.Uint64() != want {
				t.Fatalf("settled rotation acceptance=%v/%v,want%d", got, err, want)
			}
			if n, err := d.Write([]byte("refused")); n != 0 || !errors.Is(err, core.ErrFilestoreContract) {
				t.Fatalf("settled write=%d/%v", n, err)
			}
			got, err = d.AcceptedBytes()
			if err != nil || got.Uint64() != want {
				t.Fatalf("refusal changed history=%v/%v,want%d", got, err, want)
			}
			tail, err := os.ReadFile(fixture.Directory + "/second")
			if err != nil || string(tail) != "tail" {
				t.Fatalf("rotated file=%q/%v,want tail", tail, err)
			}
		})
	}
}

func TestAppendAcceptanceExcludesExistingBytesAndSurvivesRotationRefusal(t *testing.T) {
	t.Parallel()
	for _, admitted := range []bool{false, true} {
		t.Run(fmt.Sprintf("admitted_%t", admitted), func(t *testing.T) {
			t.Parallel()
			fixture := newAcceptanceFixture(t, []byte("preexisting"))
			d := fixture.Destination
			initial, err := d.AcceptedBytes()
			if err != nil || initial.Uint64() != 0 {
				t.Fatalf("existing bytes became acceptance=%v/%v", initial, err)
			}
			if _, err := d.Write([]byte("ours")); err != nil {
				t.Fatal(err)
			}
			incoming := fixture.Request
			incoming.Append = filestore.AppendCreate
			if !admitted {
				incoming.Append = filestore.AppendExisting
			}
			err = d.Rotate(t.Context(), incoming)
			if err == nil {
				t.Fatal("invalid/conflicting rotation succeeded")
			}
			got, observeErr := d.AcceptedBytes()
			if observeErr != nil || got.Uint64() != 4 {
				t.Fatalf("refused rotation history=%v/%v,want4", got, observeErr)
			}
			if err := d.Close(); err != nil {
				t.Fatal(err)
			}
			file, err := os.ReadFile(fixture.Directory + "/first")
			if err != nil || string(file) != "preexistingours" {
				t.Fatalf("refused rotation file=%q/%v", file, err)
			}
		})
	}
}

func TestAppendAcceptanceRejectsUnsetAndCopiedOwners(t *testing.T) {
	t.Parallel()
	fixture := newAcceptanceFixture(t, nil)
	_, err := fixture.Destination.Write([]byte("source"))
	if err != nil {
		t.Fatal(err)
	}
	copied := *fixture.Destination
	for _, owner := range []*filestore.AppendDestination{nil, {}, &copied} {
		got, err := owner.AcceptedBytes()
		if !errors.Is(err, core.ErrFilestoreContract) || got != (core.ByteLength{}) {
			t.Fatalf("unowned acceptance=%v/%v,want zero/refusal", got, err)
		}
	}
}

func FuzzAppendAcceptanceMatchesGoWriteReturns(f *testing.F) {
	f.Add([]byte("payload"), uint8(1))
	f.Add([]byte{}, uint8(0))
	f.Add(bytes.Repeat([]byte{'x'}, 4097), uint8(7))
	f.Fuzz(func(t *testing.T, body []byte, parts uint8) {
		if len(body) > 1<<20 {
			return
		}
		fixture := newAcceptanceFixture(t, nil)
		d := fixture.Destination
		var accepted uint64
		step := max(1, len(body)/max(1, int(parts)))
		for start := 0; start < len(body); start += step {
			end := min(len(body), start+step)
			n, err := d.Write(body[start:end])
			if err != nil {
				t.Fatal(err)
			}
			accepted += uint64(n)
		}
		if err := d.Close(); err != nil {
			t.Fatal(err)
		}
		got, err := d.AcceptedBytes()
		file, fileErr := os.ReadFile(fixture.Directory + "/first")
		if err != nil || fileErr != nil || got.Uint64() != accepted || accepted != uint64(len(body)) || !bytes.Equal(file, body) {
			t.Fatalf("acceptance=%v/%v,Go counts=%d,file=%d/%v,want exact payload%d", got, err, accepted, len(file), fileErr, len(body))
		}
	})
}
