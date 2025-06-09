package main

import (
	"log"
	"net/http"
	"time"

	"github.com/RafaelZelak/gateway/internal/config"
	"github.com/RafaelZelak/gateway/internal/router"
)

func main() {
	// Load configuration from YAML
	cfg, err := config.LoadConfig("config.yml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Build the router with all routes and middlewares
	mux, err := router.NewRouter(cfg)
	if err != nil {
		log.Fatalf("Failed to create router: %v", err)
	}

	server := &http.Server{
		Addr:         ":80",
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	log.Println("API Gateway listening on :80")
	log.Fatal(server.ListenAndServe())
}
