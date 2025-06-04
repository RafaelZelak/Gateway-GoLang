//NÃO MODIFICAR ESTE ARQUIVO

package main

import (
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
	"time"

	"golang.org/x/net/http2"
	"gopkg.in/yaml.v3"
)

type ServiceConfig struct {
	Route  string `yaml:"route"`
	Target string `yaml:"target"`
}

type Config struct {
	Services []ServiceConfig `yaml:"services"`
}

// newH2CTransport retorna um *http2.Transport configurado para h2c (HTTP/2 sem TLS).
func newH2CTransport() *http2.Transport {
	return &http2.Transport{
		AllowHTTP: true,
		DialTLS: func(network, addr string, cfg *tls.Config) (net.Conn, error) {
			return net.Dial(network, addr)
		},
	}
}

// buildProxyHandler cria um handler que faz ReverseProxy
func buildProxyHandler(targetHost string, transport *http2.Transport) http.Handler {
	parsed, err := url.Parse(targetHost)
	if err != nil {
		log.Fatalf("Invalid target URL %s: %v", targetHost, err)
	}

	proxy := httputil.NewSingleHostReverseProxy(parsed)
	proxy.Transport = transport
	return proxy
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
		handler := buildProxyHandler(svc.Target, transport)

		mux.Handle(svc.Route, handler)

		pattern := svc.Route
		if !strings.HasSuffix(pattern, "/") {
			pattern = pattern + "/"
		}
		mux.Handle(pattern, handler)

		log.Printf("Registered route %s → %s (patterns: %s and %s)\n",
			svc.Route, svc.Target, svc.Route, pattern)
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

	log.Println("API Gateway listening on :80, forwarding to backends via HTTP/2 (h2c)")
	log.Fatal(server.ListenAndServe())
}
