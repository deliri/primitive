package searchconsole

import (
	"context"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

// Server supplies typed Search Console operations to Bridge.
type Server struct{ client Client }

func NewServer(network exchange.Client, source AccessSource) (Server, error) {
	client, err := NewClient(network, source)
	return Server{client: client}, err
}

func (s Server) Query() Operation[QueryRequest, QueryPage] {
	return Operation[QueryRequest, QueryPage]{client: s.client, run: s.client.Query}
}

func (s Server) GetSite() Operation[SiteRequest, Site] {
	return Operation[SiteRequest, Site]{client: s.client, run: s.client.GetSite}
}

type Operation[Request core.Validatable, Response core.Validatable] struct {
	client Client
	run    func(context.Context, Request) (Response, error)
}

func (o Operation[Request, Response]) Validate() error {
	if o.client.source == nil || o.run == nil {
		return core.ErrSearchConsoleContract
	}
	return o.client.network.Validate()
}

func (o Operation[Request, Response]) Execute(ctx context.Context, request Request) (Response, error) {
	if err := o.Validate(); err != nil {
		var zero Response
		return zero, err
	}
	return o.run(ctx, request)
}
