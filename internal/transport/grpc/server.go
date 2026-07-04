package grpc

import (
	"errors"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/Harshmaury/verion/internal/identity"
	verionv1 "github.com/Harshmaury/verion/api/gen/go/verion/v1"
)

// Server holds the gRPC server and all registered service handlers.
type Server struct {
	grpcServer *grpc.Server
}

// New creates and registers all gRPC service handlers.
func New(
	identitySvc identity.IdentityService,
	tenantSvc identity.TenantService,
	keySvc identity.KeyService,
) *Server {
	s := grpc.NewServer()
	verionv1.RegisterIdentityServiceServer(s, newIdentityServer(identitySvc, tenantSvc))
	verionv1.RegisterKeyServiceServer(s, newKeyServer(keySvc))
	return &Server{grpcServer: s}
}

// GRPCServer returns the underlying *grpc.Server for use with net.Listener.
func (s *Server) GRPCServer() *grpc.Server { return s.grpcServer }

// toGRPCError maps domain errors to gRPC status codes.
// Must be called on every error returned from a service method.
func toGRPCError(err error) error {
	switch {
	case errors.Is(err, identity.ErrNotFound),
		errors.Is(err, identity.ErrKeyNotFound),
		errors.Is(err, identity.ErrTenantNotFound),
		errors.Is(err, identity.ErrCredentialNotFound),
		errors.Is(err, identity.ErrRecoveryNotFound):
		return status.Error(codes.NotFound, err.Error())

	case errors.Is(err, identity.ErrAlreadyExists),
		errors.Is(err, identity.ErrHandleTaken):
		return status.Error(codes.AlreadyExists, err.Error())

	case errors.Is(err, identity.ErrInvalidInput),
		errors.Is(err, identity.ErrInvalidHandle):
		return status.Error(codes.InvalidArgument, err.Error())

	case errors.Is(err, identity.ErrVersionConflict):
		return status.Error(codes.Aborted, err.Error())

	case errors.Is(err, identity.ErrIdentityInactive),
		errors.Is(err, identity.ErrIdentityTerminal),
		errors.Is(err, identity.ErrTenantInactive),
		errors.Is(err, identity.ErrKeyNotUsable),
		errors.Is(err, identity.ErrKeyCompromised),
		errors.Is(err, identity.ErrCredentialInactive),
		errors.Is(err, identity.ErrCredentialConsumed),
		errors.Is(err, identity.ErrRecoveryNotUsable),
		errors.Is(err, identity.ErrNoPrimaryKey):
		return status.Error(codes.FailedPrecondition, err.Error())

	case errors.Is(err, identity.ErrUnauthorized):
		return status.Error(codes.PermissionDenied, err.Error())

	default:
		// Never expose internal error details to callers.
		return status.Error(codes.Internal, "internal error")
	}
}
