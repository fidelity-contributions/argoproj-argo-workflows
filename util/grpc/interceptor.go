package grpc

import (
	"context"
	"runtime/debug"
	"strings"

	wfv1 "github.com/argoproj/argo-workflows/v3/pkg/apis/workflow/v1alpha1"

	log "github.com/sirupsen/logrus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"

	limiter "github.com/sethvargo/go-limiter"
)

// PanicLoggerUnaryServerInterceptor returns a new unary server interceptor for recovering from panics and returning error
func PanicLoggerUnaryServerInterceptor(log *log.Entry) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (_ interface{}, err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Errorf("Recovered from panic: %+v\n%s", r, debug.Stack())
				err = status.Errorf(codes.Internal, "%s", r)
			}
		}()
		return handler(ctx, req)
	}
}

// PanicLoggerStreamServerInterceptor returns a new streaming server interceptor for recovering from panics and returning error
func PanicLoggerStreamServerInterceptor(log *log.Entry) grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) (err error) {
		defer func() {
			if r := recover(); r != nil {
				log.Errorf("Recovered from panic: %+v\n%s", r, debug.Stack())
				err = status.Errorf(codes.Internal, "%s", r)
			}
		}()
		return handler(srv, stream)
	}
}

const (
	ArgoVersionHeader = "argo-version"
)

var (
	LastSeenServerVersion                  string
	ErrorTranslationUnaryServerInterceptor = func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		resp, err = handler(ctx, req)
		return resp, TranslateError(err)
	}
	ErrorTranslationStreamServerInterceptor = func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		return TranslateError(handler(srv, ss))
	}
)

// SetVersionHeaderUnaryServerInterceptor returns a new unary server interceptor that sets the argo-version header
func SetVersionHeaderUnaryServerInterceptor(version wfv1.Version) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		m, origErr := handler(ctx, req)
		if origErr == nil {
			// Don't set header if there was an error because attackers could use it to find vulnerable Argo servers
			err := grpc.SetHeader(ctx, metadata.Pairs(ArgoVersionHeader, version.Version))
			if err != nil {
				log.Warnf("Failed to set header '%s': %s", ArgoVersionHeader, err)
			}
		}
		return m, origErr
	}
}

// SetVersionHeaderStreamServerInterceptor returns a new stream server interceptor that sets the argo-version header
func SetVersionHeaderStreamServerInterceptor(version wfv1.Version) grpc.StreamServerInterceptor {
	return func(srv interface{}, ss grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		origErr := handler(srv, ss)
		if origErr == nil {
			// Don't set header if there was an error because attackers could use it to find vulnerable Argo servers
			err := ss.SetHeader(metadata.Pairs(ArgoVersionHeader, version.Version))
			if err != nil {
				log.Warnf("Failed to set header '%s': %s", ArgoVersionHeader, err)
			}
		}
		return origErr
	}
}

// GetVersionHeaderClientUnaryInterceptor returns a new unary client interceptor that extracts the argo-version from the response and sets the global variable LastSeenServerVersion
func GetVersionHeaderClientUnaryInterceptor(ctx context.Context, method string, req, reply interface{}, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
	var headers metadata.MD
	err := invoker(ctx, method, req, reply, cc, append(opts, grpc.Header(&headers))...)
	if err == nil && headers != nil && headers.Get(ArgoVersionHeader) != nil {
		LastSeenServerVersion = headers.Get(ArgoVersionHeader)[0]
	}
	return err
}

// RatelimitUnaryServerInterceptor returns a new unary server interceptor that performs request rate limiting.
func RatelimitUnaryServerInterceptor(ratelimiter limiter.Store) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp interface{}, err error) {
		ip := getClientIP(ctx)
		_, _, _, ok, err := ratelimiter.Take(ctx, ip)
		if err != nil {
			log.Warnf("Internal Server Error: %s", err)
			return nil, status.Errorf(codes.Internal, "%s: grpc_ratelimit middleware internal error", info.FullMethod)
		}
		if !ok {
			return nil, status.Errorf(codes.ResourceExhausted, "%s is rejected by grpc_ratelimit middleware, please retry later.", info.FullMethod)
		}
		return handler(ctx, req)
	}
}

// RatelimitStreamServerInterceptor returns a new stream server interceptor that performs rate limiting on the request.
func RatelimitStreamServerInterceptor(ratelimiter limiter.Store) grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		ctx := stream.Context()
		ip := getClientIP(ctx)
		_, _, _, ok, err := ratelimiter.Take(ctx, ip)
		if err != nil {
			log.Warnf("Internal Server Error: %s", err)
			return status.Errorf(codes.Internal, "%s: grpc_ratelimit middleware internal error", info.FullMethod)
		}
		if !ok {
			return status.Errorf(codes.ResourceExhausted, "%s is rejected by grpc_ratelimit middleware, please retry later.", info.FullMethod)
		}
		return handler(srv, stream)
	}
}

// GetClientIP inspects the context to retrieve the ip address of the client
func getClientIP(ctx context.Context) string {
	p, ok := peer.FromContext(ctx)
	if !ok {
		log.Warnf("couldn't parse client IP address")
		return ""
	}
	address := p.Addr.String()
	ip := strings.Split(address, ":")[0]
	return ip
}
