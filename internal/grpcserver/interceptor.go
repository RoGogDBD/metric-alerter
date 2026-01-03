package grpcserver

import (
	"context"
	"net"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

// IPSubnetInterceptor проверяет IP-адрес агента из метаданных.
func IPSubnetInterceptor(trustedSubnet *net.IPNet) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if trustedSubnet == nil {
			return handler(ctx, req)
		}

		md, ok := metadata.FromIncomingContext(ctx)
		if !ok {
			return nil, status.Error(codes.PermissionDenied, "missing metadata")
		}

		values := md.Get("x-real-ip")
		if len(values) == 0 {
			return nil, status.Error(codes.PermissionDenied, "missing x-real-ip")
		}

		ipString := strings.TrimSpace(values[0])
		ip := net.ParseIP(ipString)
		if ip == nil || !trustedSubnet.Contains(ip) {
			return nil, status.Error(codes.PermissionDenied, "ip not allowed")
		}

		return handler(ctx, req)
	}
}
