package remoteexec

import (
	"errors"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Sentinel errors surfaced to the app layer for localized UI messaging.
var (
	// ErrUnknownExecutor means no supervised connection exists for an ID.
	ErrUnknownExecutor = errors.New("executor is not connected")
	// ErrTokenUnavailable means the vault could not supply the shared secret.
	ErrTokenUnavailable = errors.New("executor token is unavailable")
	// ErrOffline means the executor is currently unreachable.
	ErrOffline = errors.New("executor is offline")
	// ErrHostCallFailed wraps a failed reverse tunnel call.
	ErrHostCallFailed = errors.New("host service call failed")
)

// friendlyError strips gRPC noise from user-visible messages without
// exposing addresses, tokens, or payload contents.
func friendlyError(err error) string {
	if errors.Is(err, ErrUnknownExecutor) || errors.Is(err, ErrTokenUnavailable) {
		return err.Error()
	}
	if st, ok := status.FromError(err); ok {
		switch st.Code() {
		case codes.Unavailable:
			return "connection refused"
		case codes.Unauthenticated:
			return "token rejected by executor"
		case codes.DeadlineExceeded:
			return "connection timed out"
		default:
			return st.Message()
		}
	}
	return "connection failed"
}
