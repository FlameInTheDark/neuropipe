package remoteexec

import (
	"context"
	"crypto/subtle"
	"crypto/tls"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const metadataAuthorization = "authorization"

// errUnauthenticated is returned for every request without a valid bearer
// token. The message deliberately does not distinguish missing from invalid.
var errUnauthenticated = status.Error(codes.Unauthenticated, "a valid executor token is required")

// VerifyMetadata reports whether the gRPC metadata carries exactly the
// configured bearer token. Comparison is constant-time.
func VerifyMetadata(md metadata.MD, token string) bool {
	if md == nil || token == "" {
		return false
	}
	for _, header := range md.Get(metadataAuthorization) {
		value, ok := strings.CutPrefix(header, "Bearer ")
		if !ok {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(value), []byte(token)) == 1 {
			return true
		}
	}
	return false
}

// UnaryAuthInterceptor guards every unary RPC with the shared token.
func UnaryAuthInterceptor(token string) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		if !ok || !VerifyMetadata(md, token) {
			return nil, errUnauthenticated
		}
		return handler(ctx, req)
	}
}

// StreamAuthInterceptor guards every streaming RPC with the shared token.
func StreamAuthInterceptor(token string) grpc.StreamServerInterceptor {
	return func(srv any, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		md, ok := metadata.FromIncomingContext(ss.Context())
		if !ok || !VerifyMetadata(md, token) {
			return errUnauthenticated
		}
		return handler(srv, ss)
	}
}

// tokenCredential injects the shared bearer token into every outgoing call.
// Transport security is optional: plaintext h2c is accepted so a LAN or VPN
// deployment works without certificates, while TLS is enabled per executor.
type tokenCredential struct{ token string }

func (c tokenCredential) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{metadataAuthorization: "Bearer " + c.token}, nil
}

func (c tokenCredential) RequireTransportSecurity() bool { return false }

// transportCredentials selects plaintext or TLS transport for one connection.
func transportCredentials(useTLS bool, serverName string) credentials.TransportCredentials {
	if !useTLS {
		return insecure.NewCredentials()
	}
	return credentials.NewTLS(&tls.Config{ServerName: serverName, MinVersion: tls.VersionTLS12})
}

// dialOptions assembles the client options for one executor connection: the
// transport credentials plus the per-RPC bearer token.
func dialOptions(token string, useTLS bool, serverName string) []grpc.DialOption {
	return []grpc.DialOption{
		grpc.WithTransportCredentials(transportCredentials(useTLS, serverName)),
		grpc.WithPerRPCCredentials(tokenCredential{token: token}),
	}
}
