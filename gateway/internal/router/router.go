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
			route := strings.TrimRight(svc.Route, "/")

			// serve CSS static files
			stylesPath := filepath.Join(svc.TemplateDir, "styles")
			mux.Handle(route+"/styles/", http.StripPrefix(route+"/styles/", http.FileServer(http.Dir(stylesPath))))

			// serve JS static files
			scriptsPath := filepath.Join(svc.TemplateDir, "scripts")
			mux.Handle(route+"/scripts/", http.StripPrefix(route+"/scripts/", http.FileServer(http.Dir(scriptsPath))))

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
				handler = proxy.NewWebSocketProxyHandler(svc.Route, first)
			} else if len(targets) > 1 {
				handler = proxy.BuildLoadBalancer(targets, restTransport)
			} else {
				p, err := proxy.BuildReverseProxy(first, restTransport)
				if err != nil {
					return nil, err
				}
				handler = p
			}
		}

		if svc.Login {
			// register login endpoint
			mux.Handle(svc.Route+"/login", auth.LoginHandler(svc.Route, svc.SessionDuration))
			mux.Handle(svc.Route+"/login/", auth.LoginHandler(svc.Route, svc.SessionDuration))
			// register logout endpoint
			mux.Handle(svc.Route+"/logout", auth.LogoutHandler(svc.Route))
			mux.Handle(svc.Route+"/logout/", auth.LogoutHandler(svc.Route))
			// protect all other endpoints under svc.Route
			handler = auth.SessionMiddleware(svc.Route, svc.SessionDuration)(handler)
		}

		// ensure log directory exists
		logDir := filepath.Dir(svc.Log)
		if err := os.MkdirAll(logDir, 0o755); err != nil {
			return nil, err
		}
		logFile, err := os.OpenFile(svc.Log, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0o666)
		if err != nil {
			return nil, err
		}
		logger := log.New(logFile, "", 0)

		// apply logging middleware for non-WS routes
		if !isWS {
			handler = middleware.LoggingMiddleware(handler, logger, svc.Route)
		}

		// register main handler (with and without trailing slash)
		mux.Handle(svc.Route, handler)
		mux.Handle(svc.Route+"/", handler)

		log.Printf("Registered route %s", svc.Route)
	}

	return mux, nil
}
