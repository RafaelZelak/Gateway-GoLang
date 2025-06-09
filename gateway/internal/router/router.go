// internal/router/router.go
package router

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/RafaelZelak/gateway/internal/auth"
	"github.com/RafaelZelak/gateway/internal/config"
	"github.com/RafaelZelak/gateway/internal/proxy"
	"github.com/RafaelZelak/gateway/internal/template"
	"github.com/RafaelZelak/gateway/pkg/middleware"
)

// NewRouter mounts all routes (REST, templates, WebSocket) as defined in config.
func NewRouter(cfg *config.Config) (*http.ServeMux, error) {
	mux := http.NewServeMux()
	restTransport := proxy.NewDefaultTransport()

	for _, svc := range cfg.Services {
		var handler http.Handler
		isWS := strings.HasPrefix(strings.TrimSpace(svc.Target), "ws://")

		if svc.TemplateDir != "" {
			// template handler
			tmplHandler, err := template.NewTemplateHandler(svc.TemplateDir, svc.Route, svc.TemplateRoutes)
			if err != nil {
				return nil, err
			}
			handler = tmplHandler

		} else {
			targets := strings.Split(svc.Target, ",")
			first := strings.TrimSpace(targets[0])

			if isWS {
				// WebSocket proxy (sem logging middleware)
				handler = proxy.NewWebSocketProxyHandler(svc.Route, first)
			} else if len(targets) > 1 {
				// HTTP load-balancer
				handler = proxy.BuildLoadBalancer(targets, restTransport)
			} else {
				// single HTTP reverse proxy
				p, err := proxy.BuildReverseProxy(first, restTransport)
				if err != nil {
					return nil, err
				}
				handler = p
			}
		}

		// apply auth if needed
		if svc.Auth == "private" {
			handler = auth.Middleware(handler)
		}

		// prepare log directory/file
		logDir := filepath.Dir(svc.Log)
		if err := os.MkdirAll(logDir, 0o755); err != nil {
			return nil, err
		}
		logFile, err := os.OpenFile(svc.Log, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o666)
		if err != nil {
			return nil, err
		}
		logger := log.New(logFile, "", 0)

		// only apply logging middleware on non-WS routes
		if !isWS {
			handler = middleware.LoggingMiddleware(handler, logger, svc.Route)
		}

		// register with and without trailing slash
		mux.Handle(svc.Route, handler)
		mux.Handle(svc.Route+"/", handler)

		log.Printf("Registered route %s", svc.Route)
	}

	return mux, nil
}
