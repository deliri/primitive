package runprotocol

import (
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

const (
	ScalingCaptureMaximum = 16
	ScalingSampleMinimum  = 3
	ScalingSampleMaximum  = 16
)

// ScalingSample is one observed benchmark coordinate. It records what ran;
// interpreting the curve or deciding whether it satisfies a product claim is
// outside this mechanical run agreement.
type ScalingSample struct {
	InputSize        uint64 `json:"input_size"`
	Iterations       uint64 `json:"iterations"`
	NanosecondsPerOp uint64 `json:"nanoseconds_per_op"`
	BytesPerOp       uint64 `json:"bytes_per_op"`
	AllocationsPerOp uint64 `json:"allocations_per_op"`
}

func (s ScalingSample) Validate() error {
	if s.InputSize == 0 || s.Iterations == 0 || s.NanosecondsPerOp == 0 {
		return contractError(errors.New("run protocol scaling sample is invalid"))
	}
	return nil
}

// ScalingCapture binds a canonical sequence of raw samples to the exact
// profile and machine environment that produced them. It deliberately carries
// no human claim or pass/fail assessment.
type ScalingCapture struct {
	Measurement            Identifier        `json:"measurement"`
	Profile                ProfileIdentity   `json:"profile"`
	Samples                []ScalingSample   `json:"samples"`
	BudgetSeconds          uint32            `json:"budget_seconds"`
	EnvironmentFingerprint core.SHA256Digest `json:"environment_fingerprint"`
}

func (c ScalingCapture) Validate() error {
	if c.BudgetSeconds == 0 || len(c.Samples) < ScalingSampleMinimum || len(c.Samples) > ScalingSampleMaximum {
		return contractError(errors.New("run protocol scaling capture bounds are invalid"))
	}
	if err := contractJoin(c.Measurement.Validate(), c.EnvironmentFingerprint.Validate(), c.Profile.Validate()); err != nil {
		return err
	}
	for index := range c.Samples {
		if err := c.Samples[index].Validate(); err != nil {
			return err
		}
		if index > 0 && c.Samples[index].InputSize <= c.Samples[index-1].InputSize {
			return conflictError(errors.New("run protocol scaling samples are not strictly increasing"))
		}
	}
	return nil
}
