package kittygrpc

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	"github.com/go-kit/kit/endpoint"
	"github.com/go-kit/kit/log"
)

// Server wraps an endpoint and implements grpc.Handler.
type Server struct {
	e         endpoint.Endpoint
	before    []ServerRequestFunc
	after     []ServerResponseFunc
	finalizer []ServerFinalizerFunc
	logger    log.Logger
}

// NewServer constructs a new server, which implements wraps the provided
// endpoint and implements the Handler interface. Consumers should write
// bindings that adapt the concrete gRPC methods from their compiled protobuf
// definitions to individual handlers. Request and response objects are from the
// caller business domain, not gRPC request and reply types.
func NewServer(
	e endpoint.Endpoint,
	options ...ServerOption,
) *Server {
	s := &Server{
		e:      e,
		logger: log.NewNopLogger(),
	}
	for _, option := range options {
		option(s)
	}
	return s
}

// ServerOption sets an optional parameter for servers.
type ServerOption func(*Server)

// ServerBefore functions are executed on the HTTP request object before the
// request is decoded.
func ServerBefore(before ...ServerRequestFunc) ServerOption {
	return func(s *Server) { s.before = append(s.before, before...) }
}

// ServerAfter functions are executed on the HTTP response writer after the
// endpoint is invoked, but before anything is written to the client.
func ServerAfter(after ...ServerResponseFunc) ServerOption {
	return func(s *Server) { s.after = append(s.after, after...) }
}

// ServerErrorLogger is used to log non-terminal errors. By default, no errors
// are logged.
func ServerErrorLogger(logger log.Logger) ServerOption {
	return func(s *Server) { s.logger = logger }
}

// ServerFinalizer is executed at the end of every gRPC request.
// By default, no finalizer is registered.
func ServerFinalizer(f ...ServerFinalizerFunc) ServerOption {
	return func(s *Server) { s.finalizer = append(s.finalizer, f...) }
}

// ServeGRPC implements the Handler interface.
func (s Server) Handle(ctx context.Context, req interface{}) (c context.Context, resp interface{}, err error) {
	// Retrieve gRPC metadata.
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		md = metadata.MD{}
	}

	if len(s.finalizer) > 0 {
		defer func() {
			for _, f := range s.finalizer {
				f(ctx, err)
			}
		}()
	}

	for _, f := range s.before {
		ctx = f(ctx, md)
	}

	response, err := s.e(ctx, req)
	if err != nil {
		s.logger.Log("err", err)
		return ctx, nil, err
	}

	var mdHeader, mdTrailer metadata.MD
	for _, f := range s.after {
		ctx = f(ctx, &mdHeader, &mdTrailer)
	}

	if len(mdHeader) > 0 {
		if err = grpc.SendHeader(ctx, mdHeader); err != nil {
			s.logger.Log("err", err)
			return ctx, nil, err
		}
	}

	if len(mdTrailer) > 0 {
		if err = grpc.SetTrailer(ctx, mdTrailer); err != nil {
			s.logger.Log("err", err)
			return ctx, nil, err
		}
	}

	return ctx, response, nil
}

// ServerFinalizerFunc can be used to perform work at the end of an gRPC
// request, after the response has been written to the client.
type ServerFinalizerFunc func(ctx context.Context, err error)

// Interceptor is a grpc UnaryInterceptor that injects the method name into
// context so it can be consumed by Go kit gRPC middlewares. The Interceptor
// typically is added at creation time of the grpc-go server.
// Like this: `grpc.NewServer(grpc.UnaryInterceptor(kitgrpc.Interceptor))`
func Interceptor(
	ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler,
) (resp interface{}, err error) {
	ctx = context.WithValue(ctx, ContextKeyRequestMethod, info.FullMethod)
	return handler(ctx, req)
}
