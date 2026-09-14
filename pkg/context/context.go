package context

import (
	"context"

	"github.com/sirupsen/logrus"
)

// RequestContext holds tenant and request identity information
type RequestContext struct {
	TenantID   string
	UserID     string
	RequestID  string
	TraceID    string
	ClientCert string
	Timestamp  int64
	ReleaseVer string
}

// ContextKey is a private type for context keys
type ContextKey string

const (
	RequestContextKey ContextKey = "request_context"
	TenantKey         ContextKey = "tenant_id"
	TraceIDKey        ContextKey = "trace_id"
)

// NewContextWithRequest embeds RequestContext into a context
func NewContextWithRequest(ctx context.Context, reqCtx *RequestContext) context.Context {
	ctx = context.WithValue(ctx, RequestContextKey, reqCtx)
	ctx = context.WithValue(ctx, TenantKey, reqCtx.TenantID)
	ctx = context.WithValue(ctx, TraceIDKey, reqCtx.TraceID)
	return ctx
}

// FromContext extracts RequestContext from context
func FromContext(ctx context.Context) *RequestContext {
	rc, ok := ctx.Value(RequestContextKey).(*RequestContext)
	if !ok {
		return &RequestContext{
			TenantID: "unknown",
			TraceID:  "unknown",
		}
	}
	return rc
}

// TenantFromContext extracts tenant ID from context
func TenantFromContext(ctx context.Context) string {
	tenant, ok := ctx.Value(TenantKey).(string)
	if !ok {
		return "unknown"
	}
	return tenant
}

// TraceIDFromContext extracts trace ID from context
func TraceIDFromContext(ctx context.Context) string {
	traceID, ok := ctx.Value(TraceIDKey).(string)
	if !ok {
		return "unknown"
	}
	return traceID
}

// ContextLogger wraps logrus.Entry with tenant context
func ContextLogger(ctx context.Context, log *logrus.Logger) *logrus.Entry {
	reqCtx := FromContext(ctx)
	return log.WithFields(logrus.Fields{
		"tenant_id":   reqCtx.TenantID,
		"trace_id":    reqCtx.TraceID,
		"request_id":  reqCtx.RequestID,
		"user_id":     reqCtx.UserID,
		"release_ver": reqCtx.ReleaseVer,
	})
}
