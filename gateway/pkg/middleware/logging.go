package middleware

import (
	"log"
	"net/http"
	"time"
)

// LoggingResponseWriter wraps http.ResponseWriter para capturar status code
type LoggingResponseWriter struct {
	http.ResponseWriter
	StatusCode int
}

// WriteHeader captura o status code antes de escrever
func (lrw *LoggingResponseWriter) WriteHeader(code int) {
	lrw.StatusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

// LoggingMiddleware registra timestamp, IP, método, URI, status e latência
func LoggingMiddleware(next http.Handler, logger *log.Logger, routeName string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lrw := &LoggingResponseWriter{ResponseWriter: w, StatusCode: http.StatusOK}
		next.ServeHTTP(lrw, r)
		logger.Printf("[%s] %s %s %s -> %d %v",
			time.Now().Format(time.RFC3339),
			r.RemoteAddr,
			r.Method,
			r.RequestURI,
			lrw.StatusCode,
			time.Since(start),
		)
	})
}
