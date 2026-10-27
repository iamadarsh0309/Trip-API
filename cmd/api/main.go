package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/iamadarsh0309/Trip-API/internal/config"
	"github.com/iamadarsh0309/Trip-API/internal/database"
	"github.com/iamadarsh0309/Trip-API/internal/handler"
	"github.com/iamadarsh0309/Trip-API/internal/repository"
	"github.com/iamadarsh0309/Trip-API/internal/service"

	_ "github.com/iamadarsh0309/Trip-API/docs"
)

// @title Trip Management API
// @version 1.0
// @description REST API for managing trips with idempotent creates and enforced status transitions.
// @host localhost:8080
// @BasePath /
// @schemes http
func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	gin.SetMode(cfg.GinMode)

	ctx := context.Background()

	if err := database.RunMigrations(cfg.DatabaseURL, cfg.MigrationsPath); err != nil {
		log.Fatalf("migrations: %v", err)
	}

	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer pool.Close()

	repo := repository.NewTripRepository(pool)
	svc := service.NewTripService(repo)
	h := handler.NewTripHandler(svc)
	router := handler.NewRouter(h)

	srv := &http.Server{
		Addr:              ":" + cfg.AppPort,
		Handler:           router,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("trip-api listening on :%s", cfg.AppPort)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("server: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}
