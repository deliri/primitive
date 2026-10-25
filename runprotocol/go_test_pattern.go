package runprotocol

import (
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/deliri/primitive/v2026/core"
)

// GoTestPattern is one go test -run, -bench or -fuzz pattern, split exactly as
// Go's testing package splits it: alternatives at top-level '|', elements at
// top-level '/', where brackets and parentheses nest and '\' escapes the next
// byte. Every element, rewritten as testing rewrites it, compiles; go test
// exits on one that does not. No compiled expression is retained.
type GoTestPattern struct{ value string }

func ParseGoTestPattern(value string) (GoTestPattern, error) {
	if err := walkGoTestPattern(value, func(string, bool) error { return nil }); err != nil {
		return GoTestPattern{}, err
	}
	return GoTestPattern{value: value}, nil
}

func (p GoTestPattern) Validate() error {
	return walkGoTestPattern(p.value, func(string, bool) error { return nil })
}

func (p GoTestPattern) String() string { return p.value }

func (GoTestPattern) runProtocolFact() {}

// SelectsTopLevel reports whether go test selects a top-level test, benchmark
// or fuzz target by this pattern: any alternative's first element matches it.
// Later elements select subtests and never exclude their top-level parent.
func (p GoTestPattern) SelectsTopLevel(name string) (bool, error) {
	selected := false
	err := walkGoTestPattern(p.value, func(element string, first bool) error {
		if !first || selected {
			return nil
		}
		matched, err := regexp.MatchString(element, name)
		selected = matched
		return err
	})
	if err != nil {
		return false, err
	}
	return selected, nil
}

// walkGoTestPattern visits every rewritten element in order, marking the first
// element of each alternative, after confirming it compiles. It follows
// testing.splitRegexp byte for byte; an unmatched ']' is legal there and here.
func walkGoTestPattern(pattern string, visit func(element string, first bool) error) error {
	first := true
	emit := func(element string) error {
		rewritten := rewriteGoTestName(element)
		if _, err := regexp.Compile(rewritten); err != nil {
			return errors.Join(core.ErrGoToolchainContract, err)
		}
		err := visit(rewritten, first)
		first = false
		return err
	}
	brackets, parentheses := 0, 0
	for i := 0; i < len(pattern); {
		switch pattern[i] {
		case '[':
			brackets++
		case ']':
			if brackets--; brackets < 0 {
				brackets = 0
			}
		case '(':
			if brackets == 0 {
				parentheses++
			}
		case ')':
			if brackets == 0 {
				parentheses--
			}
		case '\\':
			i++
		case '/', '|':
			if brackets == 0 && parentheses == 0 {
				if err := emit(pattern[:i]); err != nil {
					return err
				}
				if pattern[i] == '|' {
					first = true
				}
				pattern = pattern[i+1:]
				i = 0
				continue
			}
		}
		i++
	}
	return emit(pattern)
}

// rewriteGoTestName is testing.rewrite: white space becomes '_' and
// non-printable runes their quoted escapes.
func rewriteGoTestName(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch {
		case isGoTestSpace(r):
			_ = b.WriteByte('_') // strings.Builder writes always return a nil error.
		case !strconv.IsPrint(r):
			quoted := strconv.QuoteRune(r)
			_, _ = b.WriteString(quoted[1 : len(quoted)-1])
		default:
			_, _ = b.WriteRune(r)
		}
	}
	return b.String()
}

// isGoTestSpace is testing.isSpace, which differs from Unicode's Z class.
func isGoTestSpace(r rune) bool {
	if r < 0x2000 {
		switch r {
		case '\t', '\n', '\v', '\f', '\r', ' ', 0x85, 0xA0, 0x1680:
			return true
		}
		return false
	}
	if r <= 0x200a {
		return true
	}
	switch r {
	case 0x2028, 0x2029, 0x202f, 0x205f, 0x3000:
		return true
	}
	return false
}
