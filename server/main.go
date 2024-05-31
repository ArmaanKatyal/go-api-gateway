package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

type ServiceRegistry struct{}

func (sr *ServiceRegistry) Register(name string, address string) {}
func (sr *ServiceRegistry) Deregister(name string)               {}
func (sr *ServiceRegistry) GetAddress(name string) string        { return "" }

func NewServiceRegistry() *ServiceRegistry {
	return &ServiceRegistry{}
}

type RateLimiter struct{}

func initializeRoutes(r *RequestHandler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", r.Handle_request)
	return mux
}

type RegisterBody struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

type RegisterResponse struct {
	Message string `json:"message"`
}

type DeregisterBody struct {
	Name string `json:"name"`
}

type DeregisterResponse struct {
	Message string `json:"message"`
}

type RequestHandler struct {
	ServiceRegistry *ServiceRegistry
}

func NewRequestHandler(sr *ServiceRegistry) *RequestHandler {
	return &RequestHandler{ServiceRegistry: sr}
}

func (rh *RequestHandler) Handle_request(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	method := r.Method

	switch {
	case method == "POST" && path == "/register_service":
		rh.ServiceRegistry.Register_service(w, r)
	case method == "POST" && path == "/deregister_service":
		rh.ServiceRegistry.Deregister_service(w, r)
	default:
		http.Error(w, "Not Found", http.StatusNotFound)
	}
}

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
	rh := NewRequestHandler(sr)
	router := initializeRoutes(rh)
	server := &http.Server{
		Addr:         ":8080",
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	slog.Info("api gateway listening on port 8080")
	server.ListenAndServe()
}
