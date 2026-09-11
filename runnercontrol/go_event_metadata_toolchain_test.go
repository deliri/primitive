package runnercontrol_test

import (
	"bytes"
	"context"
	json "encoding/json/v2"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/deliri/primitive/v2026/runnercontrol"
)

type goEventMetadataFixture struct {
	Action runnercontrol.GoEventAction `json:"Action"`
	Key    *string                     `json:"Key,omitzero"`
	Value  *string                     `json:"Value,omitzero"`
	Path   *string                     `json:"Path,omitzero"`
}

func FuzzGoEventStreamMetadataContent(f *testing.F) {
	seedText := "é界😀\n\t\"\\"
	seed := goEventMetadataFixture{Action: runnercontrol.GoEventActionAttribute, Key: &seedText, Value: &seedText}
	if err := seed.Action.Validate(); err != nil {
		f.Fatal(err)
	}
	canonical, err := json.Marshal(seed)
	if err != nil {
		f.Fatal(err)
	}
	f.Add(canonical)
	f.Add([]byte{})
	f.Add([]byte{0xff, 0x00, 0xc0})
	f.Fuzz(func(t *testing.T, data []byte) {
		// Bound secondary oracle storage, while consuming every fuzz input byte.
		for offset := 0; ; {
			end := min(offset+goEventOracleChunk, len(data))
			want := strings.ToValidUTF8(string(data[offset:end]), "�")
			for _, fixture := range []goEventMetadataFixture{
				{Action: runnercontrol.GoEventActionAttribute, Key: &want, Value: &want},
				{Action: runnercontrol.GoEventActionArtifacts, Path: &want},
			} {
				encoded, err := json.Marshal(fixture)
				if err != nil {
					t.Fatalf("Marshal(metadata) error = %v, want nil", err)
				}
				var positions, closed [3]int
				frames := 0
				err = runnercontrol.ReadGoEventStream(t.Context(), runnercontrol.GoEventStreamRequest{
					Source: bytes.NewReader(encoded),
					OnString: func(fragment runnercontrol.GoEventStringFragment) error {
						if err := fragment.Validate(); err != nil {
							return err
						}
						var index int
						switch fragment.Field {
						case runnercontrol.GoEventFieldAction:
							return nil
						case runnercontrol.GoEventFieldKey:
							index = 0
						case runnercontrol.GoEventFieldValue:
							index = 1
						case runnercontrol.GoEventFieldPath:
							index = 2
						default:
							t.Fatalf("metadata field = %v, want action/key/value/path", fragment.Field)
						}
						next := positions[index] + len(fragment.Data)
						if next > len(want) || string(fragment.Data) != want[positions[index]:next] {
							t.Fatalf("metadata fragment %v at %d = %q, want exact encoded value", fragment.Field, positions[index], fragment.Data)
						}
						positions[index] = next
						if fragment.Final {
							closed[index]++
						}
						return nil
					},
					OnEvent: func(frame runnercontrol.GoEventFrame) error {
						frames++
						if frame.Action != fixture.Action || frame.OutputKind != runnercontrol.GoEventOutputOrdinary {
							t.Fatalf("metadata frame = %+v, want action %v and ordinary output kind", frame, fixture.Action)
						}
						return frame.Validate()
					},
				})
				for index, text := range []*string{fixture.Key, fixture.Value, fixture.Path} {
					wantBytes, wantCloses := 0, 0
					if text != nil {
						wantBytes, wantCloses = len(*text), 1
					}
					if positions[index] != wantBytes || closed[index] != wantCloses {
						t.Fatalf("metadata field %d = (%d bytes,%d closes), want (%d,%d)", index, positions[index], closed[index], wantBytes, wantCloses)
					}
				}
				if err != nil || frames != 1 {
					t.Fatalf("ReadGoEventStream(metadata) = (%v,%d frames), want (nil,1)", err, frames)
				}
			}
			if end == len(data) {
				break
			}
			offset = end
		}
	})
}

