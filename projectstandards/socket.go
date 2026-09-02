package projectstandards

import (
	"context"
	"errors"
	"net/http"

	"github.com/deliri/primitive/v2026/core"
	"github.com/deliri/primitive/v2026/exchange"
)

const (
	SocketRequestMaximumBytes = 16 * 1024
)

// QueryKind is the closed Project standards read domain.
type QueryKind uint8

const (
	QueryKindUnknown QueryKind = iota
	QueryProject
	QueryPackage
	queryKindLimit
)

func queryKindLabels() []string { return []string{"", "project", "package"} }

func (k QueryKind) Validate() error {
	return validateEnum(uint8(k), queryKindLabels(), "project standards query kind is invalid")
}
func (k QueryKind) IsValid() bool  { return k.Validate() == nil }
func (k QueryKind) String() string { return enumString(uint8(k), queryKindLabels()) }

func (k QueryKind) MarshalJSON() ([]byte, error) {
	return marshalEnum(uint8(k), queryKindLabels(), "project standards query kind is invalid")
}

func (k *QueryKind) UnmarshalJSON(data []byte) error {
	if k == nil {
		return jsonError(errors.New("nil project standards query kind receiver"))
	}
	value, err := unmarshalEnum(data, queryKindLabels(), "project standards query kind is invalid")
	if err == nil {
		*k = QueryKind(value)
	}
	return err
}

// Query selects one project index or one package snapshot at an exact source
// revision. Package is present only for QueryPackage.
type Query struct {
	Package       *SourcePath      `json:"package,omitempty"`
	Subject       SubjectIdentity  `json:"subject"`
	SchemaVersion uint16           `json:"schema_version"`
	Revision      core.BuildCommit `json:"revision"`
	Kind          QueryKind        `json:"kind"`
}

func (q Query) Validate() error {
	if q.SchemaVersion != SchemaVersion {
		return contractError(errors.New("project standards query schema version is unsupported"))
	}
	if err := contractJoin(q.Subject.Validate(), q.Revision.Validate(), q.Kind.Validate()); err != nil {
		return err
	}
	if q.Kind == QueryProject && q.Package == nil {
		return nil
	}
	if q.Kind == QueryPackage && q.Package != nil {
		return q.Package.Validate()
	}
	return conflictError(errors.New("project standards query payload differs from kind"))
}

type queryWire Query

func (q Query) MarshalJSON() ([]byte, error) {
	if err := q.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(queryWire(q))
	if err != nil {
		return nil, jsonError(err)
	}
	return encoded, nil
}

