package runnercontrol

import (
	"encoding/xml"
	"errors"
	"io"
	"math"

	"github.com/deliri/primitive/v2026/core"
)

// JUnitSummary contains only outcomes observed in closed testcase elements.
// It does not assert a planned count, completeness against a plan, or NotRun.
type JUnitSummary struct {
	Passed  uint64
	Failed  uint64
	Skipped uint64
}

// Cases derives the observed count with checked native representability.
func (s JUnitSummary) Cases() (uint64, error) {
	if s.Passed > math.MaxUint64-s.Failed || s.Passed+s.Failed > math.MaxUint64-s.Skipped {
		return 0, observationFailure("JUnit observed counts overflow", core.ErrPrimitiveContract)
	}
	return s.Passed + s.Failed + s.Skipped, nil
}

func (s JUnitSummary) Validate() error { _, err := s.Cases(); return err }

type junitCompileResult struct {
	err     error
	summary JUnitSummary
}

type junitStreamState struct {
	summary     JUnitSummary
	depth       int
	suiteDepth  int
	caseDepth   int
	caseFailed  bool
	caseSkipped bool
	rootSeen    bool
	rootSuites  bool
}

// JUnitSummaryCompiler streams one JUnit document through encoding/xml without
// a planned-unit or total-extent limit. The XML decoder owns its current token
// and nesting stack; arbitrarily large tokens or depth can require memory.
// The caller owns exactly one terminal action: Seal or Abort, both of which
// close the input and join the parser. Methods require a single caller.
type JUnitSummaryCompiler struct {
	failure error
	writer  *io.PipeWriter
	done    <-chan junitCompileResult
	sealed  bool
}

func NewJUnitSummaryCompiler() *JUnitSummaryCompiler {
	reader, writer := io.Pipe()
	done := make(chan junitCompileResult, 1)
	go func() {
		summary, err := parseJUnitStream(reader)
		closeErr := reader.Close()
		done <- junitCompileResult{summary: summary, err: errors.Join(err, closeErr)}
		close(done)
	}()
	return &JUnitSummaryCompiler{writer: writer, done: done}
}

func (c *JUnitSummaryCompiler) Write(data []byte) (int, error) {
	if c == nil || c.writer == nil || c.sealed {
		return 0, observationFailure("JUnit summary compiler is not writable", core.ErrPrimitiveContract)
	}
	if c.failure != nil {
		return 0, c.failure
	}
	written, err := c.writer.Write(data)
	if err == nil && written != len(data) {
		err = io.ErrShortWrite
	}
	if err != nil {
		c.failure = observationFailure("JUnit XML stream cannot be consumed", core.ErrPrimitiveContract, err)
		return written, c.failure
	}
	return written, nil
}

// Seal returns observed counts even when they contradict the native process
// result. Such a contradiction is a typed refusal, not successful evidence.
// Malformed or incomplete XML returns no partial summary. Native causes remain
// reachable through errors.Is/errors.As, including when the report is missing.
func (c *JUnitSummaryCompiler) Seal(executionErr error) (JUnitSummary, error) {
	if c == nil || c.writer == nil || c.sealed {
		return JUnitSummary{}, observationFailure("JUnit summary compiler cannot be sealed", core.ErrPrimitiveContract, executionErr)
	}
	c.sealed = true
	closeErr := c.writer.Close()
	result := <-c.done
	if err := errors.Join(c.failure, closeErr, result.err); err != nil {
		return JUnitSummary{}, observationFailure("JUnit summary is unavailable", core.ErrPrimitiveContract, err, executionErr)
	}
	if (executionErr == nil) != (result.summary.Failed == 0) {
		return result.summary, observationFailure("JUnit terminal counts disagree with the process exit", core.ErrPrimitiveContract, executionErr)
	}
	return result.summary, nil
}

// Abort releases an abandoned parser and is safe to defer before Seal.
func (c *JUnitSummaryCompiler) Abort() {
	if c == nil || c.writer == nil || c.sealed {
		return
	}
	c.sealed = true
	_ = c.writer.CloseWithError(core.ErrPrimitiveContract)
	<-c.done
}

func parseJUnitStream(reader io.Reader) (JUnitSummary, error) {
	decoder := xml.NewDecoder(reader)
	var state junitStreamState
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return JUnitSummary{}, observationFailure("JUnit XML cannot be decoded", core.ErrPrimitiveContract, err)
		}
		var consumeErr error
		switch value := token.(type) {
		case xml.StartElement:
			consumeErr = state.consumeStart(value)
		case xml.EndElement:
			consumeErr = state.consumeEnd(value)
		case xml.CharData:
			consumeErr = state.consumeText(value)
		case xml.Comment, xml.ProcInst, xml.Directive:
			// Native XML diagnostics carry no testcase outcome.
		default:
			consumeErr = observationFailure("JUnit decoder returned an unknown native token", core.ErrPrimitiveContract)
		}
		if consumeErr != nil {
			return JUnitSummary{}, consumeErr
		}
	}
	return state.finish()
}

