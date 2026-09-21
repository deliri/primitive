package github

import (
	"bufio"
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"io"
	"math"
)

// treeDecoder recognizes only the fixed GitHub tree grammar. It holds no
// collection, nesting stack, growing token buffer, or complete path.
type treeDecoder struct {
	source  *bufio.Reader
	visitor TreeVisitor
	state   treeDecodeState
}

func decodeTree(source io.Reader, visitor TreeVisitor) (uint64, error) {
	d := treeDecoder{source: bufio.NewReader(source), visitor: visitor}
	if err := d.expect('{'); err != nil {
		return 0, err
	}
	first := true
	for {
		name, end, err := d.member(first)
		first = false
		if err != nil {
			return 0, err
		}
		if end {
			break
		}
		if err := d.readRootMember(name); err != nil {
			return 0, err
		}

	}
	if _, err := d.next(); !errors.Is(err, io.EOF) {
		return 0, jsonStreamError(err)
	}
	if err := d.complete(); err != nil {
		return 0, err
	}
	return d.state.entries, nil
}

func (d *treeDecoder) complete() error {
	if !d.state.seenSHA || !d.state.seenURL || !d.state.seenTree || !d.state.seenTruncated || d.state.truncated {
		return core.ErrGitHubResponse
	}
	return nil
}

func (d *treeDecoder) readRootMember(name string) error {
	switch name {
	case "sha":
		return d.readIdentityMember(&d.state.seenSHA)
	case "url":
		return d.readIdentityMember(&d.state.seenURL)
	case treeTruncatedMember:
		return d.readTruncated()
	case "tree":
		if d.state.seenTree {
			return core.ErrGitHubResponse
		}
		d.state.seenTree = true
		return d.entries()
	default:
		return core.ErrGitHubResponse
	}
}

func (d *treeDecoder) readIdentityMember(seen *bool) error {
	if *seen {
		return core.ErrGitHubResponse
	}
	*seen = true
	return d.ignoreString(true)
}

func (d *treeDecoder) readTruncated() error {
	if d.state.seenTruncated {
		return core.ErrGitHubResponse
	}
	d.state.seenTruncated = true
	value, err := d.next()
	if err != nil {
		return jsonStreamError(err)
	}
	switch value {
	case 't':
		d.state.truncated = true
		return d.literal("rue")
	case 'f':
		return d.literal("alse")
	default:
		return core.ErrGitHubResponse
	}
}

func (d *treeDecoder) next() (byte, error) {
	for {
		value, err := d.source.ReadByte()
		if err != nil {
			return 0, err
		}
		switch value {
		case ' ', '\n', '\r', '\t':
			continue
		}
		return value, nil
	}
}
func (d *treeDecoder) expect(want byte) error {
	got, err := d.next()
	if err != nil || got != want {
		return jsonStreamError(err)
	}
	return nil
}
func (d *treeDecoder) literal(want string) error {
	for i := range len(want) {
		got, err := d.source.ReadByte()
		if err != nil || got != want[i] {
			return jsonStreamError(err)
		}
	}
	return nil
}
func (d *treeDecoder) member(first bool) (string, bool, error) {
	opening, err := d.memberOpening(first)
	if err != nil {
		return "", false, err
	}
	if opening == '}' {
		return "", true, nil
	}
	if opening != '"' {
		return "", false, core.ErrGitHubResponse
	}
	// Longest recognized member is treeTruncatedMember. Additional decoded name bytes
	// prove an unknown field; this is the schema, not an input-size ceiling.
	name, err := d.closedString(len(treeTruncatedMember))
	if err != nil {
		return "", false, err
	}
	if err := d.expect(':'); err != nil {
		return "", false, err
	}
	return name, false, nil
}
func (d *treeDecoder) memberOpening(first bool) (byte, error) {
	if !first {
		separator, err := d.next()
		if err != nil {
			return 0, jsonStreamError(err)
		}
		if separator == '}' {
			return '}', nil
		}
		if separator != ',' {
			return 0, core.ErrGitHubResponse
		}
	}
	opening, err := d.next()
	if err != nil {
		return 0, jsonStreamError(err)
	}
	if !first && opening == '}' {
		return 0, core.ErrGitHubResponse
	}
	return opening, nil
}
func (d *treeDecoder) closedString(maximum int) (string, error) {
	var buffer [len(treeTruncatedMember) + 1]byte
	stream := jsonStringStream{source: d.source}
	n, err := io.ReadFull(&stream, buffer[:maximum+1])
	if !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return "", jsonStreamError(err)
	}
	return string(buffer[:n]), nil
}
func (d *treeDecoder) ignoreString(nonempty bool) error {
	opening, err := d.next()
	if err != nil {
		return jsonStreamError(err)
	}
	if opening == 'n' && !nonempty {
		return d.literal("ull")
	}
	if opening != '"' {
		return core.ErrGitHubResponse
	}
	stream := jsonStringStream{source: d.source}
	return consumeIgnoredString(&stream, nonempty)
}

