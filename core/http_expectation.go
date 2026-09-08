package core

const httpHeaderExpectText = "Expect"

// HTTPExpectContinueValue is Go's standard request-body admission expectation.
const HTTPExpectContinueValue = "100-continue"

// HTTPHeaderExpect returns the validated HTTP expectation field name.
func HTTPHeaderExpect() HTTPHeaderName { return HTTPHeaderName{value: httpHeaderExpectText} }
