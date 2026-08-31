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

// JUnitObservationCompiler streams one bounded JUnit report through the
// standard XML decoder. The parser goroutine is owned by Seal: closing the
// writer ends input and Seal joins the parser before returning.
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
		return len(data), nil
	}
	if uint64(len(data)) > JUnitXMLMaximumBytes-c.bytes {
		c.failure = observationFailure("JUnit XML exceeds the byte ceiling", core.ErrPrimitiveContract)
		_ = c.writer.CloseWithError(c.failure)
		return len(data), nil
	}
	c.bytes += uint64(len(data))
	written, err := c.writer.Write(data)
	if err != nil {
		c.failure = observationFailure("JUnit XML stream cannot be consumed", core.ErrPrimitiveContract, err)
		return len(data), nil
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
	observation := JUnitObservation{Accounting: projectstandards.ExecutionAccounting{Attempts: []projectstandards.ExecutionAttempt{result.attempt}}}
	return observation, observation.Validate()
}

func parseJUnitStream(reader io.Reader, policy ObservationPolicy) (projectstandards.ExecutionAttempt, error) {
	decoder := xml.NewDecoder(reader)
	attempt := projectstandards.ExecutionAttempt{Sequence: 1, Planned: policy.ExpectedUnits, Cache: projectstandards.CacheDisabled, Filtered: policy.Filtered}
	inCase := false
	caseFailed := false
	caseSkipped := false
	var observed uint32
	depth := 0
	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return projectstandards.ExecutionAttempt{}, observationFailure("JUnit XML cannot be decoded", core.ErrPrimitiveContract, err)
		}
		switch value := token.(type) {
		case xml.StartElement:
			depth++
			if depth > JUnitXMLDepthMaximum {
				return projectstandards.ExecutionAttempt{}, observationFailure("JUnit XML exceeds the depth ceiling", core.ErrPrimitiveContract)
			}
			if duplicateJUnitAttribute(value.Attr) {
				return projectstandards.ExecutionAttempt{}, observationFailure("JUnit XML contains a duplicate attribute", core.ErrPrimitiveContract)
			}
			switch value.Name.Local {
			case "testcase":
				if inCase {
					return projectstandards.ExecutionAttempt{}, observationFailure("JUnit XML nests testcase elements", core.ErrPrimitiveContract)
				}
				inCase, caseFailed, caseSkipped = true, false, false
			case "failure", "error":
				if inCase {
					caseFailed = true
				}
			case "skipped", "disabled":
				if inCase {
					caseSkipped = true
				}
			}
		case xml.EndElement:
			if value.Name.Local == "testcase" {
				if !inCase {
					return projectstandards.ExecutionAttempt{}, observationFailure("JUnit XML closes an absent testcase", core.ErrPrimitiveContract)
				}
				observed++
				if observed > policy.ExpectedUnits {
					return projectstandards.ExecutionAttempt{}, observationFailure("JUnit XML names more test cases than planned", core.ErrPrimitiveContract)
				}
				switch {
				case caseFailed:
					attempt.Failed++
				case caseSkipped:
					attempt.Skipped++
				default:
					attempt.Passed++
				}
				inCase = false
			}
			depth--
		case xml.CharData:
			if len(value) > JUnitXMLTokenMaximumBytes {
				return projectstandards.ExecutionAttempt{}, observationFailure("JUnit XML text token exceeds the byte ceiling", core.ErrPrimitiveContract)
			}
		}
	}
	if inCase {
		return projectstandards.ExecutionAttempt{}, observationFailure("JUnit XML ends inside a testcase", core.ErrPrimitiveContract)
	}
	if observed == 0 {
		return projectstandards.ExecutionAttempt{}, observationFailure("JUnit XML contains no testcase evidence", core.ErrPrimitiveContract)
	}
	if observed < policy.ExpectedUnits {
		attempt.NotRun = policy.ExpectedUnits - observed
	}
	if err := attempt.Validate(); err != nil {
		return projectstandards.ExecutionAttempt{}, observationFailure("JUnit accounting does not close", core.ErrPrimitiveContract, err)
	}
	return attempt, nil
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
