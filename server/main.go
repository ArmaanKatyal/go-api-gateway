package main

import (
	"log/slog"
	"net/http"
	"os"
	"time"
)

func main() {
	opts := PrettyHandlerOptions{
		SlogOpts: slog.HandlerOptions{
			Level: slog.LevelDebug,
		},
	}
	handler := NewPrettyHandler(os.Stdout, opts)
	logger := slog.New(handler)
	slog.SetDefault(logger)

	LoadConf()
	rh := NewRequestHandler()
	router := InitializeRoutes(rh)
	server := &http.Server{
		Addr:         ":" + AppConfig.Server.Port,
		Handler:      router,
		ReadTimeout:  time.Duration(AppConfig.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(AppConfig.Server.WriteTimeout) * time.Second,
	}
	slog.Info("API Gateway started", "port", AppConfig.Server.Port)
	err := server.ListenAndServe()
	if err != nil {
		slog.Error("Error starting server", "error", err.Error())
	}
}
