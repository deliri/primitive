package runnercontrol

import (
	json "encoding/json/v2"
	"errors"

	"github.com/deliri/primitive/v2026/core"
)

const ExecutionAccountingUnitMaximum uint32 = 1 << 16

type ObservationFormat uint8

const (
	ObservationFormatUnknown ObservationFormat = iota
	ObservationOpaque
	ObservationGoTestJSON
	ObservationJUnitXML
	observationFormatLimit
)

func (f ObservationFormat) Validate() error {
	if f <= ObservationFormatUnknown || f >= observationFormatLimit {
		return core.ErrPrimitiveContract
	}
	return nil
}

func (f ObservationFormat) IsValid() bool { return f.Validate() == nil }

func (f ObservationFormat) String() string {
	if !f.IsValid() {
		return invalidEnumString()
	}
	return []string{"", "opaque", "go-test-json", "junit-xml"}[f]
}

func (f ObservationFormat) MarshalJSON() ([]byte, error) {
	if err := f.Validate(); err != nil {
		return nil, errors.Join(core.ErrJSONContract, err)
	}
	return core.MarshalCanonicalJSONString(f.String())
}

func (f *ObservationFormat) UnmarshalJSON(data []byte) error {
	if f == nil {
		return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
	}
	value, err := core.DecodeJSONStringToken(data)
	if err != nil {
		return err
	}
	for candidate := ObservationFormatUnknown + 1; candidate < observationFormatLimit; candidate++ {
		if candidate.String() == value {
			*f = candidate
			return nil
		}
	}
	return errors.Join(core.ErrJSONContract, core.ErrPrimitiveContract)
}

type ObservationPolicy struct {
	ExpectedUnits uint32            `json:"expected_units"`
	Format        ObservationFormat `json:"format"`
	Filtered      bool              `json:"filtered"`
}

func (p ObservationPolicy) Validate() error {
	if err := p.Format.Validate(); err != nil {
		return err
	}
	if p.ExpectedUnits > ExecutionAccountingUnitMaximum {
		return core.ErrPrimitiveContract
	}
	if p.Format == ObservationOpaque && (p.ExpectedUnits != 0 || p.Filtered) {
		return core.ErrPrimitiveContract
	}
	if (p.Format == ObservationGoTestJSON || p.Format == ObservationJUnitXML) && p.ExpectedUnits == 0 {
		return core.ErrPrimitiveContract
	}
	return nil
}

var (
	_ core.Validatable = ObservationFormatUnknown
	_ json.Unmarshaler = (*ObservationFormat)(nil)
	_ core.Validatable = ObservationPolicy{}
)
