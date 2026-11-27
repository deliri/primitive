package gmail

import (
	"context"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

// Operation is one typed server endpoint. Its Validate and Execute methods
// are the contract consumed by a Bridge client.
type Operation[Request core.Validatable, Response core.Validatable] struct {
	run func(context.Context, Request) (Response, error)
}

func (o Operation[Request, Response]) Validate() error {
	if o.run == nil {
		return core.ErrGmailContract
	}
	return nil
}

func (o Operation[Request, Response]) Execute(ctx context.Context, request Request) (Response, error) {
	if err := o.Validate(); err != nil {
		var zero Response
		return zero, err
	}
	return o.run(ctx, request)
}

// Server binds the SDK client to typed endpoints without exposing HTTP
// routing, secret acquisition or Gmail wire structs to Bridge.
type Server struct{ client Client }

func NewServer(network exchange.Client) (Server, error) {
	client, err := NewClient(network)
	return Server{client: client}, err
}

func (s Server) Validate() error { return s.client.Validate() }

func (s Server) Refresh() Operation[RefreshRequest, Access] {
	return bindOperation(s, s.client.Refresh)
}
func (s Server) EnrollWorkspaceGrant() Operation[ConsentRequest, RefreshToken] {
	return bindOperation(s, s.client.EnrollWorkspaceGrant)
}
func (s Server) Delegate() Operation[DelegatedRequest, Access] {
	return bindOperation(s, s.client.Delegate)
}
func (s Server) PrepareMessage() Operation[MessagePrepareRequest, PreparedMessage] {
	return bindOperation(s, s.client.PrepareMessage)
}
func (s Server) SendMessage() Operation[MessageSendRequest, SendAcceptance] {
	return bindOperation(s, s.client.SendMessage)
}
func (s Server) SearchRecentReply() Operation[RecentReplyRequest, RecentReply] {
	return bindOperation(s, s.client.SearchRecentReply)
}
func (s Server) ReadSentEvidence() Operation[SentEvidenceRequest, SentEvidence] {
	return bindOperation(s, s.client.ReadSentEvidence)
}
func (s Server) ReadHistoryCursor() Operation[HistoryCursorRequest, HistoryID] {
	return bindOperation(s, s.client.ReadHistoryCursor)
}
func (s Server) ListHistory() Operation[HistoryListRequest, HistoryPage] {
	return bindOperation(s, s.client.ListHistory)
}
func (s Server) ListThreads() Operation[ThreadListRequest, ThreadPage] {
	return bindOperation(s, s.client.ListThreads)
}
func (s Server) ReadThread() Operation[ThreadReadRequest, Thread] {
	return bindOperation(s, s.client.ReadThread)
}
func (s Server) ReadMessageMetadata() Operation[MessageReadRequest, MessageMetadata] {
	return bindOperation(s, s.client.ReadMessageMetadata)
}
func (s Server) ReadMessage() Operation[MessageReadRequest, Message] {
	return bindOperation(s, s.client.ReadMessage)
}

func bindOperation[Request core.Validatable, Response core.Validatable](s Server, run func(context.Context, Request) (Response, error)) Operation[Request, Response] {
	if err := s.Validate(); err != nil {
		return Operation[Request, Response]{}
	}
	return Operation[Request, Response]{run: run}
}
