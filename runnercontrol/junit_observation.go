package runnercontrol

import (
	"encoding/xml"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/runprotocol"
)

type JUnitObservation struct {
	Accounting runprotocol.ExecutionAccounting `json:"accounting"`
}

func (o JUnitObservation) Validate() error { return o.Accounting.Validate() }

type junitCompileResult struct {
	err     error
	attempt runprotocol.ExecutionAttempt
}

type junitStreamState struct {
	policy      ObservationPolicy
	attempt     runprotocol.ExecutionAttempt
	inCase      bool
	caseFailed  bool
	caseSkipped bool
	observed    uint32
	depth       int
	rootSeen    bool
}

// JUnitObservationCompiler streams one JUnit document through the standard XML
// decoder. The decoder owns its current materialized token and nesting stack;
// memory depends on those values, not the total report length. This API does
// not promise fixed memory for an arbitrarily large XML token. The caller owns
// exactly one terminal action: Seal
// closes successful input, while Abort closes abandoned input. Both join the
// parser before returning.
type JUnitObservationCompiler struct {
	failure error
	writer  *io.PipeWriter
	done    <-chan junitCompileResult
	policy  ObservationPolicy
	sealed  bool
}

func NewJUnitObservationCompiler(policy ObservationPolicy) (*JUnitObservationCompiler, error) {
	if err := policy.Validate(); err != nil {
		return nil, err
	}
	if policy.Format != ObservationJUnitXML {
		return nil, observationFailure("JUnit observation compiler requires junit-xml format", core.ErrPrimitiveContract)
	}
	reader, writer := io.Pipe()
	done := make(chan junitCompileResult, 1)
	go func() {
		attempt, err := parseJUnitStream(reader, policy)
		closeErr := reader.Close()
		done <- junitCompileResult{attempt: attempt, err: errors.Join(err, closeErr)}
		close(done)
	}()
	return &JUnitObservationCompiler{policy: policy, writer: writer, done: done}, nil
}

func (c *JUnitObservationCompiler) Write(data []byte) (int, error) {
	if c == nil || c.writer == nil || c.sealed {
		return 0, observationFailure("JUnit observation compiler is not writable", core.ErrPrimitiveContract)
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

func (c *JUnitObservationCompiler) Seal(executionErr error) (JUnitObservation, error) {
	if c == nil || c.writer == nil || c.sealed {
		return JUnitObservation{}, observationFailure("JUnit observation compiler cannot be sealed", core.ErrPrimitiveContract)
	}
	c.sealed = true
	closeErr := c.writer.Close()
	result := <-c.done
	if err := errors.Join(c.failure, closeErr, result.err); err != nil {
		return unavailableJUnitObservation(c.policy), err
	}
	if (executionErr == nil) != (result.attempt.Failed == 0) {
		return unavailableJUnitObservation(c.policy), observationFailure("JUnit terminal counts disagree with the process exit", core.ErrPrimitiveContract, executionErr)
	}
	observation := JUnitObservation{Accounting: runprotocol.ExecutionAccounting{Attempts: []runprotocol.ExecutionAttempt{result.attempt}}}
	return observation, observation.Validate()
}

// Abort releases an unsealed compiler and joins its parser. It is idempotent so
// a caller may defer it immediately after construction and still Seal on the
// successful path.
func (c *JUnitObservationCompiler) Abort() {
	if c == nil || c.writer == nil || c.sealed {
		return
	}
	c.sealed = true
	_ = c.writer.CloseWithError(core.ErrPrimitiveContract)
	<-c.done
}

func parseJUnitStream(reader io.Reader, policy ObservationPolicy) (runprotocol.ExecutionAttempt, error) {
	decoder := xml.NewDecoder(reader)
	state := junitStreamState{
		policy:  policy,
		attempt: runprotocol.ExecutionAttempt{Sequence: 1, Planned: policy.ExpectedUnits, Cache: runprotocol.CacheDisabled, Filtered: policy.Filtered},
	}
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return runprotocol.ExecutionAttempt{}, observationFailure("JUnit XML cannot be decoded", core.ErrPrimitiveContract, err)
		}
		if err := state.consume(token); err != nil {
			return runprotocol.ExecutionAttempt{}, err
		}
	}
	return state.finish()
}

