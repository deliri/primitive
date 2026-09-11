package github

import (
	"errors"
	"github.com/deliri/primitive/v2026/core"
	"io"
	"unicode"
)

// TreeEntryStream lends decoded path bytes to one synchronous visitor.
// Read through EOF, then call Observation. Neither the complete path nor the
// tree is retained. The reader must not escape the visitor's lifetime.
type TreeEntryStream struct {
	decoder             *treeDecoder
	digest              *core.DigestWriter
	path                jsonStringStream
	pathFacts           treePathFacts
	observation         TreeEntry
	terminal            error
	fields              uint8
	started, pathActive bool
}

// Observation returns typed metadata only after the entire entry reached EOF.
// Earlier calls and failed entries return a zero observation and typed refusal.
func (s *TreeEntryStream) Observation() (TreeEntry, error) {
	if s == nil || !errors.Is(s.terminal, io.EOF) {
		return TreeEntry{}, core.ErrGitHubResponse
	}
	return s.observation, s.observation.Validate()
}
func (s *TreeEntryStream) Read(p []byte) (int, error) {
	if s == nil {
		return 0, core.ErrGitHubContract
	}
	if len(p) == 0 {
		return 0, nil
	}
	if s.terminal != nil {
		return 0, s.terminal
	}
	if s.decoder == nil || s.digest == nil {
		return 0, core.ErrGitHubContract
	}
	if s.pathActive {
		n, err := s.readPath(p)
		if n != 0 || err != nil || s.pathActive {
			return n, err
		}
	}
	for {
		name, end, memberErr := s.decoder.member(!s.started)
		s.started = true
		if memberErr != nil {
			s.terminal = memberErr
			return 0, memberErr
		}
		if end {
			return 0, s.finishEntry()
		}
		if memberErr = s.readMember(name); memberErr != nil {
			s.terminal = memberErr
			return 0, memberErr
		}
		if s.pathActive {
			return s.Read(p)
		}
	}
}
func (s *TreeEntryStream) readPath(p []byte) (int, error) {
	n, readErr := s.path.Read(p)
	if n > 0 {
		wrote, hashErr := s.digest.Write(p[:n])
		if hashErr != nil || wrote != n {
			s.terminal = jsonStreamError(hashErr)
			return n, s.terminal
		}
	}
	if readErr != nil && !errors.Is(readErr, io.EOF) {
		s.terminal = readErr
		return n, readErr
	}
	if errors.Is(readErr, io.EOF) {
		s.pathActive = false
		if pathErr := s.pathFacts.finish(); pathErr != nil {
			s.terminal = pathErr
			return n, pathErr
		}
	}
	return n, nil
}
func (s *TreeEntryStream) finishEntry() error {
	if s.fields&3 != 3 {
		s.terminal = core.ErrGitHubResponse
		return s.terminal
	}
	digest, length, sealErr := s.digest.Seal()
	if sealErr != nil {
		s.terminal = jsonStreamError(sealErr)
		return s.terminal
	}
	s.observation.PathSHA256 = digest
	s.observation.PathLength = length
	if validationErr := s.observation.Validate(); validationErr != nil {
		s.terminal = validationErr
		return validationErr
	}
	s.terminal = io.EOF
	return io.EOF
}
func (s *TreeEntryStream) readMember(name string) error {
	var bit uint8
	switch name {
	case "path":
		bit = 1
	case "type":
		bit = 2
	case "mode":
		bit = 4
	case "sha":
		bit = 8
	case "url":
		bit = 16
	case "size":
		bit = 32
	default:
		return core.ErrGitHubResponse
	}
	if s.fields&bit != 0 {
		return core.ErrGitHubResponse
	}
	s.fields |= bit
	switch name {
	case "path":
		if openingErr := s.decoder.expect('"'); openingErr != nil {
			return openingErr
		}
		s.path = jsonStringStream{source: s.decoder.source, observe: s.pathFacts.admit}
		s.pathActive = true
		return nil
	case "type":
		return s.readKind()
	case "size":
		return s.decoder.ignoreUint64()
	default:
		return s.decoder.ignoreString(false)
	}
}
func (s *TreeEntryStream) readKind() error {
	if openingErr := s.decoder.expect('"'); openingErr != nil {
		return openingErr
	}
	value, decodeErr := s.decoder.closedString(len("commit"))
	if decodeErr != nil {
		return decodeErr
	}
	switch value {
	case "blob":
		s.observation.Kind = TreeEntryBlob
	case "tree":
		s.observation.Kind = TreeEntryDirectory
	case "commit":
		s.observation.Kind = TreeEntrySubmodule
	default:
		return core.ErrGitHubResponse
	}
	return nil
}

// treePathFacts recognizes SourcePath's slash-canonical text contract without
// retaining a segment. Segment lengths saturate at three because only "." and
// ".." need length-sensitive decisions; all longer segments share a rule.
type treePathFacts struct {
	segment                    uint8
	onlyDots                   bool
	seen, slash, trailingSpace bool
}

func (p *treePathFacts) admit(value rune) error {
	if !p.seen && unicode.IsSpace(value) {
		return core.ErrGitHubResponse
	}
	p.seen = true
	p.trailingSpace = unicode.IsSpace(value)
	switch value {
	case '\\', 0, '\r', '\n':
		return core.ErrGitHubResponse
	case '/':
		if p.segment == 0 || p.onlyDots && p.segment <= 2 {
			return core.ErrGitHubResponse
		}
		p.segment = 0
		p.onlyDots = false
		p.slash = true
		return nil
	}
	if p.segment == 0 {
		p.onlyDots = value == '.'
	} else {
		p.onlyDots = p.onlyDots && value == '.'
	}
	if p.segment < 3 {
		p.segment++
	}
	return nil
}
func (p *treePathFacts) finish() error {
	if !p.seen || p.trailingSpace || p.segment == 0 || p.onlyDots && (p.segment == 2 || p.segment == 1 && p.slash) {
		return core.ErrGitHubResponse
	}
	return nil
}

var _ io.Reader = (*TreeEntryStream)(nil)
