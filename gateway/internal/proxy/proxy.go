package proxy

import (
	"log"
	"math/rand"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"golang.org/x/net/http2"
)

// NewDefaultTransport returns an HTTP/2-capable transport for REST proxying.
func NewDefaultTransport() http.RoundTripper {
	tr := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		DialContext:         (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		ForceAttemptHTTP2:   true,
		TLSHandshakeTimeout: 10 * time.Second,
	}
	_ = http2.ConfigureTransport(tr)
	return tr
}

// BuildReverseProxy creates a reverse proxy for HTTP targets.
func BuildReverseProxy(target string, transport http.RoundTripper) (*httputil.ReverseProxy, error) {
	u, err := url.Parse(strings.TrimSpace(target))
	if err != nil {
		return nil, err
	}
	p := httputil.NewSingleHostReverseProxy(u)
	p.Transport = transport
	return p, nil
}

// BuildLoadBalancer creates a simple round-robin proxy for multiple HTTP targets.
func BuildLoadBalancer(targets []string, transport http.RoundTripper) http.Handler {
	var proxies []*httputil.ReverseProxy
	for _, t := range targets {
		u, err := url.Parse(strings.TrimSpace(t))
		if err != nil {
			log.Fatalf("Invalid backend URL %s: %v", t, err)
		}
		p := httputil.NewSingleHostReverseProxy(u)
		p.Transport = transport
		proxies = append(proxies, p)
	}
	rand.Seed(time.Now().UnixNano())
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		idx := rand.Intn(len(proxies))
		proxies[idx].ServeHTTP(w, r)
	})
}

// NewWebSocketProxyHandler proxies WebSocket connections between client and backend.
func NewWebSocketProxyHandler(prefix, target string) http.Handler {
	upgrader := websocket.Upgrader{CheckOrigin: func(r *http.Request) bool { return true }}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		clientConn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("WebSocket upgrade error: %v", err)
			return
		}
		defer clientConn.Close()

		backendURL := "ws://" + strings.TrimPrefix(target, "ws://") + r.URL.Path
		backendConn, _, err := websocket.DefaultDialer.Dial(backendURL, nil)
		if err != nil {
			log.Printf("WebSocket backend dial error: %v", err)
			return
		}
		defer backendConn.Close()

		errc := make(chan error, 2)

		go func() {
			for {
				mt, msg, err := clientConn.ReadMessage()
				if err != nil {
					errc <- err
					return
				}
				if err := backendConn.WriteMessage(mt, msg); err != nil {
					errc <- err
					return
				}
			}
		}()
		go func() {
			for {
				mt, msg, err := backendConn.ReadMessage()
				if err != nil {
					errc <- err
					return
				}
				if err := clientConn.WriteMessage(mt, msg); err != nil {
					errc <- err
					return
				}
			}
		}()

		<-errc
	})
}
