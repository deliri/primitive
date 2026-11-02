package stripe

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"go/parser"
	"go/token"
	"io"
	"math"
	"strconv"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/jsonio"
	stripesdk "github.com/stripe/stripe-go/v86"
)

type checkoutMetadataFixture struct {
	Owner string `json:"user_id"`
	Order string `json:"order_id"`
}

func (m checkoutMetadataFixture) Validate() error {
	if m.Owner != "owner" || m.Order != "order" {
		return core.ErrStripeBinding
	}
	return nil
}

// The published provider types own the wire spellings. Nullable members use
// pointers because the official response's integer zero loses explicit null.
type checkoutSessionFixture struct {
	ID            string                                 `json:"id"`
	Object        string                                 `json:"object"`
	Live          bool                                   `json:"livemode"`
	Status        *stripesdk.CheckoutSessionStatus       `json:"status"`
	PaymentStatus stripesdk.CheckoutSessionPaymentStatus `json:"payment_status"`
	Mode          stripesdk.CheckoutSessionMode          `json:"mode"`
	AmountTotal   *int64                                 `json:"amount_total"`
	Currency      *string                                `json:"currency"`
	Customer      *string                                `json:"customer"`
	Subscription  *string                                `json:"subscription"`
	Metadata      checkoutMetadataFixture                `json:"metadata"`
}

func checkoutFixture() checkoutSessionFixture {
	status, total, code := stripesdk.CheckoutSessionStatusExpired, int64(0), "usd"
	return checkoutSessionFixture{
		ID: "cs_test_owned", Object: "checkout.session", Live: true,
		Status: &status, PaymentStatus: stripesdk.CheckoutSessionPaymentStatusUnpaid,
		Mode: stripesdk.CheckoutSessionModeSubscription, AmountTotal: &total, Currency: &code,
		Metadata: checkoutMetadataFixture{Owner: "owner", Order: "order"},
	}
}

func checkoutFixtureBytes(t testing.TB, fixture checkoutSessionFixture) []byte {
	t.Helper()
	var destination bytes.Buffer
	if err := jsonio.WriteProjection(t.Context(), &destination, fixture, jsonio.OwnedCodecPolicy{}, jsonio.LayoutCompact); err != nil {
		t.Fatalf("WriteProjection(checkout fixture) error = %v, want nil", err)
	}
	return destination.Bytes()
}

func checkoutDecodeRequest(t testing.TB, source io.Reader, maximum uint64) CheckoutSessionDecodeRequest {
	t.Helper()
	count, err := core.NewByteCount(maximum)
	if err != nil {
		t.Fatalf("NewByteCount(%d) error = %v, want nil", maximum, err)
	}
	return CheckoutSessionDecodeRequest{Source: source, MaximumBytes: count}
}

