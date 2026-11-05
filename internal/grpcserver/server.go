// Package grpcserver exposes the internal gRPC control plane.
//
// It intentionally contains no browser-facing API: public traffic remains on
// HTTPS/JSON, while this listener is for private service networking, load
// balancers, and future independently deployable workers.
package grpcserver

import (
	"context"
	"log/slog"
	"runtime/debug"
	"time"

	"mitte/internal/store"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
)

// New creates the private gRPC server. Limits are deliberately conservative:
// the service is a control plane, not a bulk-data transport. Domain services
// added here should define their own versioned protobuf API under api/.
func New(repository *store.Store, logger *slog.Logger) *grpc.Server {
	if logger == nil {
		logger = slog.Default()
	}
	server := grpc.NewServer(
		grpc.MaxRecvMsgSize(64<<10),
		grpc.MaxSendMsgSize(64<<10),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle:     5 * time.Minute,
			MaxConnectionAge:      30 * time.Minute,
			MaxConnectionAgeGrace: 30 * time.Second,
			Time:                  2 * time.Hour,
			Timeout:               20 * time.Second,
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             30 * time.Second,
			PermitWithoutStream: false,
		}),
		grpc.ChainUnaryInterceptor(recoveryInterceptor(logger), loggingInterceptor(logger)),
	)
	grpc_health_v1.RegisterHealthServer(server, readinessServer{store: repository})
	return server
}

type readinessServer struct {
	grpc_health_v1.UnimplementedHealthServer
	store *store.Store
}

func (s readinessServer) Check(ctx context.Context, _ *grpc_health_v1.HealthCheckRequest) (*grpc_health_v1.HealthCheckResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	if err := s.store.Ping(ctx); err != nil {
		return nil, status.Error(codes.Unavailable, "database is unavailable")
	}
	return &grpc_health_v1.HealthCheckResponse{Status: grpc_health_v1.HealthCheckResponse_SERVING}, nil
}

func recoveryInterceptor(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (response any, err error) {
		defer func() {
			if recovered := recover(); recovered != nil {
				logger.Error("grpc panic recovered", "method", info.FullMethod, "panic", recovered, "stack", string(debug.Stack()))
				err = status.Error(codes.Internal, "internal server error")
			}
		}()
		return handler(ctx, request)
	}
}

func loggingInterceptor(logger *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, request any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		started := time.Now()
		response, err := handler(ctx, request)
		logger.Info("grpc request completed", "method", info.FullMethod, "code", status.Code(err).String(), "duration_ms", time.Since(started).Milliseconds())
		return response, err
	}
}