func consumeIgnoredString(stream *jsonStringStream, nonempty bool) error {
	var scratch [4096]byte
	seen := false
	for {
		n, readErr := stream.Read(scratch[:])
		seen = seen || n != 0
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			return readErr
		}
	}
	if nonempty && !seen {
		return core.ErrGitHubResponse
	}
	return nil
}
func (d *treeDecoder) ignoreUint64() error {
	first, err := d.next()
	if err != nil {
		return jsonStreamError(err)
	}
	if first == 'n' {
		return d.literal("ull")
	}
	if first < '0' || first > '9' {
		return core.ErrGitHubResponse
	}
	return d.consumeUint64(first)
}

func (d *treeDecoder) consumeUint64(first byte) error {
	value := uint64(first - '0')
	for {
		next, readErr := d.source.ReadByte()
		if readErr != nil {
			return jsonStreamError(readErr)
		}
		if next < '0' || next > '9' {
			if err := d.source.UnreadByte(); err != nil {
				return jsonStreamError(err)
			}
			return nil
		}
		if first == '0' || value > (math.MaxUint64-uint64(next-'0'))/10 {
			return core.ErrGitHubResponse
		}
		value = value*10 + uint64(next-'0')
	}
}
func (d *treeDecoder) entries() error {
	if err := d.expect('['); err != nil {
		return err
	}
	first := true
	for {
		opening, err := d.entryOpening(first)
		if err != nil {
			return err
		}
		if opening == ']' {
			return nil
		}

		first = false
		if opening != '{' || d.state.entries == math.MaxUint64 {
			return core.ErrGitHubResponse
		}
		if err := d.visitEntry(); err != nil {
			return err
		}

	}
}

func (d *treeDecoder) entryOpening(first bool) (byte, error) {
	opening, err := d.next()
	if err != nil {
		return 0, jsonStreamError(err)
	}
	if opening == ']' || first {
		return opening, nil
	}
	if opening != ',' {
		return 0, core.ErrGitHubResponse
	}
	opening, err = d.next()
	if err != nil {
		return 0, jsonStreamError(err)
	}
	// A close after a comma is not the end of an array; it is a trailing comma.
	if opening == ']' {
		return 0, core.ErrGitHubResponse
	}
	return opening, nil
}

func (d *treeDecoder) visitEntry() error {
	stream := TreeEntryStream{decoder: d, digest: core.NewDigestWriter()}
	err := d.visitor.VisitGitHubTreeEntry(&stream)
	stream.decoder = nil // the callback borrows this reader only through return
	if err != nil {
		return err
	}
	if !errors.Is(stream.terminal, io.EOF) {
		return core.ErrGitHubResponse
	}
	d.state.entries++
	return nil
}

const treeTruncatedMember = "truncated"
