package stripe

import (
	"context"
	"errors"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/deliri/primitive/v2026/contextstate"
	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/jsonio"
)

// CheckoutSessionDecodeRequest lends one unexpanded Checkout Session document.
// The source owner supplies interruption for a blocked Read and owns its close.
type CheckoutSessionDecodeRequest struct {
	Source       io.Reader
	MaximumBytes core.ByteCount
}

func (r CheckoutSessionDecodeRequest) Validate() error {
	maximum, err := r.MaximumBytes.Int64()
	if core.ReaderIsNil(r.Source) || err != nil || maximum == math.MaxInt64 {
		return contractError(errors.Join(core.ErrJSONContract, err))
	}
	return nil
}

// CheckoutSession is a provider projection, not a payment decision. Successful
// decoding requires every selected member: nil pointers mean explicit null,
// never omitted members. Amount and currency are independently nullable.
// Metadata's owner validates its own meaning. Expanded references are refused.
type CheckoutSession[Metadata core.Validatable] struct {
	ID            string
	Object        string
	Live          bool
	Status        CheckoutSessionStatus
	PaymentStatus CheckoutSessionPaymentStatus
	Mode          CheckoutSessionMode
	AmountTotal   *int64
	Currency      *string
	Customer      *string
	Subscription  *string
	Metadata      Metadata
}

func (s CheckoutSession[Metadata]) Validate() error {
	if !checkoutReference(s.ID, "cs_") || s.Object != "checkout.session" ||
		!s.Status.IsValid() || !s.PaymentStatus.IsValid() || !s.Mode.IsValid() ||
		!checkoutNullableReference(s.Customer, "cus_") || !checkoutNullableReference(s.Subscription, "sub_") {
		return core.ErrStripeContract
	}
	if s.Currency != nil {
		if len(*s.Currency) != 3 {
			return core.ErrStripeContract
		}
		for _, c := range *s.Currency {
			if c < 'a' || c > 'z' {
				return core.ErrStripeContract
			}
		}
	}
	if err := s.Metadata.Validate(); err != nil {
		return contractError(err)
	}
	return nil
}

func checkoutReference(value, prefix string) bool {
	if !strings.HasPrefix(value, prefix) || len(value) == len(prefix) {
		return false
	}
	for _, c := range value[len(prefix):] {
		if c != '_' && (c < 'a' || c > 'z') && (c < 'A' || c > 'Z') && (c < '0' || c > '9') {
			return false
		}
	}
	return true
}

func checkoutNullableReference(value *string, prefix string) bool {
	return value == nil || checkoutReference(*value, prefix)
}

// DecodeCheckoutSession projects exactly one document using Go-owned JSON
// grammar. Foreign unselected members are traversed, not retained. Neither
// authenticated provenance nor application eligibility is inferred here.
// Every refusal withdraws the complete candidate and preserves its cause.
func DecodeCheckoutSession[Metadata core.Validatable](ctx context.Context, request CheckoutSessionDecodeRequest) (CheckoutSession[Metadata], error) {
	if err := errors.Join(request.Validate(), checkoutSessionContextError(ctx)); err != nil {
		return CheckoutSession[Metadata]{}, contractError(err)
	}
	maximum, err := request.MaximumBytes.Int64()
	if err != nil {
		return CheckoutSession[Metadata]{}, contractError(err)
	}
	bounded := io.LimitedReader{R: request.Source, N: maximum + 1}
	cursor, err := jsonio.NewTokenDecoder(jsonio.TokenRequest{Source: &bounded, NestingDepthMaximum: core.JSONNestingDepthMaximum})
	if err != nil {
		return CheckoutSession[Metadata]{}, contractError(err)
	}
	result, err := decodeCheckoutSession[Metadata](ctx, cursor)
	if bounded.N == 0 {
		err = errors.Join(err, &jsonio.ExtentError{Maximum: request.MaximumBytes})
	}
	err = errors.Join(err, checkoutSessionContextError(ctx))
	if err != nil {
		return CheckoutSession[Metadata]{}, contractError(err)
	}
	return result, nil
}

