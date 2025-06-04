package main

import (
	"context"
	"crypto/tls"
	"flag"
	"html/template"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/net/http2"
	"gopkg.in/yaml.v3"
)

type ServiceConfig struct {
	Route       string `yaml:"route"`
	Target      string `yaml:"target,omitempty"`
	TemplateDir string `yaml:"templateDir,omitempty"`
	Log         string `yaml:"log,omitempty"`
}

type Config struct {
	Services []ServiceConfig `yaml:"services"`
}

type backend struct {
	url     string
	proxy   *httputil.ReverseProxy
	healthy int32
}

type lbHandler struct {
	backends []*backend
	counter  uint64
	interval time.Duration
	client   *http.Client
}

type templateHandler struct {
	templates *template.Template
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

func newH2CTransport() *http2.Transport {
	return &http2.Transport{
		AllowHTTP: true,
		DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
			return net.Dial(network, addr)
		},
	}
}

func buildSingleHostProxy(targetHost string, transport *http2.Transport) http.Handler {
	parsed, err := url.Parse(targetHost)
	if err != nil {
		log.Fatalf("Invalid target URL %s: %v", targetHost, err)
	}
	proxy := httputil.NewSingleHostReverseProxy(parsed)
	proxy.Transport = transport
	return proxy
}

func newLBHandler(targets []string, transport *http2.Transport) http.Handler {
	backends := make([]*backend, 0, len(targets))
	client := &http.Client{
		Timeout:   2 * time.Second,
		Transport: transport,
	}

	for _, raw := range targets {
		addr := strings.TrimSpace(raw)
		if addr == "" {
			continue
		}
		parsed, err := url.Parse(addr)
		if err != nil {
			log.Fatalf("Invalid target URL %s: %v", addr, err)
		}
		revProxy := httputil.NewSingleHostReverseProxy(parsed)
		revProxy.Transport = transport

		b := &backend{
			url:     addr,
			proxy:   revProxy,
			healthy: 1,
		}
		backends = append(backends, b)
	}

	if len(backends) == 0 {
		log.Fatalf("No valid targets provided for load balancer: %v", targets)
	}

	handler := &lbHandler{
		backends: backends,
		interval: 10 * time.Second,
		client:   client,
	}

	for _, b := range handler.backends {
		go handler.monitorHealth(b)
		log.Printf("Health check started for %s\n", b.url)
	}
	return handler
}

func (l *lbHandler) monitorHealth(b *backend) {
	for {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.url, nil)
		if err != nil {
			atomic.StoreInt32(&b.healthy, 0)
			cancel()
		} else {
			resp, err := l.client.Do(req)
			if err != nil || resp.StatusCode >= 500 {
				atomic.StoreInt32(&b.healthy, 0)
			} else {
				atomic.StoreInt32(&b.healthy, 1)
			}
			if resp != nil {
				resp.Body.Close()
			}
			cancel()
		}
		time.Sleep(l.interval)
	}
}

func (l *lbHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	total := uint64(len(l.backends))
	if total == 0 {
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
		return
	}
	for i := uint64(0); i < total; i++ {
		idx := atomic.AddUint64(&l.counter, 1)
		b := l.backends[(idx-1)%total]
		if atomic.LoadInt32(&b.healthy) == 1 {
			b.proxy.ServeHTTP(w, r)
			return
		}
	}
	http.Error(w, "Bad Gateway: no healthy backends", http.StatusBadGateway)
}

func newTemplateHandler(dirPath string) http.Handler {
	patterns := filepath.Join(dirPath, "*.html")
	tmpl, err := template.ParseGlob(patterns)
	if err != nil {
		log.Fatalf("Error parsing templates in %s: %v", dirPath, err)
	}
	return &templateHandler{templates: tmpl}
}

