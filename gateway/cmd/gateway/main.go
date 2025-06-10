package main

import (
	"log"
	"net/http"

	"github.com/RafaelZelak/gateway/internal/config"
	"github.com/RafaelZelak/gateway/internal/jobs"
	"github.com/RafaelZelak/gateway/internal/router"
	"github.com/joho/godotenv"
)

func main() {
	// ensure external DNS resolution works (adds 8.8.8.8 if missing)
	jobs.EnsureResolvConf()

	// initialize job scheduler
	if err := jobs.InitJobScheduler(); err != nil {
		log.Fatalf("Failed to init job scheduler: %v", err)
	}

	// load .env if present
	if err := godotenv.Load(); err != nil {
		log.Printf(".env not found, relying on environment variables: %v", err)
	}

	// load gateway configuration
	cfg, err := config.LoadConfig("config.yml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// build HTTP router
	mux, err := router.NewRouter(cfg)
	if err != nil {
		log.Fatalf("Failed to create router: %v", err)
	}

	// start HTTP server
	port := "8080"
	if port == "" {
		port = "80"
	}
	log.Printf("Starting server on port %s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("Server failed: %v", err)
	}
}
