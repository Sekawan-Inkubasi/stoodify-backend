package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"stoodify_backend/config"
	"stoodify_backend/routes"
	"stoodify_backend/workers"
)

func main() {
	config.KonekKeDB()

	// Scheduler: tandai tugas terlambat + notifikasi H-1/H-0.
	berhenti := make(chan struct{})
	go workers.JalankanScheduler(config.DB, berhenti)

	sinyal := make(chan os.Signal, 1)
	signal.Notify(sinyal, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sinyal
		log.Println("menerima sinyal berhenti")
		close(berhenti)
		os.Exit(0)
	}()

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
