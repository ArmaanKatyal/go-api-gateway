package main

import (
	"io"
	"log/slog"
	"net/http"
	"strings"
)

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

// health is a simple health check endpoint
func health(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("OK")); err != nil {
		slog.Error("Error writing response", "error", err.Error())
	}
}

// config returns the application configuration
func config(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(AppConfig.GetConfMarshal()); err != nil {
		slog.Error("Error writing response", "error", err.Error())
	}
}

// InitializeRoutes initializes the application routes
func InitializeRoutes(r *RequestHandler) *http.ServeMux {
	go r.ServiceRegistry.Heartbeat()
	go r.RateLimiter.cleanupVisitors()

	mux := http.NewServeMux()
	mux.HandleFunc("POST /services/register", r.ServiceRegistry.Register_service)
	mux.HandleFunc("POST /services/deregister", r.ServiceRegistry.Deregister_service)
	mux.HandleFunc("GET /services", r.ServiceRegistry.Get_services)
	mux.HandleFunc("GET /health", health)
	mux.HandleFunc("GET /config", config)
	mux.HandleFunc("/", r.handle_request)
	return mux
}

// resolve_path splits the path into service name and route path
func resolve_path(path string) (string, []string) {
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return path, nil
	}
	return parts[1], parts[2:]
}

// create_forward_uri creates a new uri based on the resolved request
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

// handle_request handles the incoming request and forwards it to the resolved service
func (rh *RequestHandler) handle_request(w http.ResponseWriter, r *http.Request) {
	slog.Info("Received request", "path", r.URL.Path, "method", r.Method)
	service_name, route := resolve_path(r.URL.Path)
	if ok, err := rh.ServiceRegistry.IsWhitelisted(service_name, r.RemoteAddr); !ok || err != nil {
		slog.Error("Unauthorized request", "path", r.URL.Path, "method", r.Method, "ip", r.RemoteAddr)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if !rh.RateLimiter.Allow(r.RemoteAddr) {
		slog.Error("Rate limit exceeded", "path", r.URL.Path, "method", r.Method, "ip", r.RemoteAddr)
		http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
		return
	}
	// TODO: Authenticate the request

	slog.Info("Resolving service", "service_name", service_name)

	address := rh.ServiceRegistry.GetAddress(service_name)
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

// forward_request forwards the request to the resolved service
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