func (q *Query) UnmarshalJSON(data []byte) error {
	if q == nil {
		return jsonError(errors.New("nil project standards query receiver"))
	}
	wire, err := core.DecodeStrictJSONStructure[queryWire](data, aboutJSONLimits())
	if err != nil {
		return jsonError(err)
	}
	candidate := Query(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*q = candidate
	return nil
}

// Response carries exactly one project or package projection matching Query.
type Response struct {
	Project       *Project         `json:"project,omitempty"`
	Package       *PackageSnapshot `json:"package,omitempty"`
	Query         Query            `json:"query"`
	SchemaVersion uint16           `json:"schema_version"`
}

func (r Response) Validate() error {
	if r.SchemaVersion != SchemaVersion {
		return contractError(errors.New("project standards response schema version is unsupported"))
	}
	if err := r.Query.Validate(); err != nil {
		return err
	}
	if (r.Project != nil) == (r.Package != nil) {
		return contractError(errors.New("project standards response must carry exactly one projection"))
	}
	if r.Project != nil {
		return r.validateProject()
	}
	return r.validatePackage()
}

func (r Response) validateProject() error {
	if r.Query.Kind != QueryProject {
		return conflictError(errors.New("project standards project response differs from query kind"))
	}
	if err := r.Project.Validate(); err != nil {
		return err
	}
	if !sameSubject(r.Project.Subject, r.Query.Subject) || r.Project.Revision != r.Query.Revision {
		return conflictError(errors.New("project standards project response differs from query source"))
	}
	return nil
}

func (r Response) validatePackage() error {
	if r.Query.Kind != QueryPackage || r.Query.Package == nil {
		return conflictError(errors.New("project standards package response differs from query kind"))
	}
	if err := r.Package.Validate(); err != nil {
		return err
	}
	if !sameSubject(r.Package.Package.Subject, r.Query.Subject) || r.Package.Package.Revision != r.Query.Revision || r.Package.Package.Knowledge.Path != *r.Query.Package {
		return conflictError(errors.New("project standards package response differs from query source"))
	}
	return nil
}

type responseWire Response

func (r Response) MarshalJSON() ([]byte, error) {
	if err := r.Validate(); err != nil {
		return nil, jsonError(err)
	}
	encoded, err := core.MarshalCanonicalJSONDocument(responseWire(r))
	if err != nil {
		return nil, jsonError(err)
	}
	return encoded, nil
}

func (r *Response) UnmarshalJSON(data []byte) error {
	if r == nil {
		return jsonError(errors.New("nil project standards response receiver"))
	}
	wire, err := core.DecodeStrictJSONStructure[responseWire](data, aboutJSONLimits())
	if err != nil {
		return jsonError(err)
	}
	candidate := Response(wire)
	if err := candidate.Validate(); err != nil {
		return jsonError(err)
	}
	*r = candidate
	return nil
}

func aboutJSONLimits() core.StrictJSONLimits {
	return core.DefaultStrictJSONLimits()
}

// Repository is the narrow product-owned persistence capability the About
// server consumes. Primitive owns no Firestore, Postgres, or SQLite policy.
type Repository interface {
	Project(context.Context, SubjectIdentity, core.BuildCommit) (Project, error)
	Package(context.Context, SubjectIdentity, core.BuildCommit, SourcePath) (PackageSnapshot, error)
}

// Service resolves validated queries against one product-owned repository.
type Service struct{ repository Repository }

// NewService constructs one Project standards query service.
func NewService(repository Repository) (Service, error) {
	if repository == nil {
		return Service{}, contractError(errors.New("project standards repository is nil"))
	}
	return Service{repository: repository}, nil
}

// Resolve loads exactly the projection named by query and revalidates it
// before it can cross the package boundary.
func (s Service) Resolve(ctx context.Context, query Query) (Response, error) {
	if s.repository == nil {
		return Response{}, contractError(errors.New("project standards service is unconstructed"))
	}
	if err := query.Validate(); err != nil {
		return Response{}, err
	}
	if err := contextTerminal(ctx); err != nil {
		return Response{}, err
	}
	if query.Kind == QueryProject {
		project, err := s.repository.Project(ctx, query.Subject, query.Revision)
		if err != nil {
			return Response{}, err
		}
		response := Response{SchemaVersion: SchemaVersion, Query: query, Project: &project}
		return response, response.Validate()
	}
	packageSnapshot, err := s.repository.Package(ctx, query.Subject, query.Revision, *query.Package)
	if err != nil {
		return Response{}, err
	}
	response := Response{SchemaVersion: SchemaVersion, Query: query, Package: &packageSnapshot}
	return response, response.Validate()
}

func contextTerminal(ctx context.Context) error {
	if ctx == nil {
		return contractError(errors.New("project standards context is nil"))
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return nil
}

// SocketContract constructs the paired read route without owning its path.
func SocketContract(path exchange.SocketRoutePath) (exchange.JSONSocketContract, error) {
	requestLimit, err := core.NewByteCount(SocketRequestMaximumBytes)
	if err != nil {
		return exchange.JSONSocketContract{}, contractError(err)
	}
	responseLimit, err := core.NewByteCount(core.JSONDocumentMaximumBytes)
	if err != nil {
		return exchange.JSONSocketContract{}, contractError(err)
	}
	contract := exchange.JSONSocketContract{
		Path:             path,
		Route:            exchange.RouteSemantics{Method: exchange.MethodPost, Replay: exchange.ReplaySingleAttempt},
		RequestBodyLimit: requestLimit, ResponseBodyLimit: responseLimit, SuccessStatus: core.HTTPStatusOK(),
	}
	if err := contract.Validate(); err != nil {
		return exchange.JSONSocketContract{}, transportError(err)
	}
	return contract, nil
}

// Client owns one paired Project standards socket.
type Client struct{ socket exchange.ClientSocket }

// NewClient constructs an Project standards client from an already validated Exchange
// configuration whose contract must equal SocketContract for its path.
func NewClient(configuration exchange.ClientSocketConfiguration) (Client, error) {
	want, err := SocketContract(configuration.Contract.Path)
	if err != nil {
		return Client{}, err
	}
	if !sameSocketContract(configuration.Contract, want) {
		return Client{}, transportError(errors.New("project standards client socket contract differs from Project standards contract"))
	}
	socket, err := exchange.NewClientSocket(configuration)
	if err != nil {
		return Client{}, transportError(err)
	}
	return Client{socket: socket}, nil
}

// Fetch sends one typed Project standards query through Exchange.
func (c Client) Fetch(ctx context.Context, query Query) (FetchResult, error) {
	if err := c.socket.Validate(); err != nil {
		return FetchResult{}, transportError(err)
	}
	response, err := exchange.SendSocketJSON[Query, Response](ctx, c.socket, query)
	if err != nil {
		return FetchResult{}, transportError(err)
	}
	result := FetchResult{Response: response.Body, Metadata: response.Metadata}
	return result, result.Validate()
}

// FetchResult preserves the exact Exchange execution metadata beside the
// validated Project standards response.
type FetchResult struct {
	Metadata exchange.ResponseMetadata `json:"metadata"`
	Response Response                  `json:"response"`
}

func (r FetchResult) Validate() error {
	return contractJoin(r.Response.Validate(), r.Metadata.Validate())
}

// Server owns one paired Project standards socket.
type Server struct {
	service Service
	socket  exchange.ServerSocket
}

// NewServer constructs the paired server over one product repository.
func NewServer(path exchange.SocketRoutePath, repository Repository) (Server, error) {
	contract, err := SocketContract(path)
	if err != nil {
		return Server{}, err
	}
	socket, err := exchange.NewServerSocket(contract)
	if err != nil {
		return Server{}, transportError(err)
	}
	service, err := NewService(repository)
	if err != nil {
		return Server{}, err
	}
	return Server{socket: socket, service: service}, nil
}

// Serve decodes, resolves, and emits one request through the real paired
// Exchange server path. HTTP status policy for failures remains with the
// product handler that calls Serve.
func (s Server) Serve(writer http.ResponseWriter, request *http.Request) error {
	if s.service.repository == nil {
		return contractError(errors.New("project standards server is unconstructed"))
	}
	call, err := exchange.NewSocketServerCall(writer, request)
	if err != nil {
		return transportError(err)
	}
	received, err := exchange.ReceiveSocketJSON[Query, *Query](s.socket, call)
	if err != nil {
		return transportError(err)
	}
	response, err := s.service.Resolve(request.Context(), *received.Body)
	if err != nil {
		return err
	}
	if err := exchange.WriteSocketJSON(s.socket, call, response); err != nil {
		return transportError(err)
	}
	return nil
}

func sameSocketContract(left, right exchange.JSONSocketContract) bool {
	return left.Path.String() == right.Path.String() && left.Route == right.Route && left.RequestBodyLimit == right.RequestBodyLimit && left.ResponseBodyLimit == right.ResponseBodyLimit && left.SuccessStatus == right.SuccessStatus
}
