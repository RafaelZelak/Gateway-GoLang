package main

import (
	"context"
	"crypto/tls"
	"flag"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"golang.org/x/net/http2"
	"gopkg.in/yaml.v3"
)

// ServiceConfig represents one entry em config.yml
type ServiceConfig struct {
	Route  string `yaml:"route"`  // e.g. "/health"
	Target string `yaml:"target"` // e.g. "http://srv1:8000, http://srv2:8000"
}

// Config mantém a lista de serviços do config.yml
type Config struct {
	Services []ServiceConfig `yaml:"services"`
}

// backend encapsula info de cada instância (URL, proxy e estado de saúde)
type backend struct {
	url     string
	proxy   *httputil.ReverseProxy
	healthy int32 // 1 = saudável, 0 = down
}

// lbHandler implementa round-robin considerando apenas backends saudáveis
type lbHandler struct {
	backends []*backend
	counter  uint64
	interval time.Duration
	client   *http.Client
}

// newH2CTransport retorna um *http2.Transport configurado para h2c (HTTP/2 sem TLS)
func newH2CTransport() *http2.Transport {
	return &http2.Transport{
		AllowHTTP: true,
		DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
			return net.Dial(network, addr) // conexão TCP sem TLS
		},
	}
}

// buildSingleHostProxy cria um ReverseProxy que encaminha para targetHost único
func buildSingleHostProxy(targetHost string, transport *http2.Transport) http.Handler {
	parsed, err := url.Parse(targetHost)
	if err != nil {
		log.Fatalf("Invalid target URL %s: %v", targetHost, err)
	}
	proxy := httputil.NewSingleHostReverseProxy(parsed)
	proxy.Transport = transport
	return proxy
}

// newLBHandler recebe lista de URLs, cria um backend para cada e inicia health checks
func newLBHandler(targets []string, transport *http2.Transport) http.Handler {
	backends := make([]*backend, 0, len(targets))
	// HTTP client com timeout para health check
	httpClient := &http.Client{
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
			healthy: 1, // assumimos saudável inicialmente; o health check ajusta depois
		}
		backends = append(backends, b)
	}

	if len(backends) == 0 {
		log.Fatalf("No valid targets provided for load balancer: %v", targets)
	}

	handler := &lbHandler{
		backends: backends,
		interval: 10 * time.Second, // intervalo para checar cada backend
		client:   httpClient,
	}

	// Inicia uma goroutine de health check para cada backend
	for _, b := range handler.backends {
		go handler.monitorHealth(b)
		log.Printf("Health check iniciado para %s\n", b.url)
	}

	return handler
}

// monitorHealth executa health checks periódicos em um backend
func (l *lbHandler) monitorHealth(b *backend) {
	for {
		// Use contexto com timeout para não travar indefinidamente
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.url, nil)
		if err != nil {
			atomic.StoreInt32(&b.healthy, 0)
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
		}

		time.Sleep(l.interval)
	}
}

// ServeHTTP seleciona, em round-robin, o próximo backend saudável
func (l *lbHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	total := uint64(len(l.backends))
	if total == 0 {
		http.Error(w, "Bad Gateway", http.StatusBadGateway)
		return
	}

	// Tentamos encontrar um backend saudável em até "total" tentativas
	for i := uint64(0); i < total; i++ {
		// índice atômico
		idx := atomic.AddUint64(&l.counter, 1)
		b := l.backends[(idx-1)%total]

		if atomic.LoadInt32(&b.healthy) == 1 {
			// backend saudável encontrado; encaminha a requisição
			b.proxy.ServeHTTP(w, r)
			return
		}
	}

	// Nenhum backend saudável encontrado
	http.Error(w, "Bad Gateway: no healthy backends", http.StatusBadGateway)
}

func main() {
	// 1) Lê caminho do config via flag (padrão "config.yml")
	configPath := flag.String("config", "config.yml", "Path to YAML config")
	flag.Parse()

	absConfig, err := filepath.Abs(*configPath)
	if err != nil {
		log.Fatalf("Failed to resolve config path: %v", err)
	}

	// 2) Carrega e faz parsing do YAML
	data, err := ioutil.ReadFile(absConfig)
	if err != nil {
		log.Fatalf("Error reading config file %s: %v", absConfig, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		log.Fatalf("Error parsing YAML: %v", err)
	}

	// 3) Cria transporte HTTP/2 (h2c) para comunicação interna
	transport := newH2CTransport()

	mux := http.NewServeMux()
	for _, svc := range cfg.Services {
		targets := strings.Split(svc.Target, ",")
		var handler http.Handler

		if len(targets) > 1 {
			// cria handler com load balancing e health checks
			handler = newLBHandler(targets, transport)
			log.Printf("Registered route %s → [load-balanced com health-check: %v]\n", svc.Route, targets)
		} else {
			// comportamento original (proxy único)
			handler = buildSingleHostProxy(strings.TrimSpace(targets[0]), transport)
			log.Printf("Registered route %s → %s\n", svc.Route, strings.TrimSpace(targets[0]))
		}

		// registra rota sem barra final
		mux.Handle(svc.Route, handler)
		// registra rota com barra final
		pattern := svc.Route
		if !strings.HasSuffix(pattern, "/") {
			pattern = pattern + "/"
		}
		mux.Handle(pattern, handler)
	}

	// catch-all devolve 404
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

	log.Println("API Gateway listening on :80 com health checks para backends")
	log.Fatal(server.ListenAndServe())
}
