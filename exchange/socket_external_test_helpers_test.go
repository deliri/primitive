package exchange_test

import (
	"net/http"
	"net/http/httptest"

	"github.com/deliri/primitive/v2026/exchange"
)

type socketTestScope interface {
	Helper()
	Fatalf(string, ...any)
}

func socketServerCall(t socketTestScope, request *http.Request) exchange.SocketServerCall {
	t.Helper()
	if request == nil {
		return exchange.SocketServerCall{}
	}
	return socketServerCallFrom(t, httptest.NewRecorder(), request)
}

func socketServerCallFrom(t socketTestScope, writer http.ResponseWriter, request *http.Request) exchange.SocketServerCall {
	t.Helper()
	if writer == nil || request == nil {
		return exchange.SocketServerCall{}
	}
	call, err := exchange.NewSocketServerCall(writer, request)
	if err != nil {
		t.Fatalf("NewSocketServerCall() error = %v, want nil", err)
	}
	return call
}
