package main

import (
	"log"

	"dawhub/internal/config"
	"dawhub/internal/server"
)

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Fatal("Failed to load config:", err)
	}

	// Create and start server
	srv, err := server.New(cfg)
	if err != nil {
		log.Fatal("Failed to create server:", err)
	}

	log.Fatal(srv.Start())
}
