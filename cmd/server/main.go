package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/HarmanSingh16/HMS-backend/internal/api"
	"github.com/HarmanSingh16/HMS-backend/internal/api/handler"
	"github.com/HarmanSingh16/HMS-backend/internal/config"
	"github.com/HarmanSingh16/HMS-backend/internal/db"
	"github.com/HarmanSingh16/HMS-backend/internal/service"
	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("failed to load config %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("unable to connect to db: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("database ping failed: %v", err)
	}
	log.Println("connected to db successfully")

	// querier and handlers
	querier := db.New(pool)

	// Services:
	authService := service.NewAuthService(querier, cfg)
	accountService := service.NewAccountService(querier, pool, cfg)
	patientService := service.NewPatientService(querier, pool, cfg)

	// Handlers
	authHandler := handler.NewAuthHanlder(authService)
	adminHandler := handler.NewAdminHandler(accountService)
	patientHandler := handler.NewPatientHandler(patientService)

	router := api.SetupRouter(&api.RouterConfig{
		Config:         cfg,
		AuthHandler:    authHandler,
		AdminHandler:   adminHandler,
		PatientHandler: patientHandler,
	})

	srv := &http.Server{
		Addr:    fmt.Sprintf(":%s", cfg.ServerPort),
		Handler: router,
	}

	go func() {
		log.Printf("server running on port: %s", cfg.ServerPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("server error: %v", err)
		}
	}()

	// Interrupt and stop
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("shutting down server gracefully...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("server forced to shutdown: %v", err)
	}

	log.Println("server exited cleanly")
}
