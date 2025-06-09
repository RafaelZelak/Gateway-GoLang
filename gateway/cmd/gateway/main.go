package main

import (
	"log"
	"net"
	"net/http"
	"time"

	"github.com/RafaelZelak/gateway/internal/config"
	"github.com/RafaelZelak/gateway/internal/router"
	"golang.org/x/net/netutil"
)

func main() {
	cfg, err := config.LoadConfig("config.yml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	mux, err := router.NewRouter(cfg)
	if err != nil {
		log.Fatalf("Failed to create router: %v", err)
	}

	listener, err := net.Listen("tcp", ":80")
	if err != nil {
		log.Fatalf("Listen error: %v", err)
	}

	listener = netutil.LimitListener(listener.(*net.TCPListener), 100)

	server := &http.Server{
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Println("API Gateway listening on :80")
	log.Fatal(server.Serve(listener))
}