func (s *junitStreamState) consumeText(value xml.CharData) error {
	if s.depth == 0 {
		for _, character := range value {
			if character != ' ' && character != '\t' && character != '\n' && character != '\r' {
				return observationFailure("JUnit XML contains text outside its root", core.ErrPrimitiveContract)
			}
		}
	}
	return nil
}

func (s *junitStreamState) consumeStart(element xml.StartElement) error {
	if duplicateJUnitAttribute(element.Attr) {
		return observationFailure("JUnit XML contains a duplicate attribute", core.ErrPrimitiveContract)
	}
	if s.depth == 0 {
		if s.rootSeen {
			return observationFailure("JUnit XML contains multiple roots", core.ErrPrimitiveContract)
		}
		if element.Name.Local != "testsuite" && element.Name.Local != "testsuites" {
			return observationFailure("JUnit XML root is not testsuite or testsuites", core.ErrPrimitiveContract)
		}
		s.rootSeen = true
		s.rootSuites = element.Name.Local == "testsuites"
	}
	switch element.Name.Local {
	case "testsuites":
		if s.depth != 0 {
			return observationFailure("JUnit XML nests a testsuites root", core.ErrPrimitiveContract)
		}
	case "testsuite":
		if s.depth != 0 && !(s.suiteDepth > 0 && s.depth == s.suiteDepth) && !(s.depth == 1 && s.rootSuites) {
			return observationFailure("JUnit testsuite is outside a suite container", core.ErrPrimitiveContract)
		}
		s.suiteDepth = s.depth + 1
	case junitTestCaseElementText:
		if s.caseDepth != 0 || s.suiteDepth == 0 || s.depth != s.suiteDepth {
			return observationFailure("JUnit testcase is not a direct suite child", core.ErrPrimitiveContract)
		}
		s.caseDepth, s.caseFailed, s.caseSkipped = s.depth+1, false, false
	case "failure", junitErrorElement, "skipped", "disabled":
		if s.caseDepth == 0 || s.depth != s.caseDepth {
			return observationFailure("JUnit outcome is not a direct testcase child", core.ErrPrimitiveContract)
		}
		if element.Name.Local == "failure" || element.Name.Local == junitErrorElement {
			s.caseFailed = true
		} else {
			s.caseSkipped = true
		}
	default:
		if s.depth == 1 && s.rootSuites || s.depth == s.suiteDepth && element.Name.Local != "properties" && element.Name.Local != "system-out" && element.Name.Local != "system-err" {
			return observationFailure("JUnit suite contains an unknown structural child", core.ErrPrimitiveContract)
		}
	}
	s.depth++
	return nil
}

func (s *junitStreamState) consumeEnd(element xml.EndElement) error {
	defer func() { s.depth-- }()
	switch element.Name.Local {
	case "testsuite":
		s.suiteDepth = s.depth - 1
		if s.rootSuites && s.depth == 2 {
			s.suiteDepth = 0
		}
	case junitTestCaseElementText:
		// The standard XML decoder already enforces matching start/end names.
		if s.caseDepth != s.depth {
			return observationFailure("JUnit XML closes an absent testcase", core.ErrPrimitiveContract)
		}
		if cases, err := s.summary.Cases(); err != nil || cases == math.MaxUint64 {
			return observationFailure("JUnit observed counts overflow", core.ErrPrimitiveContract, err)
		}
		switch {
		case s.caseFailed:
			s.summary.Failed++
		case s.caseSkipped:
			s.summary.Skipped++
		default:
			s.summary.Passed++
		}
		s.caseDepth = 0
	}
	return nil
}

func (s *junitStreamState) finish() (JUnitSummary, error) {
	cases, err := s.summary.Cases()
	if err != nil {
		return JUnitSummary{}, err
	}
	if !s.rootSeen || s.depth != 0 || s.caseDepth != 0 || cases == 0 {
		return JUnitSummary{}, observationFailure("JUnit XML contains no complete testcase evidence", core.ErrPrimitiveContract)
	}
	return s.summary, nil
}

var (
	_ core.Validatable = JUnitSummary{}
	_ io.Writer        = (*JUnitSummaryCompiler)(nil)
)
