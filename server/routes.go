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
		if _, err := w.Write([]byte("OK")); err != nil {
			slog.Error("Error writing response", "error", err.Error())
		}
	default:
		handle_request(w, r, rh.RateLimiter, rh.ServiceRegistry)
	}
}

func InitializeRoutes(r *RequestHandler) *http.ServeMux {
	go r.ServiceRegistry.Heartbeat()
	go r.RateLimiter.cleanupVisitors()

	mux := http.NewServeMux()
	mux.HandleFunc("/", r.HandleRoutes)
	return mux
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
