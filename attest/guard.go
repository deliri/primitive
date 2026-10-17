package attest

import "errors"

type guardedResult[T any] struct {
	value T
	err   error
}

func guardedCall[T any](call func() (T, error)) (T, error) {
	var result guardedResult[T]
	func() {
		defer func() {
			if caught := recover(); caught != nil {
				var zero T
				result.value = zero
				result.err = errors.New(panicAtConsumerBoundaryErrorText)
				if cause, ok := caught.(error); ok {
					result.err = errors.Join(result.err, cause)
				}
			}
		}()
		result.value, result.err = call()
	}()
	return result.value, result.err
}
