package exchange

import "github.com/deliri/primitive/v2026/core"

func (r JSONRequest[Body]) Validate() error {
	if err := validateRequestMetadata(requestMetadata{
		target: r.Target, semantics: r.Semantics, headers: r.Headers,
		capture: r.CaptureHeaders, expected: r.ExpectedStatus,
	}); err != nil {
		return err
	}
	if err := validateCallerValue(r.Body); err != nil {
		return requestError(err)
	}
	return nil
}

func (r NoBodyRequest) Validate() error {
	return validateRequestMetadata(requestMetadata{
		target: r.Target, semantics: r.Semantics, headers: r.Headers,
		capture: r.CaptureHeaders, expected: r.ExpectedStatus,
	})
}

func (r NoBodyBoundedRequest) Validate() error {
	if err := validateRequestMetadata(requestMetadata{
		target: r.Target, semantics: r.Semantics, headers: r.Headers,
		capture: r.CaptureHeaders, expected: r.ExpectedStatus,
	}); err != nil {
		return err
	}
	if err := validateOptionalMediaType(
		r.ExpectedResponseContentType,
	); err != nil {
		return requestError(err)
	}
	return nil
}

func (r BoundedRequest) Validate() error {
	if err := validateRequestMetadata(requestMetadata{
		target: r.Target, semantics: r.Semantics, headers: r.Headers,
		capture: r.CaptureHeaders, expected: r.ExpectedStatus,
	}); err != nil {
		return err
	}
	if err := r.RequestContentType.Validate(); err != nil {
		return requestError(core.ErrExchangeContentType)
	}
	if err := validateOptionalMediaType(r.ExpectedResponseContentType); err != nil {
		return requestError(err)
	}
	return nil
}

func (r UploadRequest) Validate() error {
	if err := validateRequestMetadata(requestMetadata{
		target: r.Target, semantics: r.Semantics, headers: r.Headers,
		capture: r.CaptureHeaders, expected: r.ExpectedStatus,
	}); err != nil {
		return err
	}
	if r.Source == nil {
		return requestError(core.ErrExchangeContract)
	}
	if r.Semantics.Replay != ReplaySingleAttempt &&
		r.Semantics.Replay != ReplaySingleAttemptWithIdempotencyKey {
		return requestError(core.ErrExchangeContract)
	}
	if _, err := r.ContentLength.Int64(); err != nil {
		return requestError(err)
	}
	if err := r.ContentType.Validate(); err != nil {
		return requestError(core.ErrExchangeContentType)
	}
	return nil
}

func (r DownloadRequest) Validate() error {
	if err := validateRequestMetadata(requestMetadata{
		target: r.Target, semantics: r.Semantics, headers: r.Headers,
		capture: r.CaptureHeaders, expected: r.ExpectedStatus,
	}); err != nil {
		return err
	}
	if r.Destination == nil {
		return requestError(core.ErrExchangeContract)
	}
	if r.Semantics.Replay != ReplaySingleAttempt {
		return requestError(core.ErrExchangeContract)
	}
	if _, err := r.ResponseBodyLimit.Int64(); err != nil {
		return requestError(err)
	}
	if err := validateOptionalMediaType(r.ExpectedResponseContentType); err != nil {
		return requestError(err)
	}
	return nil
}

func (r StreamRoundTripRequest) Validate() error {
	if err := validateRequestMetadata(requestMetadata{
		target: r.Target, semantics: r.Semantics, headers: r.Headers,
		capture: r.CaptureHeaders, expected: r.ExpectedStatus,
	}); err != nil {
		return err
	}
	if r.Source == nil || r.Destination == nil ||
		(r.Semantics.Replay != ReplaySingleAttempt &&
			r.Semantics.Replay != ReplaySingleAttemptWithIdempotencyKey) {
		return requestError(core.ErrExchangeContract)
	}
	if _, err := r.RequestContentLength.Int64(); err != nil {
		return requestError(err)
	}
	if _, err := r.ResponseBodyLimit.Int64(); err != nil {
		return requestError(err)
	}
	if err := r.RequestContentType.Validate(); err != nil {
		return requestError(core.ErrExchangeContentType)
	}
	if err := validateOptionalMediaType(r.ExpectedResponseContentType); err != nil {
		return requestError(err)
	}
	return nil
}

type requestMetadata struct {
	target    Target
	semantics RequestSemantics
	headers   Headers
	capture   HeaderSelection
	expected  core.HTTPStatusCode
}

func validateRequestMetadata(metadata requestMetadata) (err error) {
	defer func() {
		if recover() != nil {
			err = requestError(core.ErrExchangeContract)
		}
	}()
	if metadata.target == nil {
		return requestError(core.ErrExchangeContract)
	}
	if err := metadata.target.Validate(); err != nil {
		return requestError(err)
	}
	if err := metadata.semantics.Validate(); err != nil {
		return requestError(err)
	}
	if err := metadata.headers.Validate(); err != nil {
		return requestError(err)
	}
	if err := metadata.capture.Validate(); err != nil {
		return requestError(err)
	}
	if err := metadata.expected.Validate(); err != nil {
		return requestError(err)
	}
	return nil
}

func validateOptionalMediaType(mediaType core.HTTPMediaType) error {
	if mediaType.IsZero() {
		return nil
	}
	if err := mediaType.Validate(); err != nil {
		return core.ErrExchangeContentType
	}
	return nil
}

func validateCallerValue(value core.Validatable) (err error) {
	defer func() {
		if recover() != nil {
			err = core.ErrExchangeContract
		}
	}()
	if value == nil {
		return core.ErrExchangeContract
	}
	return value.Validate()
}

var (
	_ core.Validatable = NoBodyRequest{}
	_ core.Validatable = NoBodyBoundedRequest{}
	_ core.Validatable = BoundedRequest{}
	_ core.Validatable = UploadRequest{}
	_ core.Validatable = DownloadRequest{}
)
