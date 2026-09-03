package github

import (
	"bytes"
	"encoding/base64"
	json "encoding/json/v2"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

type responsePressureClass uint8

const (
	responsePressureUnknown responsePressureClass = iota
	responsePressureAccepted
	responsePressureRejected
	responsePressureBoundary
	responsePressureLimit
)

type responsePressureCase struct {
	run      func() (uint64, error)
	wantErr  error
	name     string
	wantFact uint64
	class    responsePressureClass
}

type treeResponseFixture struct {
	SHA       string          `json:"sha"`
	URL       string          `json:"url"`
	Tree      []treeEntryWire `json:"tree"`
	Truncated bool            `json:"truncated"`
}

func TestGitHubResponseDecodersHostileBoundaryTable(t *testing.T) {
	t.Parallel()

	commit := parsedCommit(t).String()
	request := FileRequest{
		Repository: parsedRepository(t, "owner/repository"), Commit: parsedCommit(t),
		Path: parsedPath(t, "source/main.go"), MaximumBytes: byteCountFixture(t, 1024),
	}
	now, err := temporal.ParseRFC3339("2026-09-03T10:00:00Z")
	if err != nil {
		t.Fatalf("temporal.ParseRFC3339(fixed now) error = %v, want nil", err)
	}
	tokenAt := func(minutes uint64) []byte {
		expiresAt, addErr := now.Add(durationMinutesFixture(t, minutes))
		if addErr != nil {
			t.Fatalf("fixed token expiry +%dm error = %v, want nil", minutes, addErr)
		}
		expiresText, textErr := expiresAt.RFC3339()
		if textErr != nil {
			t.Fatalf("fixed token expiry +%dm text error = %v, want nil", minutes, textErr)
		}
		return []byte(fmt.Sprintf("{\"token\":\"ghs_fixed\",\"expires_at\":%q}", expiresText))
	}
	tagsAtLimit := make([]tagWire, core.GitHubTagPageMaximumEntries)
	for index := range tagsAtLimit {
		tagsAtLimit[index] = tagWire{Name: "tag-" + strconv.Itoa(index), Commit: tagCommitWire{SHA: commit}}
	}
	tagsOverLimit := append(cloneTagWires(tagsAtLimit), tagWire{Name: "over-limit", Commit: tagCommitWire{SHA: commit}})

	cases := []responsePressureCase{
		{name: "accepted empty tag page preserves zero facts", class: responsePressureAccepted, run: tagDecoder(marshalGitHubFixture(t, []tagWire{}))},
		{name: "accepted one tag preserves its commit", class: responsePressureAccepted, wantFact: 1, run: tagDecoder(marshalGitHubFixture(t, []tagWire{{Name: "v1", Commit: tagCommitWire{SHA: commit}}}))},
		{name: "accepted two distinct tags preserve cardinality", class: responsePressureAccepted, wantFact: 2, run: tagDecoder(marshalGitHubFixture(t, []tagWire{{Name: "v1", Commit: tagCommitWire{SHA: commit}}, {Name: "v2", Commit: tagCommitWire{SHA: commit}}}))},
		{name: "accepted head preserves one exact commit", class: responsePressureAccepted, wantFact: 1, run: headDecoder(marshalGitHubFixture(t, []headWire{{SHA: commit}}))},
		{name: "accepted empty file preserves zero bytes", class: responsePressureAccepted, run: fileDecoder(request, filePayload(t, request.Path.String(), nil, 0, "base64"))},
		{name: "accepted one-byte file preserves one byte", class: responsePressureAccepted, wantFact: 1, run: fileDecoder(request, filePayload(t, request.Path.String(), []byte("x"), 1, "base64"))},
		{name: "accepted installation access remains usable", class: responsePressureAccepted, wantFact: 1, run: tokenDecoder(tokenAt(60), now)},
		{name: "accepted empty tree preserves zero visits", class: responsePressureAccepted, run: treeDecoderRun(treePayload(t, nil, false), 1)},
		{name: "accepted blob tree preserves one visit", class: responsePressureAccepted, wantFact: 1, run: treeDecoderRun(treePayload(t, []treeEntryWire{treeWire("main.go", "blob", commit)}, false), 1)},
		{name: "accepted mixed tree preserves all closed kinds", class: responsePressureAccepted, wantFact: 3, run: treeDecoderRun(treePayload(t, []treeEntryWire{treeWire("main.go", "blob", commit), treeWire("internal", "tree", commit), treeWire("module", "commit", commit)}, false), 3)},

		{name: "rejected tag unknown member cannot escape", class: responsePressureRejected, wantErr: core.ErrGitHubResponse, run: tagDecoder([]byte("[{\"name\":\"v1\",\"commit\":{\"sha\":\"0123456789abcdef0123456789abcdef01234567\",\"url\":\"\"},\"zipball_url\":\"\",\"tarball_url\":\"\",\"node_id\":\"\",\"future\":true}]"))},
		{name: "rejected tag missing commit SHA cannot become fact", class: responsePressureRejected, wantErr: core.ErrGitHubResponse, run: tagDecoder(marshalGitHubFixture(t, []tagWire{{Name: "v1"}}))},
		{name: "rejected empty head list cannot invent commit", class: responsePressureRejected, wantErr: core.ErrGitHubResponse, run: headDecoder([]byte("[]"))},
		{name: "rejected multiple heads cannot select by order", class: responsePressureRejected, wantErr: core.ErrGitHubResponse, run: headDecoder(marshalGitHubFixture(t, []headWire{{SHA: commit}, {SHA: commit}}))},
		{name: "rejected foreign file path cannot rebind request", class: responsePressureRejected, wantErr: core.ErrGitHubResponse, run: fileDecoder(request, filePayload(t, "other.go", []byte("x"), 1, "base64"))},
		{name: "rejected file encoding cannot be guessed", class: responsePressureRejected, wantErr: core.ErrGitHubResponse, run: fileDecoder(request, filePayload(t, request.Path.String(), []byte("x"), 1, "none"))},
		{name: "rejected bearer control cannot become credential", class: responsePressureRejected, wantErr: core.ErrGitHubResponse, run: tokenDecoder([]byte("{\"token\":\"bad token\",\"expires_at\":\"2026-09-03T11:00:00Z\"}"), now)},
		{name: "rejected malformed expiry cannot become credential", class: responsePressureRejected, wantErr: core.ErrGitHubResponse, run: tokenDecoder([]byte("{\"token\":\"ghs_fixed\",\"expires_at\":\"tomorrow\"}"), now)},
		{name: "rejected tree root unknown member cannot escape", class: responsePressureRejected, wantErr: core.ErrGitHubResponse, run: treeDecoderRun([]byte("{\"sha\":\"x\",\"url\":\"x\",\"tree\":[],\"truncated\":false,\"future\":true}"), 1)},
		{name: "rejected tree entry unknown kind cannot become fact", class: responsePressureRejected, wantErr: core.ErrGitHubResponse, run: treeDecoderRun(treePayload(t, []treeEntryWire{treeWire("main.go", "future", commit)}, false), 1)},

		{name: "boundary tag page exact provider cardinality is admitted", class: responsePressureBoundary, wantFact: core.GitHubTagPageMaximumEntries, run: tagDecoder(marshalGitHubFixture(t, tagsAtLimit))},
		{name: "boundary tag page one above provider cardinality is refused", class: responsePressureBoundary, wantErr: core.ErrGitHubResponse, run: tagDecoder(marshalGitHubFixture(t, tagsOverLimit))},
		{name: "boundary duplicate tag member is refused by strict JSON", class: responsePressureBoundary, wantErr: core.ErrGitHubResponse, run: tagDecoder([]byte("[{\"name\":\"v1\",\"name\":\"v2\",\"commit\":{\"sha\":\"0123456789abcdef0123456789abcdef01234567\",\"url\":\"\"},\"zipball_url\":\"\",\"tarball_url\":\"\",\"node_id\":\"\"}]"))},
		{name: "boundary truncated tag document is refused", class: responsePressureBoundary, wantErr: core.ErrGitHubResponse, run: tagDecoder([]byte("[{\"name\":\"v1\""))},
		{name: "boundary trailing tag document is refused", class: responsePressureBoundary, wantErr: core.ErrGitHubResponse, run: tagDecoder([]byte("[] []"))},
		{name: "boundary SHA-256 head is admitted", class: responsePressureBoundary, wantFact: 1, run: headDecoder(marshalGitHubFixture(t, []headWire{{SHA: strings.Repeat("a", 64)}}))},
		{name: "boundary head one below SHA-1 width is refused", class: responsePressureBoundary, wantErr: core.ErrGitHubResponse, run: headDecoder(marshalGitHubFixture(t, []headWire{{SHA: strings.Repeat("a", 39)}}))},
		{name: "boundary head one above SHA-1 width is refused", class: responsePressureBoundary, wantErr: core.ErrGitHubResponse, run: headDecoder(marshalGitHubFixture(t, []headWire{{SHA: strings.Repeat("a", 41)}}))},
		{name: "boundary file exact caller byte ceiling is admitted", class: responsePressureBoundary, wantFact: 1024, run: fileDecoder(request, filePayload(t, request.Path.String(), bytes.Repeat([]byte("x"), 1024), 1024, "base64"))},
		{name: "boundary file one above caller byte ceiling is refused", class: responsePressureBoundary, wantErr: core.ErrGitHubResponse, run: fileDecoder(request, filePayload(t, request.Path.String(), bytes.Repeat([]byte("x"), 1025), 1025, "base64"))},
		{name: "boundary file claimed length below bytes is refused", class: responsePressureBoundary, wantErr: core.ErrGitHubResponse, run: fileDecoder(request, filePayload(t, request.Path.String(), []byte("xy"), 1, "base64"))},
		{name: "boundary file malformed base64 is refused", class: responsePressureBoundary, wantErr: core.ErrGitHubResponse, run: fileDecoder(request, []byte(fmt.Sprintf("{\"name\":\"\",\"path\":%q,\"sha\":\"\",\"size\":1,\"url\":\"\",\"html_url\":\"\",\"git_url\":\"\",\"download_url\":\"\",\"type\":\"file\",\"content\":\"!\",\"encoding\":\"base64\",\"_links\":{\"self\":\"\",\"git\":\"\",\"html\":\"\"}}", request.Path.String())))},
		{name: "boundary duplicate file path is refused by strict JSON", class: responsePressureBoundary, wantErr: core.ErrGitHubResponse, run: fileDecoder(request, []byte(fmt.Sprintf("{\"name\":\"\",\"path\":%q,\"path\":%q,\"sha\":\"\",\"size\":0,\"url\":\"\",\"html_url\":\"\",\"git_url\":\"\",\"download_url\":\"\",\"type\":\"file\",\"content\":\"\",\"encoding\":\"base64\",\"_links\":{\"self\":\"\",\"git\":\"\",\"html\":\"\"}}", request.Path.String(), request.Path.String())))},
		{name: "boundary token exact refresh threshold is refused", class: responsePressureBoundary, wantErr: core.ErrGitHubResponse, run: tokenDecoder(tokenAt(1), now)},
		{name: "boundary token one minute above refresh threshold is admitted", class: responsePressureBoundary, wantFact: 1, run: tokenDecoder(tokenAt(2), now)},
		{name: "boundary expired token is refused", class: responsePressureBoundary, wantErr: core.ErrGitHubResponse, run: tokenDecoder([]byte("{\"token\":\"ghs_fixed\",\"expires_at\":\"2026-09-03T09:59:59Z\"}"), now)},
		{name: "boundary tree exact caller cardinality is admitted", class: responsePressureBoundary, wantFact: 1, run: treeDecoderRun(treePayload(t, []treeEntryWire{treeWire("main.go", "blob", commit)}, false), 1)},
		{name: "boundary tree one above caller cardinality is refused", class: responsePressureBoundary, wantErr: core.ErrGitHubResponse, run: treeDecoderRun(treePayload(t, []treeEntryWire{treeWire("a.go", "blob", commit), treeWire("b.go", "blob", commit)}, false), 1)},
		{name: "boundary duplicate tree root member is refused", class: responsePressureBoundary, wantErr: core.ErrGitHubResponse, run: treeDecoderRun([]byte("{\"sha\":\"x\",\"url\":\"x\",\"tree\":[],\"tree\":[],\"truncated\":false}"), 1)},
		{name: "boundary provider-truncated tree cannot claim completion", class: responsePressureBoundary, wantErr: core.ErrGitHubResponse, run: treeDecoderRun(treePayload(t, nil, true), 1)},
	}

	wantClasses := [responsePressureLimit]uint64{
		responsePressureAccepted: 10,
		responsePressureRejected: 10,
		responsePressureBoundary: 20,
	}
	gotClasses := [responsePressureLimit]uint64{}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()

			gotFact, gotErr := testCase.run()
			if !errors.Is(gotErr, testCase.wantErr) {
				t.Fatalf("GitHub response decode error = %v, want %v", gotErr, testCase.wantErr)
			}
			if gotFact != testCase.wantFact {
				t.Fatalf("GitHub response decoded fact = %d, want %d", gotFact, testCase.wantFact)
			}
		})
		gotClasses[testCase.class]++
	}
	if gotClasses != wantClasses {
		t.Fatalf("GitHub hostile response classes = %v, want %v", gotClasses, wantClasses)
	}
}

