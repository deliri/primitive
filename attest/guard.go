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
			if recover() != nil {
				var zero T
				result.value = zero
				result.err = errors.New(panicAtConsumerBoundaryErrorText)
			}
		}()
		result.value, result.err = call()
	}()
	return result.value, result.err
}
