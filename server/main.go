package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	rh := NewRequestHandler()
	router := initializeRoutes(rh)
	server := &http.Server{
		Addr:         ":8080",
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	slog.Info("API Gateway started", "port", 8080)
	err := server.ListenAndServe()
	if err != nil {
		slog.Error("Error starting server", "error", err.Error())
	}
}
