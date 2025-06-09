package middleware

import (
	"net/http"
	"sync"

	"golang.org/x/time/rate"
)

var (
	limitersMutex   = &sync.Mutex{}
	limiters        = make(map[string]*rate.Limiter)
	semaphoresMutex = &sync.Mutex{}
	connSemaphores  = make(map[string]chan struct{})
)

// RateLimit aplica um leaky‐bucket por IP (rps, burst).
func RateLimit(rps float64, burst int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.RemoteAddr

			limitersMutex.Lock()
			lim, ok := limiters[key]
			if !ok {
				lim = rate.NewLimiter(rate.Limit(rps), burst)
				limiters[key] = lim
			}
			limitersMutex.Unlock()

			if !lim.Allow() {
				w.Header().Set("Retry-After", "1")
				http.Error(w, http.StatusText(http.StatusTooManyRequests), http.StatusTooManyRequests)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// ConnLimit aplica semáforo por IP para limitar conexões simultâneas.
func ConnLimit(limit int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			key := r.RemoteAddr

			semaphoresMutex.Lock()
			sem, ok := connSemaphores[key]
			if !ok {
				sem = make(chan struct{}, limit)
				connSemaphores[key] = sem
			}
			semaphoresMutex.Unlock()

			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
				next.ServeHTTP(w, r)
			default:
				http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
			}
		})
	}
}

// QueueLimit controla uma fila de requisições antes de processar.
func QueueLimit(size int) func(http.Handler) http.Handler {
	queue := make(chan struct{}, size)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			select {
			case queue <- struct{}{}:
				defer func() { <-queue }()
				next.ServeHTTP(w, r)
			default:
				http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
			}
		})
	}
}