func (th *templateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tmplName := "index.html"
	if r.URL.Path != "" && r.URL.Path != "/" {
		clean := strings.TrimPrefix(r.URL.Path, "/")
		tmplName = clean + ".html"
	}
	if th.templates.Lookup(tmplName) == nil {
		http.NotFound(w, r)
		return
	}
	if err := th.templates.ExecuteTemplate(w, tmplName, nil); err != nil {
		http.Error(w, "Template rendering error", http.StatusInternalServerError)
	}
}

func sanitizeRoute(route string) string {
	trimmed := strings.TrimPrefix(route, "/")
	safe := strings.ReplaceAll(trimmed, "/", "_")
	if safe == "" {
		safe = "root"
	}
	return safe
}

func createLoggedHandler(handler http.Handler, logger *log.Logger, routeName string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		lrw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		handler.ServeHTTP(lrw, r)

		duration := time.Since(start)
		logger.Printf("[%s] %s %s %s -> %d %v\n",
			time.Now().Format(time.RFC3339),
			r.RemoteAddr,
			r.Method,
			r.RequestURI,
			lrw.statusCode,
			duration,
		)
	})
}

func main() {
	configPath := flag.String("config", "config.yml", "Path to YAML config")
	flag.Parse()

	absConfig, err := filepath.Abs(*configPath)
	if err != nil {
		log.Fatalf("Failed to resolve config path: %v", err)
	}

	data, err := ioutil.ReadFile(absConfig)
	if err != nil {
		log.Fatalf("Error reading config file %s: %v", absConfig, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		log.Fatalf("Error parsing YAML: %v", err)
	}

	transport := newH2CTransport()

	mux := http.NewServeMux()
	for _, svc := range cfg.Services {
		var handler http.Handler

		if svc.TemplateDir != "" {
			if _, err := os.Stat(svc.TemplateDir); os.IsNotExist(err) {
				log.Fatalf("Template directory does not exist: %s", svc.TemplateDir)
			}
			handler = newTemplateHandler(svc.TemplateDir)
			log.Printf("Registered route %s → [internal template: %s]\n", svc.Route, svc.TemplateDir)

		} else if svc.Target != "" {
			targets := strings.Split(svc.Target, ",")
			if len(targets) > 1 {
				handler = newLBHandler(targets, transport)
				log.Printf("Registered route %s → [load-balanced: %v]\n", svc.Route, targets)
			} else {
				handler = buildSingleHostProxy(strings.TrimSpace(targets[0]), transport)
				log.Printf("Registered route %s → %s\n", svc.Route, strings.TrimSpace(targets[0]))
			}
		} else {
			log.Fatalf("Service %s must have either target or templateDir defined", svc.Route)
		}

		if svc.Log != "" {
			if err := os.MkdirAll(svc.Log, 0755); err != nil {
				log.Fatalf("Failed to create log directory %s: %v", svc.Log, err)
			}
			routeName := sanitizeRoute(svc.Route)
			filename := routeName + ".log"
			pathLog := filepath.Join(svc.Log, filename)
			f, err := os.OpenFile(pathLog, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
			if err != nil {
				log.Fatalf("Failed to open log file %s: %v", pathLog, err)
			}
			logger := log.New(f, "", 0)
			handler = createLoggedHandler(handler, logger, routeName)
			log.Printf("Logging enabled for route %s → %s\n", svc.Route, pathLog)
		}

		if svc.TemplateDir != "" {
			mux.Handle(svc.Route, http.StripPrefix(svc.Route, handler))
			pattern := svc.Route
			if !strings.HasSuffix(pattern, "/") {
				pattern = pattern + "/"
			}
			mux.Handle(pattern, http.StripPrefix(pattern, handler))
		} else {
			mux.Handle(svc.Route, handler)
			pattern := svc.Route
			if !strings.HasSuffix(pattern, "/") {
				pattern = pattern + "/"
			}
			mux.Handle(pattern, handler)
		}
	}

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	})

	server := &http.Server{
		Addr:         ":80",
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Println("API Gateway listening on :80 (h2c) with internal templates support")
	log.Fatal(server.ListenAndServe())
}