func TestCheckoutSessionProjectionLayerTriad(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		edit func(*checkoutSessionFixture)
	}{
		{name: "positive exact expired unpaid zero and explicit absent references"},
		{name: "neutral explicit null status remains observed null", edit: func(v *checkoutSessionFixture) { v.Status = nil }},
		{name: "open status is factual not eligible expiry", edit: func(v *checkoutSessionFixture) { s := stripesdk.CheckoutSessionStatusOpen; v.Status = &s }},
		{name: "complete status is factual not eligible expiry", edit: func(v *checkoutSessionFixture) { s := stripesdk.CheckoutSessionStatusComplete; v.Status = &s }},
		{name: "paid state preserved independently", edit: func(v *checkoutSessionFixture) { v.PaymentStatus = stripesdk.CheckoutSessionPaymentStatusPaid }},
		{name: "no payment required is not unpaid", edit: func(v *checkoutSessionFixture) {
			v.PaymentStatus = stripesdk.CheckoutSessionPaymentStatusNoPaymentRequired
		}},
		{name: "payment mode preserved", edit: func(v *checkoutSessionFixture) { v.Mode = stripesdk.CheckoutSessionModePayment }},
		{name: "setup mode preserved", edit: func(v *checkoutSessionFixture) { v.Mode = stripesdk.CheckoutSessionModeSetup }},
		{name: "test mode explicit false is present", edit: func(v *checkoutSessionFixture) { v.Live = false }},
		{name: "neutral explicit null amount is not zero", edit: func(v *checkoutSessionFixture) { v.AmountTotal = nil }},
		{name: "neutral null currency does not fabricate USD for numeric zero", edit: func(v *checkoutSessionFixture) { v.Currency = nil }},
		{name: "negative integer is preserved without invented finance policy", edit: func(v *checkoutSessionFixture) { n := int64(-1); v.AmountTotal = &n }},
		{name: "signed integer minimum remains exact", edit: func(v *checkoutSessionFixture) { n := int64(math.MinInt64); v.AmountTotal = &n }},
		{name: "signed integer maximum remains exact", edit: func(v *checkoutSessionFixture) { n := int64(math.MaxInt64); v.AmountTotal = &n }},
		{name: "nonnull customer is not absent", edit: func(v *checkoutSessionFixture) { s := "cus_owned"; v.Customer = &s }},
		{name: "nonnull subscription is not absent", edit: func(v *checkoutSessionFixture) { s := "sub_owned"; v.Subscription = &s }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			fixture := checkoutFixture()
			if tc.edit != nil {
				tc.edit(&fixture)
			}
			body := checkoutFixtureBytes(t, fixture)
			got, err := DecodeCheckoutSession[checkoutMetadataFixture](t.Context(), checkoutDecodeRequest(t, bytes.NewReader(body), uint64(len(body))))
			if err != nil || got.Validate() != nil {
				t.Fatalf("DecodeCheckoutSession() = (%+v, %v), want admitted provider facts", got, err)
			}
			wantStatus := "null"
			if fixture.Status != nil {
				wantStatus = string(*fixture.Status)
			}
			if got.ID != fixture.ID || got.Object != fixture.Object || got.Live != fixture.Live || got.Status.String() != wantStatus || got.PaymentStatus.String() != string(fixture.PaymentStatus) || got.Mode.String() != string(fixture.Mode) || got.Metadata != fixture.Metadata || !checkoutPointerEqual(got.AmountTotal, fixture.AmountTotal) || !checkoutPointerEqual(got.Currency, fixture.Currency) || !checkoutPointerEqual(got.Customer, fixture.Customer) || !checkoutPointerEqual(got.Subscription, fixture.Subscription) {
				t.Fatalf("DecodeCheckoutSession() = %+v, want exact typed fixture %+v", got, fixture)
			}
		})
	}
	t.Run("negative invalid metadata preserves owning refusal", func(t *testing.T) {
		t.Parallel()
		fixture := checkoutFixture()
		fixture.Metadata.Owner = "foreign"
		body := checkoutFixtureBytes(t, fixture)
		got, err := DecodeCheckoutSession[checkoutMetadataFixture](t.Context(), checkoutDecodeRequest(t, bytes.NewReader(body), uint64(len(body))))
		if !errors.Is(err, core.ErrStripeContract) || !errors.Is(err, core.ErrStripeBinding) || !checkoutSessionZero(got) {
			t.Fatalf("DecodeCheckoutSession(metadata refusal) = (%+v, %v), want zero and both owning identities", got, err)
		}
	})
}

func checkoutPointerEqual[T comparable](got, want *T) bool {
	return got == nil && want == nil || got != nil && want != nil && *got == *want
}

func checkoutSessionZero(got CheckoutSession[checkoutMetadataFixture]) bool {
	return got.ID == "" && got.Object == "" && !got.Live && got.Status == 0 && got.PaymentStatus == 0 && got.Mode == 0 && got.AmountTotal == nil && got.Currency == nil && got.Customer == nil && got.Subscription == nil && got.Metadata == (checkoutMetadataFixture{})
}

