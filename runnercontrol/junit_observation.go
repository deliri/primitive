package runnercontrol

import (
	"encoding/xml"
	"errors"
	"io"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/projectstandards"
)

const (
	JUnitXMLMaximumBytes      uint64 = 8 << 20
	JUnitXMLTokenMaximumBytes        = 1 << 20
	JUnitXMLDepthMaximum             = 64
)

type JUnitObservation struct {
	Accounting projectstandards.ExecutionAccounting `json:"accounting"`
}

func (o JUnitObservation) Validate() error { return o.Accounting.Validate() }

type junitCompileResult struct {
	attempt projectstandards.ExecutionAttempt
	err     error
}

type junitStreamState struct {
	policy      ObservationPolicy
	attempt     projectstandards.ExecutionAttempt
	inCase      bool
	caseFailed  bool
	caseSkipped bool
	observed    uint32
	depth       int
}

// JUnitObservationCompiler streams one bounded JUnit report through the
// standard XML decoder. The caller owns exactly one terminal action: Seal
// closes successful input, while Abort closes abandoned input. Both join the
// parser before returning.
type JUnitObservationCompiler struct {
	policy  ObservationPolicy
	writer  *io.PipeWriter
	done    <-chan junitCompileResult
	sealed  bool
	bytes   uint64
	failure error
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
	if uint64(len(data)) > JUnitXMLMaximumBytes-c.bytes {
		c.failure = observationFailure("JUnit XML exceeds the byte ceiling", core.ErrPrimitiveContract)
		_ = c.writer.CloseWithError(c.failure)
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
	writtenBytes, conversionErr := core.CheckedUint64FromInt64(int64(written))
	if conversionErr != nil {
		c.failure = observationFailure("JUnit XML write extent cannot be represented", core.ErrPrimitiveContract, conversionErr)
		return written, c.failure
	}
	c.bytes += writtenBytes
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
	observation := JUnitObservation{Accounting: projectstandards.ExecutionAccounting{Attempts: []projectstandards.ExecutionAttempt{result.attempt}}}
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

func parseJUnitStream(reader io.Reader, policy ObservationPolicy) (projectstandards.ExecutionAttempt, error) {
	decoder := xml.NewDecoder(reader)
	state := junitStreamState{
		policy:  policy,
		attempt: projectstandards.ExecutionAttempt{Sequence: 1, Planned: policy.ExpectedUnits, Cache: projectstandards.CacheDisabled, Filtered: policy.Filtered},
	}
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return projectstandards.ExecutionAttempt{}, observationFailure("JUnit XML cannot be decoded", core.ErrPrimitiveContract, err)
		}
		if err := state.consume(token); err != nil {
			return projectstandards.ExecutionAttempt{}, err
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
		if len(value) > JUnitXMLTokenMaximumBytes {
			return observationFailure("JUnit XML text token exceeds the byte ceiling", core.ErrPrimitiveContract)
		}
	}
	return nil
}

func (s *junitStreamState) consumeStart(element xml.StartElement) error {
	s.depth++
	if s.depth > JUnitXMLDepthMaximum {
		return observationFailure("JUnit XML exceeds the depth ceiling", core.ErrPrimitiveContract)
	}
	if duplicateJUnitAttribute(element.Attr) {
		return observationFailure("JUnit XML contains a duplicate attribute", core.ErrPrimitiveContract)
	}
	switch element.Name.Local {
	case "testcase":
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
	if element.Name.Local != "testcase" {
		return nil
	}
	if !s.inCase {
		return observationFailure("JUnit XML closes an absent testcase", core.ErrPrimitiveContract)
	}
	s.observed++
	if s.observed > s.policy.ExpectedUnits {
		return observationFailure("JUnit XML names more test cases than planned", core.ErrPrimitiveContract)
	}
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

func (s *junitStreamState) finish() (projectstandards.ExecutionAttempt, error) {
	if s.inCase {
		return projectstandards.ExecutionAttempt{}, observationFailure("JUnit XML ends inside a testcase", core.ErrPrimitiveContract)
	}
	if s.observed == 0 {
		return projectstandards.ExecutionAttempt{}, observationFailure("JUnit XML contains no testcase evidence", core.ErrPrimitiveContract)
	}
	if s.observed < s.policy.ExpectedUnits {
		s.attempt.NotRun = s.policy.ExpectedUnits - s.observed
	}
	if err := s.attempt.Validate(); err != nil {
		return projectstandards.ExecutionAttempt{}, observationFailure("JUnit accounting does not close", core.ErrPrimitiveContract, err)
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
	return JUnitObservation{Accounting: projectstandards.ExecutionAccounting{Attempts: []projectstandards.ExecutionAttempt{{Sequence: 1, Planned: policy.ExpectedUnits, Unavailable: policy.ExpectedUnits, Cache: projectstandards.CacheDisabled, Filtered: policy.Filtered}}}}
}

var (
	_ core.Validatable = JUnitObservation{}
	_ io.Writer        = (*JUnitObservationCompiler)(nil)
)
