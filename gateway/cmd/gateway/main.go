package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ChGSufro/sis-distribuidos-casino/gateway/internal/client"
	"github.com/ChGSufro/sis-distribuidos-casino/gateway/internal/config"
	"github.com/ChGSufro/sis-distribuidos-casino/gateway/internal/router"
)

func main() {
	log.Println("Iniciando API Gateway de Casino Online...")

	cfg := config.Load()

	// 1. Inicializar clientes gRPC hacia los microservicios
	clients, err := client.Init(cfg)
	if err != nil {
		log.Fatalf("Fallo crítico al inicializar clientes gRPC: %v", err)
	}
	defer clients.Close()

	// 2. Construir router HTTP con handlers y middlewares
	r := router.New(clients)

	addr := fmt.Sprintf(":%s", cfg.HTTPPort)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// 3. Canal de señales para graceful shutdown
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.Printf("API Gateway HTTP escuchando en http://0.0.0.0%s", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("Error crítico en servidor HTTP: %v", err)
		}
	}()

	// Esperar señal de apagado
	sig := <-stopChan
	log.Printf("Señal de apagado recibida (%s). Cerrando API Gateway...", sig)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Error durante el apagado del servidor HTTP: %v", err)
	}

	log.Println("API Gateway detenido ordenadamente.")
}
