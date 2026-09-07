package awsidentity

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/temporal"
)

type awsInvalidIngress uint8

const (
	awsInvalidClient awsInvalidIngress = iota
	awsInvalidRequest
	awsInvalidAudience
	awsInvalidPolicy
	awsCancelledContext
	awsExpiredContext
	awsNilContext
)

func TestAWSAcquireInvalidIngressCannotPerformEffect(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		kind    awsInvalidIngress
		wantErr error
	}{
		{"unset client", awsInvalidClient, core.ErrAWSIdentityContract},
		{"unset request", awsInvalidRequest, core.ErrAWSIdentityContract},
		{"request audience changed after construction", awsInvalidAudience, core.ErrAWSIdentityContract},
		{"request policy unset after construction", awsInvalidPolicy, core.ErrAWSIdentityContract},
		{"pre cancelled context", awsCancelledContext, context.Canceled},
		{"expired temporal budget", awsExpiredContext, context.DeadlineExceeded},
		{"nil context", awsNilContext, core.ErrAWSIdentityContract},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			body := &awsObservedBody{reader: bytes.NewReader(awsProviderBytes(t, awsProviderDocument(awsTestBearer)))}
			transport := &awsResponseTransport{body: body, status: http.StatusOK, length: -1}
			client := awsClient(t, transport)
			request := awsRequest(t)
			ctx := t.Context()
			switch tc.kind {
			case awsInvalidClient:
				client = Client{}
			case awsInvalidRequest:
				request = Request{}
			case awsInvalidAudience:
				request.audience = Audience{}
			case awsInvalidPolicy:
				request.policy = Policy{}
			case awsCancelledContext:
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			case awsExpiredContext:
				var cancel context.CancelFunc
				var err error
				ctx, cancel, err = temporal.WithTimeout(temporal.TimeoutRequest{Parent: ctx, Duration: temporal.Duration{}})
				if err != nil {
					t.Fatalf("temporal zero budget fixture error = %v, want nil", err)
				}
				defer cancel()
			case awsNilContext:
				ctx = nil
			default:
				t.Fatalf("ingress mutation = %d, want declared boundary", tc.kind)
			}
			got, err := Acquire(ctx, client, request)
			if got != (Token{}) || !errors.Is(err, tc.wantErr) || !errors.Is(err, core.ErrAWSIdentityContract) {
				t.Fatalf("Acquire invalid ingress = (%v,%v), want zero AWS and %v refusal", got, err, tc.wantErr)
			}
			if transport.calls != 0 || body.bytes != 0 || body.closes != 0 {
				t.Fatalf("invalid ingress effects = %d/%d/%d, want none", transport.calls, body.bytes, body.closes)
			}
		})
	}
}