func tagDecoder(payload []byte) func() (uint64, error) {
	return func() (uint64, error) {
		got, err := decodeTagPage(payload)
		return uint64(len(got)), err
	}
}

func headDecoder(payload []byte) func() (uint64, error) {
	return func() (uint64, error) {
		got, err := decodeHead(payload)
		if err != nil {
			return 0, err
		}
		return 1, got.Validate()
	}
}

func fileDecoder(request FileRequest, payload []byte) func() (uint64, error) {
	return func() (uint64, error) {
		got, err := fileObservation(request, payload)
		return uint64(len(got.Content)), err
	}
}

func tokenDecoder(payload []byte, now temporal.Instant) func() (uint64, error) {
	return func() (uint64, error) {
		_, err := decodeInstallationToken(payload, now)
		if err != nil {
			return 0, err
		}
		return 1, nil
	}
}

func treeDecoderRun(payload []byte, maximum uint64) func() (uint64, error) {
	return func() (uint64, error) {
		visitor := &collectingTreeVisitor{}
		return decodeTree(bytes.NewReader(payload), maximum, visitor)
	}
}

func filePayload(t testing.TB, path string, content []byte, size uint64, encoding string) []byte {
	t.Helper()
	return marshalGitHubFixture(t, contentsWire{
		Path: path, Size: size, Type: "file", Encoding: encoding,
		Content: base64.StdEncoding.EncodeToString(content),
	})
}

func treePayload(t testing.TB, entries []treeEntryWire, truncated bool) []byte {
	t.Helper()
	return marshalGitHubFixture(t, treeResponseFixture{
		SHA: parsedCommit(t).String(), URL: "https://api.github.com/tree",
		Tree: entries, Truncated: truncated,
	})
}

func treeWire(path, kind, sha string) treeEntryWire {
	return treeEntryWire{Path: path, Mode: "100644", Type: kind, SHA: sha, URL: "https://api.github.com/object"}
}

func marshalGitHubFixture[Value any](t testing.TB, value Value) []byte {
	t.Helper()
	payload, err := json.Marshal(value)
	if err != nil {
		t.Fatalf("json.Marshal(GitHub fixture) error = %v, want nil", err)
	}
	return payload
}

func cloneTagWires(value []tagWire) []tagWire {
	return append([]tagWire(nil), value...)
}
