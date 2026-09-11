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
		switch name {
		case "sha":
			if d.state.seenSHA {
				return 0, core.ErrGitHubResponse
			}
			d.state.seenSHA = true
			if err := d.ignoreString(true); err != nil {
				return 0, err
			}
		case "url":
			if d.state.seenURL {
				return 0, core.ErrGitHubResponse
			}
			d.state.seenURL = true
			if err := d.ignoreString(true); err != nil {
				return 0, err
			}
		case "truncated":
			if d.state.seenTruncated {
				return 0, core.ErrGitHubResponse
			}
			d.state.seenTruncated = true
			value, valueErr := d.next()
			if valueErr != nil {
				return 0, jsonStreamError(valueErr)
			}
			switch value {
			case 't':
				d.state.truncated = true
				if err := d.literal("rue"); err != nil {
					return 0, err
				}
			case 'f':
				if err := d.literal("alse"); err != nil {
					return 0, err
				}
			default:
				return 0, core.ErrGitHubResponse
			}
		case "tree":
			if d.state.seenTree {
				return 0, core.ErrGitHubResponse
			}
			d.state.seenTree = true
			if err := d.entries(); err != nil {
				return 0, err
			}
		default:
			return 0, core.ErrGitHubResponse
		}
	}
	if _, err := d.next(); !errors.Is(err, io.EOF) {
		return 0, jsonStreamError(err)
	}
	if !d.state.seenSHA || !d.state.seenURL || !d.state.seenTree || !d.state.seenTruncated || d.state.truncated {
		return 0, core.ErrGitHubResponse
	}
	return d.state.entries, nil
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
	if !first {
		separator, err := d.next()
		if err != nil {
			return "", false, jsonStreamError(err)
		}
		if separator == '}' {
			return "", true, nil
		}
		if separator != ',' {
			return "", false, core.ErrGitHubResponse
		}
	}
	opening, err := d.next()
	if err != nil {
		return "", false, jsonStreamError(err)
	}
	if first && opening == '}' {
		return "", true, nil
	}
	if opening != '"' {
		return "", false, core.ErrGitHubResponse
	}
	// Longest recognized member is "truncated". Additional decoded name bytes
	// prove an unknown field; this is the schema, not an input-size ceiling.
	name, err := d.closedString(len("truncated"))
	if err != nil {
		return "", false, err
	}
	if err := d.expect(':'); err != nil {
		return "", false, err
	}
	return name, false, nil
}
func (d *treeDecoder) closedString(maximum int) (string, error) {
	var buffer [len("truncated") + 1]byte
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
		opening, err := d.next()
		if err != nil {
			return jsonStreamError(err)
		}
		if opening == ']' {
			return nil
		}
		if !first {
			if opening != ',' {
				return core.ErrGitHubResponse
			}
			opening, err = d.next()
			if err != nil {
				return jsonStreamError(err)
			}
		}
		first = false
		if opening != '{' || d.state.entries == math.MaxUint64 {
			return core.ErrGitHubResponse
		}
		stream := TreeEntryStream{decoder: d, digest: core.NewDigestWriter()}
		err = d.visitor.VisitGitHubTreeEntry(&stream)
		stream.decoder = nil // the callback borrows this reader only through return
		if err != nil {
			return err
		}
		if !errors.Is(stream.terminal, io.EOF) {
			return core.ErrGitHubResponse
		}
		d.state.entries++
	}
}
