package main

import (
	"log"
	"os"

	"stoodify_backend/config"
	"stoodify_backend/routes"
	// "stoodify_backend/routes"
)

func main() {
	config.KonekKeDB()

	router := routes.SetupRoutes()

	port := os.Getenv("PORT")

	if port == "" {
		port = "8080"
	}

	log.Printf("Server running on port %s", port)

	if err := router.Run(":" + port); err != nil {
		log.Fatalf("Server failed to run: %v", err)
	}
}
