package main

import (
	"encoding/json"
	"io"
	"log/slog"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	MaxRequestsPerMinute = 100
)

type ServiceRegistry struct {
	mu       sync.RWMutex
	Services map[string]string `json:"services"`
}

func (sr *ServiceRegistry) Register(name string, address string) {
	slog.Info("Registering service", "name", name, "address", address)
	sr.mu.Lock()
	defer sr.mu.Unlock()
	sr.Services[name] = address
}

func (sr *ServiceRegistry) Deregister(name string) {
	slog.Info("Deregistering service", "name", name)
	sr.mu.Lock()
	defer sr.mu.Unlock()
	delete(sr.Services, name)
}

func (sr *ServiceRegistry) GetAddress(name string) string {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	val, ok := sr.Services[name]
	if !ok {
		return ""
	}
	return val
}

func NewServiceRegistry() *ServiceRegistry {
	return &ServiceRegistry{
		Services: make(map[string]string),
	}
}

type client struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type RateLimiter struct {
	mu       sync.RWMutex
	visitors map[string]*client
}

func (rl *RateLimiter) cleanupVisitors() {
	for {
		slog.Info("Cleaning up visitors")
		time.Sleep(time.Minute)
		rl.mu.Lock()
		for ip, v := range rl.visitors {
			if time.Since(v.lastSeen) > 2*time.Minute {
				delete(rl.visitors, ip)
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *RateLimiter) Allow(address string) bool {
	rl.mu.Lock()
	ip, _, err := net.SplitHostPort(address)
	if err != nil {
		slog.Error("Error splitting address", "error", err.Error())
		return false
	}

	if _, found := rl.visitors[ip]; !found {
		rl.visitors[ip] = &client{
			limiter: rate.NewLimiter(rate.Every(time.Minute), MaxRequestsPerMinute),
		}
	}
	rl.visitors[ip].lastSeen = time.Now()

	if !rl.visitors[ip].limiter.Allow() {
		rl.mu.Unlock()
		return false
	}
	rl.mu.Unlock()
	return true
}

func NewRateLimiter() *RateLimiter {
	return &RateLimiter{
		visitors: make(map[string]*client),
	}
}

func initializeRoutes(r *RequestHandler) *http.ServeMux {
	go r.RateLimiter.cleanupVisitors()
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

func resolve_path(path string) (string, []string) {
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return path, nil
	}
	return parts[1], parts[2:]
}

func create_forward_uri(address string, route []string, query string) string {
	if !strings.HasPrefix(address, "http://") && !strings.HasPrefix(address, "https://") {
		address = "http://" + address
	}
	forward_uri := address + "/" + strings.Join(route, "/")
	if query != "" {
		forward_uri = forward_uri + "?" + query
	}
	return forward_uri
}

func handle_request(w http.ResponseWriter, r *http.Request, rl *RateLimiter, sr *ServiceRegistry) {
	slog.Info("Received request", "path", r.URL.Path, "method", r.Method)
	if !rl.Allow(r.RemoteAddr) {
		slog.Error("Rate limit exceeded", "path", r.URL.Path, "method", r.Method, "ip", r.RemoteAddr)
		http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
		return
	}
	// TODO: Authenticate the request

	service_name, route := resolve_path(r.URL.Path)

	slog.Info("Resolving service", "service_name", service_name)

	address := sr.GetAddress(service_name)
	if address == "" {
		slog.Error("Service not found", "service_name", service_name)
		http.Error(w, "service not found", http.StatusNotFound)
		return
	}
	// Create a new uri based on the resolved request
	forward_uri := create_forward_uri(address, route, r.URL.RawQuery)

	slog.Info("Forwarding request", "forward_uri", forward_uri)
	// Forward the request to the resolved service
	if err := forward_request(w, r, forward_uri); err != nil {
		slog.Error("Error forwarding request", "error", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func forward_request(w http.ResponseWriter, r *http.Request, forward_uri string) error {
	req, err := http.NewRequest(r.Method, forward_uri, r.Body)
	if err != nil {
		return err
	}
	req.Header = r.Header
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// Copy the response from the resolved service
	for k, v := range resp.Header {
		w.Header()[k] = v
	}
	w.WriteHeader(resp.StatusCode)
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		return err
	}
	return nil
}

func (sr *ServiceRegistry) Register_service(w http.ResponseWriter, r *http.Request) {
	var rb RegisterBody
	err := json.NewDecoder(r.Body).Decode(&rb)
	if err != nil {
		slog.Error("Error decoding request", "error", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	sr.Register(rb.Name, rb.Address)
	j, err := json.Marshal(RegisterResponse{Message: "service " + rb.Name + " registered"})
	if err != nil {
		slog.Error("Error marshalling response", "error", err.Error())
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
		slog.Error("Error decoding request", "error", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	sr.Deregister(db.Name)
	j, err := json.Marshal(DeregisterResponse{Message: "service " + db.Name + " deregistered"})
	if err != nil {
		slog.Error("Error marshalling response", "error", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(j)
}

func (sr *ServiceRegistry) Get_services(w http.ResponseWriter, r *http.Request) {
	slog.Info("Retrieved registered services")
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
	slog.Info("API Gateway started", "port", 8080)
	server.ListenAndServe()
}
