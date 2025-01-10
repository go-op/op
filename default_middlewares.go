package fuego

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
)

// By default, all logging is enabled
var defaultLoggingConfig = LoggingConfig{
	RequestIDFunc: defaultRequestIDFunc,
}

// LoggingConfig is the configuration for the default logging middleware
//
// It allows for request and response logging to be disabled independently,
// and for a custom request ID generator to be used
//
// For example:
//
//	config := fuego.LoggingConfig{
//		    DisableRequest:  true,
//		    RequestIDFunc: func() string {
//		        return fmt.Sprintf("custom-%d", time.Now().UnixNano())
//		    },
//		}
//
// The above configuration will disable the debug request logging and
// override the default request ID generator (UUID) with a custom one that
// appends the current Unix time in nanoseconds for response logs
type LoggingConfig struct {
	// Optional custom request ID generator
	RequestIDFunc func() string
	// If true, request logging is disabled
	DisableRequest bool
	// If true, response logging is disabled
	DisableResponse bool
}

func (l *LoggingConfig) Disabled() bool {
	return l.DisableRequest && l.DisableResponse
}

// defaultRequestIDFunc generates a UUID as the default request ID if none exist in X-Request-ID header
func defaultRequestIDFunc() string {
	return uuid.New().String()
}

// responseWriter wraps [http.ResponseWriter] to capture response metadata.
// Implements [http.ResponseWriter.Write] to ensure proper status code capture for implicit 200 responses
type responseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func newResponseWriter(w http.ResponseWriter) *responseWriter {
	return &responseWriter{ResponseWriter: w}
}

func (rw *responseWriter) WriteHeader(code int) {
	if rw.wroteHeader {
		return
	}

	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
	rw.wroteHeader = true
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

func logRequest(requestID string, r *http.Request) {
	slog.Debug("incoming request",
		"request_id", requestID,
		"method", r.Method,
		"path", r.URL.Path,
		"timestamp", time.Now().Format(time.RFC3339),
		"remote_addr", r.RemoteAddr,
		"user_agent", r.UserAgent(),
	)
}

func logResponse(r *http.Request, rw *responseWriter, requestID string, duration time.Duration) {
	slog.Info("outgoing response",
		"request_id", requestID,
		"method", r.Method,
		"path", r.URL.Path,
		"timestamp", time.Now().Format(time.RFC3339),
		"remote_addr", r.RemoteAddr,
		"duration_ms", duration.Milliseconds(),
		"status_code", rw.status,
	)
}

type defaultLogger struct {
	s *Server
}

func newDefaultLogger(s *Server) defaultLogger {
	return defaultLogger{s: s}
}

// defaultLogger.middleware is the default middleware that logs incoming requests and outgoing responses.
//
// By default, request logging will be logged at the debug level, and response
// logging will be logged at the info level
//
// Log levels managed by [WithLogHandler]
func (l defaultLogger) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = l.s.loggingConfig.RequestIDFunc()
		}
		w.Header().Set("X-Request-ID", requestID)

		wrapped := newResponseWriter(w)

		if !l.s.loggingConfig.DisableRequest {
			logRequest(requestID, r)
		}

		next.ServeHTTP(wrapped, r)

		if !l.s.loggingConfig.DisableResponse {
			duration := time.Since(start)
			logResponse(r, wrapped, requestID, duration)
		}
	})
}
