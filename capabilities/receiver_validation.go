package capabilities

import "github.com/deliri/primitive/v2026/core"

// ValidatesReceiver reports a reviewed alternative to a receiver's Validate
// method. A true result promises that successful return requires a valid
// receiver and that an invalid receiver's refusal is preserved. It does not
// promise that a particular caller invokes the method or propagates its error;
// those are separate compiler observations.
//
// Only NumericInstant.Instant is currently advertised. An unadvertised valid
// symbol returns false, nil: neither its spelling nor package membership is
// proof of validation. StandardSymbol must identify the declaring receiver,
// rather than a caller's alias or variable name.
func (s StandardSymbol) ValidatesReceiver() (bool, error) {
	if err := s.Validate(); err != nil {
		return false, err
	}
	path, err := core.PackageTemporal.ImportPath()
	if err != nil {
		return false, err
	}
	return s.ImportPath.String() == path && s.Receiver != nil &&
		s.Receiver.String() == "NumericInstant" && s.Selector.String() == "Instant", nil
}
