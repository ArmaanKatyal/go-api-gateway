package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"
)

type ServiceRegistry struct {
	mu       sync.RWMutex
	Services map[string]string `json:"services"`
}

func (sr *ServiceRegistry) Register(name string, address string) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	sr.Services[name] = address
}

func (sr *ServiceRegistry) Deregister(name string) {
	sr.mu.Lock()
	defer sr.mu.Unlock()
	delete(sr.Services, name)
}

func (sr *ServiceRegistry) GetAddress(name string) string {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	return sr.Services[name]
}

func NewServiceRegistry() *ServiceRegistry {
	return &ServiceRegistry{
		Services: make(map[string]string),
	}
}

type RateLimiter struct{}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{}
}

func initializeRoutes(r *RequestHandler) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/", r.HandleRoutes)
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
	RateLimiter     *RateLimiter
}

func NewRequestHandler() *RequestHandler {
	return &RequestHandler{
		ServiceRegistry: NewServiceRegistry(),
		RateLimiter:     NewRateLimiter(),
	}
}

func (rh *RequestHandler) HandleRoutes(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	method := r.Method

	switch {
	case method == "POST" && path == "/services/register":
		rh.ServiceRegistry.Register_service(w, r)
	case method == "POST" && path == "/services/deregister":
		rh.ServiceRegistry.Deregister_service(w, r)
	case method == "GET" && path == "/services":
		rh.ServiceRegistry.Get_services(w, r)
	case method == "GET" && path == "/health":
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	default:
		handle_request(w, r, rh.RateLimiter, rh.ServiceRegistry)
	}
}

func handle_request(w http.ResponseWriter, r *http.Request, rl *RateLimiter, sr *ServiceRegistry) {}

func (sr *ServiceRegistry) Register_service(w http.ResponseWriter, r *http.Request) {
	var rb RegisterBody
	err := json.NewDecoder(r.Body).Decode(&rb)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	sr.Register(rb.Name, rb.Address)
	j, err := json.Marshal(RegisterResponse{Message: "service " + rb.Name + " registered"})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(j)
}

func (sr *ServiceRegistry) Deregister_service(w http.ResponseWriter, r *http.Request) {
	var db DeregisterBody
	err := json.NewDecoder(r.Body).Decode(&db)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	sr.Deregister(db.Name)
	j, err := json.Marshal(DeregisterResponse{Message: "service " + db.Name + " deregistered"})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(j)
}

func (sr *ServiceRegistry) Get_services(w http.ResponseWriter, r *http.Request) {
	j, err := json.Marshal(sr.Services)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(j)
}

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
	slog.Info("api gateway listening on port 8080")
	server.ListenAndServe()
}
