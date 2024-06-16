package main

import (
	"context"
	"crypto/tls"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"time"
)

func main() {
	// Initialize logger
	opts := PrettyHandlerOptions{
		SlogOpts: slog.HandlerOptions{
			Level: slog.LevelDebug,
		},
	}
	handler := NewPrettyHandler(os.Stdout, opts)
	logger := slog.New(handler)
	slog.SetDefault(logger)

	// Load configuration
	LoadConf()
	// Initialize registry
	rh := NewRequestHandler()
	router := InitializeRoutes(rh)

	tlsConfig := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}
	server := &http.Server{
		Addr:         ":" + AppConfig.Server.Port,
		Handler:      router,
		ReadTimeout:  time.Duration(AppConfig.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(AppConfig.Server.WriteTimeout) * time.Second,
		TLSConfig:    tlsConfig,
	}

	slog.Info("API Gateway started", "port", AppConfig.Server.Port)
	go func() {
		// Start server
		if TLSEnabled() {
			if err := server.ListenAndServeTLS(GetCertFile(), GetKeyFile()); err != nil {
				slog.Error("Error starting server", "error", err.Error())
				os.Exit(1)
			}
		} else {
			if err := server.ListenAndServe(); err != nil {
				slog.Error("Error starting server", "error", err.Error())
				os.Exit(1)
			}
		}
	}()

	// Graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)

	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(AppConfig.Server.GracefulTimeout)*time.Second)
	defer cancel()
	slog.Info("Gracefully shutting down server")
	if err := server.Shutdown(ctx); err != nil {
		slog.Error("Error shutting down server", "error", err.Error())
		os.Exit(1)
	}
}
