package runprotocol

import (
	"errors"
	"testing"

	"github.com/deliri/primitive/v2026/core"
)

func TestScalingCaptureRecordsRawFactsWithoutInterpretingTheirMeaning(t *testing.T) {
	t.Parallel()

	cases := []struct {
		setup   func(testing.TB) ScalingCapture
		wantErr error
		name    string
	}{
		{name: "minimum three ordered samples are admitted", setup: func(t testing.TB) ScalingCapture { return fixtureScalingCapture(t, ScalingSampleMinimum) }},
		{name: "maximum ordered samples are admitted", setup: func(t testing.TB) ScalingCapture { return fixtureScalingCapture(t, ScalingSampleMaximum) }},
		{name: "one sample below minimum is refused", setup: func(t testing.TB) ScalingCapture { return fixtureScalingCapture(t, ScalingSampleMinimum-1) }, wantErr: core.ErrRunProtocolContract},
		{name: "one sample above maximum is refused", setup: func(t testing.TB) ScalingCapture { return fixtureScalingCapture(t, ScalingSampleMaximum+1) }, wantErr: core.ErrRunProtocolContract},
		{name: "zero execution budget is refused", setup: func(t testing.TB) ScalingCapture {
			got := fixtureScalingCapture(t, ScalingSampleMinimum)
			got.BudgetSeconds = 0
			return got
		}, wantErr: core.ErrRunProtocolContract},
		{name: "missing measurement identity is refused", setup: func(t testing.TB) ScalingCapture {
			got := fixtureScalingCapture(t, ScalingSampleMinimum)
			got.Measurement = Identifier{}
			return got
		}, wantErr: core.ErrRunProtocolContract},
		{name: "missing profile identity is refused", setup: func(t testing.TB) ScalingCapture {
			got := fixtureScalingCapture(t, ScalingSampleMinimum)
			got.Profile = ProfileIdentity{}
			return got
		}, wantErr: core.ErrRunProtocolContract},
		{name: "missing environment fingerprint is refused", setup: func(t testing.TB) ScalingCapture {
			got := fixtureScalingCapture(t, ScalingSampleMinimum)
			got.EnvironmentFingerprint = core.SHA256Digest{}
			return got
		}, wantErr: core.ErrRunProtocolContract},
		{name: "zero input size is refused", setup: func(t testing.TB) ScalingCapture {
			got := fixtureScalingCapture(t, ScalingSampleMinimum)
			got.Samples[0].InputSize = 0
			return got
		}, wantErr: core.ErrRunProtocolContract},
		{name: "zero iteration count is refused", setup: func(t testing.TB) ScalingCapture {
			got := fixtureScalingCapture(t, ScalingSampleMinimum)
			got.Samples[0].Iterations = 0
			return got
		}, wantErr: core.ErrRunProtocolContract},
		{name: "zero duration is refused", setup: func(t testing.TB) ScalingCapture {
			got := fixtureScalingCapture(t, ScalingSampleMinimum)
			got.Samples[0].NanosecondsPerOp = 0
			return got
		}, wantErr: core.ErrRunProtocolContract},
		{name: "duplicate input coordinate is refused", setup: func(t testing.TB) ScalingCapture {
			got := fixtureScalingCapture(t, ScalingSampleMinimum)
			got.Samples[1].InputSize = got.Samples[0].InputSize
			return got
		}, wantErr: core.ErrRunProtocolConflict},
		{name: "descending input coordinate is refused", setup: func(t testing.TB) ScalingCapture {
			got := fixtureScalingCapture(t, ScalingSampleMinimum)
			got.Samples[2].InputSize = got.Samples[1].InputSize - 1
			return got
		}, wantErr: core.ErrRunProtocolConflict},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			gotErr := testCase.setup(t).Validate()
			if testCase.wantErr == nil && gotErr != nil {
				t.Fatalf("ScalingCapture.Validate() error = %v, want nil", gotErr)
			}
			if testCase.wantErr != nil && !errors.Is(gotErr, testCase.wantErr) {
				t.Fatalf("ScalingCapture.Validate() error = %v, want %v", gotErr, testCase.wantErr)
			}
		})
	}
}

func fixtureScalingCapture(t testing.TB, sampleCount int) ScalingCapture {
	t.Helper()
	samples := make([]ScalingSample, sampleCount)
	for index := range samples {
		samples[index] = ScalingSample{
			InputSize:        uint64(index + 1),
			Iterations:       100,
			NanosecondsPerOp: uint64(index + 10),
		}
	}
	return ScalingCapture{
		Measurement:            fixtureIdentifier(t, "encode-envelope"),
		Profile:                fixtureProfile(t, "acceptance"),
		Samples:                samples,
		BudgetSeconds:          30,
		EnvironmentFingerprint: core.NewSHA256Digest([core.SHA256DigestBytes]byte{1}),
	}
}
