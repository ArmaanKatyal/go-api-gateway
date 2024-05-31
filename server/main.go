package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

type ServiceRegistry struct{}

func NewServiceRegistry() ServiceRegistry {
	return ServiceRegistry{}
}

type RateLimiter struct{}

func initializeRoutes(r *ServiceRegistry) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /register_service", r.Register_service)
	mux.HandleFunc("POST /deregister_service", r.Deregister_service)
	return mux
}

type RegisterBody struct{}

type RegisterResponse struct {
	Message string `json:"message"`
}

type DeregisterBody struct{}

type DeregisterResponse struct{}

func (sr *ServiceRegistry) Register_service(w http.ResponseWriter, r *http.Request) {
	j, err := json.Marshal(RegisterResponse{Message: "Hello World"})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	w.WriteHeader(http.StatusOK)
	w.Write(j)
}

func (sr *ServiceRegistry) Deregister_service(w http.ResponseWriter, r *http.Request) {}

func main() {
	sr := NewServiceRegistry()
	router := initializeRoutes(&sr)
	server := &http.Server{
		Addr:         ":8080",
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	slog.Info("api gateway listening on port 8080")
	server.ListenAndServe()
}
