package auth

import (
	"context"
	pb "github.com/poponyas/GrpcChat/gen/grpc/chat_v1"
	pkgcontext "github.com/poponyas/GrpcChat/pkg/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"strings"
	"time"
)

func BearerToken(ctx context.Context) string {
	md, _ := metadata.FromIncomingContext(ctx)
	values := md.Get("authorization")
	if len(values) != 1 || !strings.HasPrefix(values[0], "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(values[0], "Bearer ")
}
func (s *Service) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, next grpc.UnaryHandler) (any, error) {
		if info.FullMethod == pb.AuthService_Register_FullMethodName || info.FullMethod == pb.AuthService_Login_FullMethodName {
			return next(ctx, req)
		}
		id, err := s.Authenticate(ctx, BearerToken(ctx))
		if err != nil {
			return nil, err
		}
		ctx = context.WithValue(ctx, pkgcontext.UserIDKey, id)
		switch in := req.(type) {
		case *pb.CreateProfileInput:
			return nil, status.Error(codes.Unimplemented, "use AuthService.Register to create an account")
		case *pb.DeleteProfileInput:
			return nil, status.Error(codes.Unimplemented, "account deletion is not supported yet")
		case *pb.UpdateProfileInput:
			if in.GetId() != id.String() {
				return nil, status.Error(codes.PermissionDenied, "can only update your own profile")
			}
			if in.Name != nil {
				return nil, status.Error(codes.InvalidArgument, "changing login name is not supported yet")
			}
		}
		return next(ctx, req)
	}
}

type authenticatedStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *authenticatedStream) Context() context.Context { return s.ctx }
func (s *Service) StreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv any, stream grpc.ServerStream, info *grpc.StreamServerInfo, next grpc.StreamHandler) error {
		id, err := s.Authenticate(stream.Context(), BearerToken(stream.Context()))
		if err != nil {
			return err
		}
		ctx, cancel := context.WithCancel(context.WithValue(stream.Context(), pkgcontext.UserIDKey, id))
		defer cancel()
		// Revoke idle subscriptions as well as unary calls after logout/expiry.
		go func() {
			timer := time.NewTicker(15 * time.Second)
			defer timer.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-timer.C:
					check, done := context.WithTimeout(ctx, 5*time.Second)
					_, err := s.Authenticate(check, BearerToken(stream.Context()))
					done()
					if err != nil {
						cancel()
						return
					}
				}
			}
		}()
		return next(srv, &authenticatedStream{ServerStream: stream, ctx: ctx})
	}
}