func TestCheckoutSessionRequiredMembers(t *testing.T) {
	t.Parallel()
	for _, member := range []string{"id", "object", "livemode", "status", "payment_status", "mode", "amount_total", "currency", "customer", "subscription", "metadata"} {
		t.Run("missing "+member+" cannot become zero null or false", func(t *testing.T) {
			t.Parallel()
			body := checkoutFixtureBytes(t, checkoutFixture())
			// Explicit wire mutation removes exactly one member from a typed seed.
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(body, &fields); err != nil {
				t.Fatal(err)
			}
			if _, exists := fields[member]; !exists {
				t.Fatalf("typed fixture member %s absent before mutation, want present", member)
			}
			delete(fields, member)
			mutated, err := json.Marshal(fields)
			if err != nil {
				t.Fatal(err)
			}
			got, err := DecodeCheckoutSession[checkoutMetadataFixture](t.Context(), checkoutDecodeRequest(t, bytes.NewReader(mutated), uint64(len(mutated))))
			if !errors.Is(err, core.ErrStripeContract) || !checkoutSessionZero(got) {
				t.Fatalf("DecodeCheckoutSession(missing %s) = (%+v, %v), want zero typed refusal", member, got, err)
			}
		})
	}
}

func TestCheckoutSessionHostileWire(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, old, replacement string
		cause                  error
	}{
		{name: "wrong object family", old: `"object":"checkout.session"`, replacement: `"object":"customer"`},
		{name: "foreign ID family", old: `"cs_test_owned"`, replacement: `"cus_owned"`},
		{name: "empty checkout ID suffix", old: `"cs_test_owned"`, replacement: `"cs_"`},
		{name: "checkout ID punctuation refused", old: `"cs_test_owned"`, replacement: `"cs_bad/path"`},
		{name: "null identity refused", old: `"id":"cs_test_owned"`, replacement: `"id":null`},
		{name: "null livemode cannot default false", old: `"livemode":true`, replacement: `"livemode":null`},
		{name: "string livemode cannot impersonate bool", old: `"livemode":true`, replacement: `"livemode":"true"`},
		{name: "future status refused", old: `"status":"expired"`, replacement: `"status":"future"`},
		{name: "numeric status refused", old: `"status":"expired"`, replacement: `"status":0`},
		{name: "future payment state refused", old: `"payment_status":"unpaid"`, replacement: `"payment_status":"future"`},
		{name: "null payment state refused", old: `"payment_status":"unpaid"`, replacement: `"payment_status":null`},
		{name: "future mode refused", old: `"mode":"subscription"`, replacement: `"mode":"future"`},
		{name: "null mode refused", old: `"mode":"subscription"`, replacement: `"mode":null`},
		{name: "fraction is not an integer", old: `"amount_total":0`, replacement: `"amount_total":0.5`, cause: strconv.ErrSyntax},
		{name: "exponent is not integer wire representation", old: `"amount_total":0`, replacement: `"amount_total":1e0`, cause: strconv.ErrSyntax},
		{name: "string amount is not zero", old: `"amount_total":0`, replacement: `"amount_total":"0"`},
		{name: "positive integer overflow", old: `"amount_total":0`, replacement: `"amount_total":9223372036854775808`, cause: strconv.ErrRange},
		{name: "negative integer overflow", old: `"amount_total":0`, replacement: `"amount_total":-9223372036854775809`, cause: strconv.ErrRange},
		{name: "currency wrong type", old: `"currency":"usd"`, replacement: `"currency":0`},
		{name: "currency wrong length", old: `"currency":"usd"`, replacement: `"currency":"us"`},
		{name: "currency uppercase is not provider grammar", old: `"currency":"usd"`, replacement: `"currency":"USD"`},
		{name: "empty customer is not explicit null", old: `"customer":null`, replacement: `"customer":""`},
		{name: "foreign customer reference refused", old: `"customer":null`, replacement: `"customer":"sub_owned"`},
		{name: "expanded customer outside unexpanded contract", old: `"customer":null`, replacement: `"customer":{"id":"cus_owned"}`},
		{name: "expanded subscription outside unexpanded contract", old: `"subscription":null`, replacement: `"subscription":{"id":"sub_owned"}`},
		{name: "subscription wrong type", old: `"subscription":null`, replacement: `"subscription":false`},
		{name: "null metadata is not empty admitted owner", old: `"metadata":{`, replacement: `"metadata":null,"ignored_metadata":{`},
		{name: "duplicate selected member refused", old: `"amount_total":0`, replacement: `"amount_total":0,"amount_total":0`},
		{name: "escaped duplicate member refused", old: `"amount_total":0`, replacement: `"amount_total":0,"amount_\u0074otal":0`},
		{name: "duplicate foreign members refused", old: `"amount_total":0`, replacement: `"extra":1,"extra":1,"amount_total":0`},
		{name: "duplicate ignored nested members refused", old: `"amount_total":0`, replacement: `"extra":{"a":1,"a":1},"amount_total":0`},
		{name: "duplicate metadata member refused", old: `"user_id":"owner"`, replacement: `"user_id":"owner","user_id":"owner"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := string(checkoutFixtureBytes(t, checkoutFixture()))
			if strings.Count(body, tc.old) != 1 {
				t.Fatalf("fixture mutation occurrences = %d, want exactly 1 for %s", strings.Count(body, tc.old), tc.old)
			}
			mutated := strings.Replace(body, tc.old, tc.replacement, 1)
			got, err := DecodeCheckoutSession[checkoutMetadataFixture](t.Context(), checkoutDecodeRequest(t, strings.NewReader(mutated), uint64(len(mutated))))
			if !errors.Is(err, core.ErrStripeContract) || tc.cause != nil && !errors.Is(err, tc.cause) || !checkoutSessionZero(got) {
				t.Fatalf("DecodeCheckoutSession(hostile) = (%+v, %v), want zero and Stripe/cause %v", got, err, tc.cause)
			}
		})
	}
}

func TestCheckoutSessionExtentAndSource(t *testing.T) {
	t.Parallel()
	t.Run("exact admitted byte extent and one either side", func(t *testing.T) {
		t.Parallel()
		body := checkoutFixtureBytes(t, checkoutFixture())
		for _, delta := range []int{-1, 0, 1} {
			maximum := uint64(len(body) + delta)
			got, err := DecodeCheckoutSession[checkoutMetadataFixture](t.Context(), checkoutDecodeRequest(t, bytes.NewReader(body), maximum))
			if delta < 0 {
				var extent *jsonio.ExtentError
				if !errors.Is(err, core.ErrStripeContract) || !errors.As(err, &extent) || !checkoutSessionZero(got) {
					t.Fatalf("DecodeCheckoutSession(max %d) = (%+v, %v), want zero typed extent refusal", maximum, got, err)
				}
			} else if err != nil || got.ID != "cs_test_owned" {
				t.Fatalf("DecodeCheckoutSession(max %d) = (%+v, %v), want full admitted checkout", maximum, got, err)
			}
		}
	})
	cases := []struct {
		name       string
		makeSource func([]byte) io.Reader
		want       error
	}{
		{name: "one byte reads preserve native grammar", makeSource: func(b []byte) io.Reader { return iotest.OneByteReader(bytes.NewReader(b)) }},
		{name: "source cause after complete JSON is not success", makeSource: func(b []byte) io.Reader {
			return io.MultiReader(bytes.NewReader(b), iotest.ErrReader(io.ErrClosedPipe))
		}, want: io.ErrClosedPipe},
		{name: "source cause inside metadata withdraws candidate", makeSource: func(b []byte) io.Reader {
			return io.MultiReader(bytes.NewReader(b[:len(b)-3]), iotest.ErrReader(io.ErrNoProgress))
		}, want: io.ErrNoProgress},
		{name: "second value is not ignored", makeSource: func(b []byte) io.Reader { return io.MultiReader(bytes.NewReader(b), strings.NewReader(`{}`)) }, want: core.ErrJSONContract},
		{name: "truncated document refused", makeSource: func(b []byte) io.Reader { return bytes.NewReader(b[:len(b)-1]) }, want: core.ErrJSONContract},
		{name: "empty document refused", makeSource: func([]byte) io.Reader { return strings.NewReader("") }, want: core.ErrJSONContract},
		{name: "root array refused", makeSource: func([]byte) io.Reader { return strings.NewReader(`[]`) }, want: core.ErrJSONContract},
		{name: "typed nil source refused before read", makeSource: func([]byte) io.Reader { var r *bytes.Reader; return r }, want: core.ErrStripeContract},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := checkoutFixtureBytes(t, checkoutFixture())
			got, err := DecodeCheckoutSession[checkoutMetadataFixture](t.Context(), checkoutDecodeRequest(t, tc.makeSource(body), uint64(len(body)+16)))
			if tc.want == nil {
				if err != nil || got.ID != "cs_test_owned" {
					t.Fatalf("DecodeCheckoutSession(one byte) = (%+v, %v), want full checkout", got, err)
				}
				return
			}
			if !errors.Is(err, core.ErrStripeContract) || !errors.Is(err, tc.want) || !checkoutSessionZero(got) {
				t.Fatalf("DecodeCheckoutSession(source) = (%+v, %v), want zero and cause %v", got, err, tc.want)
			}
		})
	}
}

func TestCheckoutSessionCancellation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		during bool
	}{
		{name: "cancel before first source read preserves cause"},
		{name: "cancel during source read preserves cause", during: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := checkoutFixtureBytes(t, checkoutFixture())
			ctx, cancel := context.WithCancelCause(t.Context())
			defer cancel(nil)
			var source io.Reader = bytes.NewReader(body)
			if tc.during {
				source = &checkoutCancelReader{source: source, cancel: cancel}
			} else {
				cancel(io.ErrClosedPipe)
			}
			got, err := DecodeCheckoutSession[checkoutMetadataFixture](ctx, checkoutDecodeRequest(t, source, uint64(len(body))))
			if !errors.Is(err, core.ErrStripeContract) || !errors.Is(err, context.Canceled) || !errors.Is(err, io.ErrClosedPipe) || !checkoutSessionZero(got) {
				t.Fatalf("DecodeCheckoutSession(cancel) = (%+v, %v), want zero and native/custom cancellation", got, err)
			}
		})
	}
}

type checkoutCancelReader struct {
	source io.Reader
	cancel context.CancelCauseFunc
}

func (r *checkoutCancelReader) Read(p []byte) (int, error) {
	n, err := r.source.Read(p)
	r.cancel(io.ErrClosedPipe)
	return n, err
}

func TestCheckoutSessionEnumsExhaustUnderlyingDomains(t *testing.T) {
	t.Parallel()
	for n := 0; n <= math.MaxUint8; n++ {
		status, payment, mode := CheckoutSessionStatus(n), CheckoutSessionPaymentStatus(n), CheckoutSessionMode(n)
		for _, got := range []struct {
			valid bool
			err   error
			text  string
			want  bool
		}{
			{status.IsValid(), status.Validate(), status.String(), n >= 1 && n <= 4},
			{payment.IsValid(), payment.Validate(), payment.String(), n >= 1 && n <= 3},
			{mode.IsValid(), mode.Validate(), mode.String(), n >= 1 && n <= 3},
		} {
			if got.valid != got.want || (got.err == nil) != got.want || !got.want && (!errors.Is(got.err, core.ErrStripeContract) || got.text != core.UnknownEnumDiagnostic) || got.want && got.text == core.UnknownEnumDiagnostic {
				t.Fatalf("enum %d observation = %+v, want closed typed domain", n, got)
			}
		}
	}
}

func TestCheckoutSessionAdmissionBoundaries(t *testing.T) {
	t.Parallel()
	t.Run("zero maximum refused", func(t *testing.T) {
		t.Parallel()
		got, err := DecodeCheckoutSession[checkoutMetadataFixture](t.Context(), CheckoutSessionDecodeRequest{Source: strings.NewReader("{}")})
		if !errors.Is(err, core.ErrStripeContract) || !checkoutSessionZero(got) {
			t.Fatalf("DecodeCheckoutSession(zero extent) = (%+v, %v), want zero typed refusal", got, err)
		}
	})
	for _, maximum := range []uint64{math.MaxInt64, math.MaxUint64} {
		t.Run("unrepresentable EOF probe "+strconv.FormatUint(maximum, 10), func(t *testing.T) {
			t.Parallel()
			got, err := DecodeCheckoutSession[checkoutMetadataFixture](t.Context(), checkoutDecodeRequest(t, strings.NewReader("{}"), maximum))
			if !errors.Is(err, core.ErrStripeContract) || !checkoutSessionZero(got) {
				t.Fatalf("DecodeCheckoutSession(extreme extent) = (%+v, %v), want zero typed refusal", got, err)
			}
		})
	}
	t.Run("nil context refused", func(t *testing.T) {
		t.Parallel()
		body := checkoutFixtureBytes(t, checkoutFixture())
		var absent context.Context
		got, err := DecodeCheckoutSession[checkoutMetadataFixture](absent, checkoutDecodeRequest(t, bytes.NewReader(body), uint64(len(body))))
		if !errors.Is(err, core.ErrStripeContract) || !errors.Is(err, core.ErrNilContext) || !checkoutSessionZero(got) {
			t.Fatalf("DecodeCheckoutSession(nil context) = (%+v, %v), want zero typed context refusal", got, err)
		}
	})
	cases := []struct {
		name    string
		suffix  string
		wantErr bool
	}{
		{name: "trailing whitespace admitted within extent", suffix: " \t\n"},
		{name: "late second scalar refused", suffix: " null", wantErr: true},
		{name: "late malformed value refused", suffix: " [", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := append(checkoutFixtureBytes(t, checkoutFixture()), tc.suffix...)
			got, err := DecodeCheckoutSession[checkoutMetadataFixture](t.Context(), checkoutDecodeRequest(t, bytes.NewReader(body), uint64(len(body))))
			if tc.wantErr {
				if !errors.Is(err, core.ErrStripeContract) || !checkoutSessionZero(got) {
					t.Fatalf("DecodeCheckoutSession(trailing refusal) = (%+v, %v), want zero typed refusal", got, err)
				}
				return
			}
			if err != nil || got.ID != "cs_test_owned" {
				t.Fatalf("DecodeCheckoutSession(trailing whitespace) = (%+v, %v), want admitted checkout", got, err)
			}
		})
	}
	t.Run("ignored foreign data preserves selected facts", func(t *testing.T) {
		t.Parallel()
		body := checkoutFixtureBytes(t, checkoutFixture())
		body = append(append([]byte(`{"foreign":{"values":[null,true,0,"value"]},`), body[1:]...), ' ')
		got, err := DecodeCheckoutSession[checkoutMetadataFixture](t.Context(), checkoutDecodeRequest(t, bytes.NewReader(body), uint64(len(body))))
		if err != nil || got.ID != "cs_test_owned" || got.AmountTotal == nil || *got.AmountTotal != 0 || got.Customer != nil {
			t.Fatalf("DecodeCheckoutSession(foreign fields) = (%+v, %v), want unchanged selected facts", got, err)
		}
	})
	t.Run("ignored deeply nested foreign value refuses at owned depth", func(t *testing.T) {
		t.Parallel()
		body := checkoutFixtureBytes(t, checkoutFixture())
		extra := `{"foreign":` + strings.Repeat("[", int(core.JSONNestingDepthMaximum)) + "0" + strings.Repeat("]", int(core.JSONNestingDepthMaximum)) + "," + string(body[1:])
		got, err := DecodeCheckoutSession[checkoutMetadataFixture](t.Context(), checkoutDecodeRequest(t, strings.NewReader(extra), uint64(len(extra))))
		if !errors.Is(err, core.ErrStripeContract) || !errors.Is(err, core.ErrJSONContract) || !checkoutSessionZero(got) {
			t.Fatalf("DecodeCheckoutSession(deep foreign value) = (%+v, %v), want zero typed depth refusal", got, err)
		}
	})
}

func TestStripeInventoryRecognizesGenericReceiverRoles(t *testing.T) {
	t.Parallel()
	file, err := parser.ParseFile(token.NewFileSet(), "fixture.go", `package p; type P[A,B any] struct{}; func (P[A,B]) stripeProtocolFact() {}; type Q[A any] struct{}; func (*Q[A]) stripeInternalFlow() {}; type Missing[A any] struct{}`, 0)
	if err != nil {
		t.Fatalf("ParseFile(generic inventory) error = %v, want nil", err)
	}
	structs, roles := make(map[string]struct{}), make(map[string]struct{})
	collectStripeStructRoles(file, structs, roles)
	got := missingStripeStructRoles(structs, roles)
	if len(got) != 1 || got[0] != "Missing" {
		t.Fatalf("generic inventory missing = %v, want only Missing", got)
	}
}