// Drives the installed Go producer; handwritten JSON cannot prove that its
// current event vocabulary is accepted by the shared decoder.
func TestGoEventStreamAcceptsNativeMetadata(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name       string
		body       string
		wantAction runnercontrol.GoEventAction
	}{
		{name: "attribute remains an event", body: `t.Attr("purpose", "decoder evidence")`, wantAction: runnercontrol.GoEventActionAttribute},
		{name: "artifact directory remains an event", body: `_ = t.ArtifactDir()`, wantAction: runnercontrol.GoEventActionArtifacts},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			for _, file := range []struct{ name, content string }{
				{name: "go.mod", content: "module metadatafixture\n\ngo 1.27.1\n"},
				{name: "metadata_test.go", content: "package metadatafixture\nimport \"testing\"\nfunc TestMetadata(t *testing.T) { t.Parallel(); " + tc.body + " }\n"},
			} {
				if err := os.WriteFile(filepath.Join(dir, file.name), []byte(file.content), 0600); err != nil {
					t.Fatalf("WriteFile(%s) error = %v, want nil", file.name, err)
				}
			}
			output, err := os.Create(filepath.Join(dir, "events.jsonl"))
			if err != nil {
				t.Fatalf("Create(events) error = %v, want nil", err)
			}
			t.Cleanup(func() {
				if err := output.Close(); err != nil {
					t.Errorf("Close(events) error = %v, want nil", err)
				}
			})
			ctx, cancel := context.WithTimeout(t.Context(), time.Minute)
			defer cancel()
			command := exec.CommandContext(ctx, "go", "test", "-json", "-count=1", "-artifacts", "-outputdir", dir, ".")
			command.Dir, command.Stdout, command.Stderr = dir, output, output
			if err := command.Run(); err != nil {
				t.Fatalf("go test metadata fixture error = %v, want nil", err)
			}
			if _, err := output.Seek(0, 0); err != nil {
				t.Fatalf("Seek(events) error = %v, want nil", err)
			}
			frames, metadata := 0, 0
			var key, value, path strings.Builder
			err = runnercontrol.ReadGoEventStream(ctx, runnercontrol.GoEventStreamRequest{
				Source: output,
				OnString: func(fragment runnercontrol.GoEventStringFragment) error {
					if err := fragment.Validate(); err != nil {
						return err
					}
					switch fragment.Field {
					case runnercontrol.GoEventFieldKey:
						key.Write(fragment.Data)
					case runnercontrol.GoEventFieldValue:
						value.Write(fragment.Data)
					case runnercontrol.GoEventFieldPath:
						path.Write(fragment.Data)
					}
					return nil
				},
				OnEvent: func(frame runnercontrol.GoEventFrame) error {
					frames++
					if frame.Action == tc.wantAction {
						metadata++
						switch frame.Action {
						case runnercontrol.GoEventActionAttribute:
							if key.String() != "purpose" || value.String() != "decoder evidence" || path.Len() != 0 {
								t.Fatalf("attribute = (%q, %q, %q), want (purpose, decoder evidence, empty)", key.String(), value.String(), path.String())
							}
						case runnercontrol.GoEventActionArtifacts:
							rel, err := filepath.Rel(dir, path.String())
							info, statErr := os.Stat(path.String())
							if err != nil || !filepath.IsLocal(rel) || statErr != nil || !info.IsDir() || key.Len() != 0 || value.Len() != 0 {
								t.Fatalf("artifact path = %q, relative %q/%v, stat %v/%v, attribute %q/%q; want owned directory and no attribute", path.String(), rel, err, info, statErr, key.String(), value.String())
							}
						default:
							t.Fatalf("metadata action = %v, want attribute or artifacts", frame.Action)
						}
					}
					key.Reset()
					value.Reset()
					path.Reset()
					return frame.Validate()
				},
			})
			if err != nil || frames == 0 || metadata != 1 {
				t.Fatalf("ReadGoEventStream(native metadata) = (%v, %d frames, %d metadata), want (nil, nonempty, 1)", err, frames, metadata)
			}
		})
	}
}
