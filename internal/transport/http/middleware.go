package http

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"
)

// ── Context key ───────────────────────────────────────────────────────────────

type contextKey string

const requestIDKey contextKey = "request_id"

// RequestIDFromContext returns the request ID stored in ctx, or empty string.
func RequestIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(requestIDKey).(string)
	return v
}

// ── responseWriter wrapper ────────────────────────────────────────────────────

type responseWriter struct {
	http.ResponseWriter
	status  int
	written bool
}

func (rw *responseWriter) WriteHeader(status int) {
	rw.status = status
	rw.written = true
	rw.ResponseWriter.WriteHeader(status)
}

func (rw *responseWriter) statusCode() int {
	if !rw.written {
		return http.StatusOK
	}
	return rw.status
}

// ── Middleware ────────────────────────────────────────────────────────────────

// RequestID generates a unique request ID for every request.
// Sets header X-Request-ID and stores the value in the request context.
func RequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := fmt.Sprintf("%d", time.Now().UnixNano())
		w.Header().Set("X-Request-ID", id)
		ctx := context.WithValue(r.Context(), requestIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// Logger logs every completed request using log/slog.
// Logs method, path, status, duration, and request_id.
func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w}
		next.ServeHTTP(rw, r)
		slog.Info("request",
			"method",      r.Method,
			"path",        r.URL.Path,
			"status",      rw.statusCode(),
			"duration_ms", time.Since(start).Milliseconds(),
			"request_id",  RequestIDFromContext(r.Context()),
		)
	})
}

// CORS adds permissive CORS headers for development.
// Handles OPTIONS preflight by returning 204 immediately.
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// Recover catches panics and returns 500 instead of crashing the server.
// Logs the panic and full stack trace at Error level.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				slog.Error("panic recovered",
					"panic", rec,
					"stack", string(debug.Stack()),
				)
				writeError(w, http.StatusInternalServerError, "internal error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// Chain applies middleware in order. The first middleware is outermost.
func Chain(h http.Handler, middleware ...func(http.Handler) http.Handler) http.Handler {
	for i := len(middleware) - 1; i >= 0; i-- {
		h = middleware[i](h)
	}
	return h
}
