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
	return ioValueIsNil(reflect.ValueOf(reader))
}

// WriterIsNil reports whether writer is absent or contains a typed nil value.
// It applies the same admission rule as ReaderIsNil before stream execution.
func WriterIsNil(writer io.Writer) bool {
	return ioValueIsNil(reflect.ValueOf(writer))
}

func ioValueIsNil(value reflect.Value) bool {
	switch value.Kind() {
	case reflect.Invalid:
		return true
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
