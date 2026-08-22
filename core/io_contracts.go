package core

import (
	"io"
	"reflect"
)

// ReaderConsecutiveEmptyReadMaximum is the common streaming refusal threshold
// for consecutive io.Reader results of (0, nil). The standard library permits
// that result transiently but defines io.ErrNoProgress for repeated instances;
// Primitive bounds the otherwise unending wait at this shared ceiling.
const ReaderConsecutiveEmptyReadMaximum = 100

// ReaderIsNil reports whether reader is a nil interface or contains a typed
// nil value. It closes the shared io.Reader ingress rule before a package calls
// a method on an externally supplied implementation.
func ReaderIsNil(reader io.Reader) bool {
	if reader == nil {
		return true
	}
	value := reflect.ValueOf(reader)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