// Contextstate admits the native terminal state. Cause is observed only for
// that state; a hostile Value method cannot replace the admitted sentinel.
func checkoutSessionContextError(ctx context.Context) (err error) {
	err = contextstate.Validate(ctx)
	if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	defer func() { _ = recover() }()
	return errors.Join(err, context.Cause(ctx))
}

func decodeCheckoutSession[Metadata core.Validatable](ctx context.Context, cursor *jsonio.TokenDecoder) (CheckoutSession[Metadata], error) {
	var result CheckoutSession[Metadata]
	token, err := cursor.Decode(ctx)
	if err != nil || token.Kind != jsonio.TokenObjectStart {
		return result, errors.Join(core.ErrJSONContract, err)
	}
	var seen uint16
	for {
		token, err = cursor.Decode(ctx)
		if err != nil {
			return CheckoutSession[Metadata]{}, err
		}
		if token.Kind == jsonio.TokenObjectEnd {
			break
		}
		if token.Kind != jsonio.TokenString {
			return CheckoutSession[Metadata]{}, core.ErrJSONContract
		}
		member := checkoutSessionMember(token.Text)
		if member == 0 {
			err = cursor.SkipValue(ctx)
		} else if seen&member != 0 {
			err = core.ErrJSONContract
		} else {
			seen |= member
			err = result.decodeMember(ctx, cursor, token.Text)
		}
		if err != nil {
			return CheckoutSession[Metadata]{}, err
		}
	}
	if seen != (1<<11)-1 {
		return CheckoutSession[Metadata]{}, core.ErrJSONContract
	}
	if err := errors.Join(cursor.RequireEOF(ctx), result.Validate()); err != nil {
		return CheckoutSession[Metadata]{}, err
	}
	return result, nil
}

func checkoutSessionMember(name string) uint16 {
	switch name {
	case "id":
		return 1 << 0
	case "object":
		return 1 << 1
	case "livemode":
		return 1 << 2
	case "status":
		return 1 << 3
	case "payment_status":
		return 1 << 4
	case "mode":
		return 1 << 5
	case "amount_total":
		return 1 << 6
	case "currency":
		return 1 << 7
	case "customer":
		return 1 << 8
	case "subscription":
		return 1 << 9
	case "metadata":
		return 1 << 10
	default:
		return 0
	}
}

func (s *CheckoutSession[Metadata]) decodeMember(ctx context.Context, cursor *jsonio.TokenDecoder, name string) error {
	if name == "metadata" {
		value, err := jsonio.DecodeObjectAt[Metadata](ctx, cursor, jsonio.MembersIgnoreExtra)
		s.Metadata = value
		return err
	}
	token, err := cursor.Decode(ctx)
	if err != nil {
		return err
	}
	switch name {
	case "id", "object":
		if token.Kind != jsonio.TokenString {
			return core.ErrJSONContract
		}
		if name == "id" {
			s.ID = token.Text
		} else {
			s.Object = token.Text
		}
	case "livemode":
		if token.Kind != jsonio.TokenTrue && token.Kind != jsonio.TokenFalse {
			return core.ErrJSONContract
		}
		s.Live = token.Kind == jsonio.TokenTrue
	case "status":
		s.Status, err = checkoutStatus(token)
	case "payment_status":
		s.PaymentStatus, err = checkoutPaymentStatus(token)
	case "mode":
		s.Mode, err = checkoutMode(token)
	case "amount_total":
		if token.Kind == jsonio.TokenNull {
			return nil
		}
		if token.Kind != jsonio.TokenNumber {
			return core.ErrJSONContract
		}
		var value int64
		value, err = strconv.ParseInt(token.Text, 10, 64)
		if err == nil {
			s.AmountTotal = &value
		}
	case "currency", "customer", "subscription":
		if token.Kind == jsonio.TokenNull {
			return nil
		}
		if token.Kind != jsonio.TokenString {
			return core.ErrJSONContract
		}
		value := token.Text
		switch name {
		case "currency":
			s.Currency = &value
		case "customer":
			s.Customer = &value
		case "subscription":
			s.Subscription = &value
		}
	default:
		return core.ErrJSONContract
	}
	return err
}

var _ core.Validatable = CheckoutSessionDecodeRequest{}