func (s *junitStreamState) consume(token xml.Token) error {
	switch value := token.(type) {
	case xml.StartElement:
		return s.consumeStart(value)
	case xml.EndElement:
		return s.consumeEnd(value)
	case xml.CharData:
		if s.depth == 0 {
			for _, character := range value {
				if character != ' ' && character != '\t' && character != '\n' && character != '\r' {
					return observationFailure("JUnit XML contains text outside its root", core.ErrPrimitiveContract)
				}
			}
		}
	}
	return nil
}

func (s *junitStreamState) consumeStart(element xml.StartElement) error {
	if s.depth == 0 {
		if s.rootSeen {
			return observationFailure("JUnit XML contains multiple roots", core.ErrPrimitiveContract)
		}
		s.rootSeen = true
	}
	s.depth++
	if duplicateJUnitAttribute(element.Attr) {
		return observationFailure("JUnit XML contains a duplicate attribute", core.ErrPrimitiveContract)
	}
	switch element.Name.Local {
	case junitTestCaseElementText:
		if s.inCase {
			return observationFailure("JUnit XML nests testcase elements", core.ErrPrimitiveContract)
		}
		s.inCase, s.caseFailed, s.caseSkipped = true, false, false
	case "failure", "error":
		s.caseFailed = s.inCase
	case "skipped", "disabled":
		s.caseSkipped = s.inCase
	}
	return nil
}

func (s *junitStreamState) consumeEnd(element xml.EndElement) error {
	defer func() { s.depth-- }()
	if element.Name.Local != junitTestCaseElementText {
		return nil
	}
	if !s.inCase {
		return observationFailure("JUnit XML closes an absent testcase", core.ErrPrimitiveContract)
	}
	if s.observed >= s.policy.ExpectedUnits {
		return observationFailure("JUnit XML names more test cases than planned", core.ErrPrimitiveContract)
	}
	s.observed++
	s.recordCase()
	s.inCase = false
	return nil
}

func (s *junitStreamState) recordCase() {
	switch {
	case s.caseFailed:
		s.attempt.Failed++
	case s.caseSkipped:
		s.attempt.Skipped++
	default:
		s.attempt.Passed++
	}
}

func (s *junitStreamState) finish() (runprotocol.ExecutionAttempt, error) {
	if s.inCase {
		return runprotocol.ExecutionAttempt{}, observationFailure("JUnit XML ends inside a testcase", core.ErrPrimitiveContract)
	}
	if s.observed == 0 {
		return runprotocol.ExecutionAttempt{}, observationFailure("JUnit XML contains no testcase evidence", core.ErrPrimitiveContract)
	}
	if s.observed < s.policy.ExpectedUnits {
		s.attempt.NotRun = s.policy.ExpectedUnits - s.observed
	}
	if err := s.attempt.Validate(); err != nil {
		return runprotocol.ExecutionAttempt{}, observationFailure("JUnit accounting does not close", core.ErrPrimitiveContract, err)
	}
	return s.attempt, nil
}

func duplicateJUnitAttribute(attributes []xml.Attr) bool {
	for index := range attributes {
		for previous := range index {
			if attributes[previous].Name == attributes[index].Name {
				return true
			}
		}
	}
	return false
}

func unavailableJUnitObservation(policy ObservationPolicy) JUnitObservation {
	return JUnitObservation{Accounting: runprotocol.ExecutionAccounting{Attempts: []runprotocol.ExecutionAttempt{{Sequence: 1, Planned: policy.ExpectedUnits, Unavailable: policy.ExpectedUnits, Cache: runprotocol.CacheDisabled, Filtered: policy.Filtered}}}}
}

var (
	_ core.Validatable = JUnitObservation{}
	_ io.Writer        = (*JUnitObservationCompiler)(nil)
)
