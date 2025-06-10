package main

import (
	"log"
	"net/http"

	"github.com/RafaelZelak/gateway/internal/config"
	"github.com/RafaelZelak/gateway/internal/router"
	"github.com/joho/godotenv"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Printf(".env not found, relying on environment variables: %v", err)
	}

	cfg, err := config.LoadConfig("config.yml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	mux, err := router.NewRouter(cfg)
	if err != nil {
		log.Fatalf("Failed to create router: %v", err)
	}

	port := "8080"
	if port == "" {
		port = "80"
	}
	log.Printf("Starting server on port %s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
