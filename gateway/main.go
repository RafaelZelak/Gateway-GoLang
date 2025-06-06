package main

import (
	"context"
	"flag"
	"html/template"
	"io"
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
	Route          string            `yaml:"route"`
	Target         string            `yaml:"target,omitempty"`
	TemplateDir    string            `yaml:"templateDir,omitempty"`
	TemplateRoutes map[string]string `yaml:"templateRoutes,omitempty"`
	Log            string            `yaml:"log,omitempty"`
	Auth           string            `yaml:"auth,omitempty"`
}

type Config struct {
	Services []ServiceConfig `yaml:"services"`
}

type backend struct {
	url   *url.URL
	proxy *httputil.ReverseProxy
}

type lbHandler struct {
	backends []*backend
	counter  uint64
}

func newLBHandler(targets []string, transport http.RoundTripper) http.Handler {
	var backends []*backend
	for _, target := range targets {
		targetURL, err := url.Parse(strings.TrimSpace(target))
		if err != nil {
			log.Fatalf("Invalid target URL %s: %v", target, err)
		}
		proxy := httputil.NewSingleHostReverseProxy(targetURL)
		proxy.Transport = transport
		backends = append(backends, &backend{url: targetURL, proxy: proxy})
	}
	return &lbHandler{backends: backends}
}

func (lb *lbHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	i := atomic.AddUint64(&lb.counter, 1)
	backend := lb.backends[i%uint64(len(lb.backends))]
	backend.proxy.ServeHTTP(w, r)
}

func buildWebSocketProxy(target string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isWebSocket(r) {
			http.Error(w, "Expected WebSocket connection", http.StatusBadRequest)
			return
		}

		req := r.Clone(context.Background())
		req.URL.Scheme = "http"
		req.URL.Host = strings.TrimPrefix(target, "ws://")
		req.URL.Path = r.URL.Path
		req.Host = req.URL.Host

		dialer := net.Dialer{Timeout: 5 * time.Second}
		connBackend, err := dialer.Dial("tcp", req.URL.Host)
		if err != nil {
			http.Error(w, "Could not connect to WebSocket backend", http.StatusBadGateway)
			return
		}

		hj, ok := w.(http.Hijacker)
		if !ok {
			http.Error(w, "Hijacking not supported", http.StatusInternalServerError)
			return
		}

		connClient, _, err := hj.Hijack()
		if err != nil {
			http.Error(w, "Hijack failed", http.StatusInternalServerError)
			return
		}

		defer connBackend.Close()
		defer connClient.Close()

		err = req.Write(connBackend)
		if err != nil {
			log.Printf("Failed to write WebSocket handshake: %v", err)
			return
		}

		go io.Copy(connBackend, connClient)
		io.Copy(connClient, connBackend)
	})
}

type templateHandler struct {
	templates *template.Template
	baseRoute string
	aliases   map[string]string
}

func newTemplateHandler(dirPath, route string, aliases map[string]string) http.Handler {
	patterns := filepath.Join(dirPath, "*.html")
	tmpl, err := template.ParseGlob(patterns)
	if err != nil {
		log.Fatalf("Error parsing templates in %s: %v", dirPath, err)
	}
	return &templateHandler{templates: tmpl, baseRoute: route, aliases: aliases}
}

func (th *templateHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	tmplName := "index.html"
	path := strings.TrimPrefix(r.URL.Path, th.baseRoute)
	path = strings.Trim(path, "/")

	if path != "" {
		if mapped, ok := th.aliases[path]; ok {
			tmplName = mapped
		} else if strings.HasSuffix(path, ".html") {
			tmplName = path
		} else {
			tmplName = path + ".html"
		}
	}

	if th.templates.Lookup(tmplName) == nil {
		http.NotFound(w, r)
		return
	}
	if err := th.templates.ExecuteTemplate(w, tmplName, nil); err != nil {
		http.Error(w, "Template rendering error", http.StatusInternalServerError)
	}
}

func newDefaultTransport() http.RoundTripper {
	tr := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2: true,
	}
	_ = http2.ConfigureTransport(tr)
	return tr
}

func isWebSocket(r *http.Request) bool {
	connHeader := strings.ToLower(r.Header.Get("Connection"))
	upgradeHeader := strings.ToLower(r.Header.Get("Upgrade"))
	return strings.Contains(connHeader, "upgrade") && upgradeHeader == "websocket"
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

	transport := newDefaultTransport()
	mux := http.NewServeMux()

	for _, svc := range cfg.Services {
		var handler http.Handler

		if svc.TemplateDir != "" {
			if _, err := os.Stat(svc.TemplateDir); os.IsNotExist(err) {
				log.Fatalf("Template directory does not exist: %s", svc.TemplateDir)
			}
			handler = newTemplateHandler(svc.TemplateDir, svc.Route, svc.TemplateRoutes)
			mux.Handle(svc.Route+"/", handler)
			mux.Handle(svc.Route, handler)
			log.Printf("Registered template route %s → %s", svc.Route, svc.TemplateDir)
		} else {
			targets := strings.Split(svc.Target, ",")
			firstTarget := strings.TrimSpace(targets[0])

			if strings.HasPrefix(firstTarget, "ws://") || strings.HasPrefix(firstTarget, "wss://") {
				handler = buildWebSocketProxy(firstTarget)
				mux.Handle(svc.Route, handler)
				log.Printf("Registered WS route %s → %s", svc.Route, firstTarget)
			} else if len(targets) > 1 {
				handler = newLBHandler(targets, transport)
				mux.Handle(svc.Route+"/", handler)
				mux.Handle(svc.Route, handler)
				log.Printf("Registered LB route %s → %v", svc.Route, targets)
			} else {
				// Parse the full target URL (e.g. "http://health_service:8000")
				targetURL, err := url.Parse(firstTarget)
				if err != nil {
					log.Fatalf("Invalid target URL %s: %v", firstTarget, err)
				}

				proxy := httputil.NewSingleHostReverseProxy(targetURL)
				proxy.Transport = transport
				handler = proxy
				mux.Handle(svc.Route+"/", handler)
				mux.Handle(svc.Route, handler)
				log.Printf("Registered HTTP route %s → %s", svc.Route, firstTarget)
			}
		}
	}

	srv := &http.Server{
		Addr:         ":80",
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Println("API Gateway listening on :80 (HTTP/1.1 + HTTP/2 + WebSocket + Templates)")
	log.Fatal(srv.ListenAndServe())
}
