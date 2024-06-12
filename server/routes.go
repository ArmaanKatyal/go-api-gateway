package main

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/google/uuid"
	"github.com/sony/gobreaker/v2"
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

// RequestToMap converts the request to a map
func RequestToMap(r *http.Request) map[string]interface{} {
	result := make(map[string]interface{})

	result["method"] = r.Method

	result["url"] = r.URL.String()

	headers := make(map[string]string)
	for name, values := range r.Header {
		headers[name] = values[0] // Use the first value for each header
	}
	result["headers"] = headers

	queryParams := make(map[string]string)
	for name, values := range r.URL.Query() {
		queryParams[name] = values[0] // Use the first value for each query parameter
	}
	result["query_params"] = queryParams

	if err := r.ParseForm(); err == nil {
		formValues := make(map[string]string)
		for name, values := range r.Form {
			formValues[name] = values[0] // Use the first value for each form field
		}
		result["form_values"] = formValues
	}

	return result
}

// health is a simple health check endpoint
func health(w http.ResponseWriter, r *http.Request) {
	slog.Info("Health check", "req", RequestToMap(r))
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("OK")); err != nil {
		slog.Error("Error writing response", "error", err.Error())
	}
}

// config returns the application configuration
func config(w http.ResponseWriter, r *http.Request) {
	slog.Info("Get config", "req", RequestToMap(r))
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
	mux.HandleFunc("POST /services/update", r.ServiceRegistry.Update_service)
	mux.HandleFunc("GET /health", health)
	mux.HandleFunc("GET /config", config)
	mux.HandleFunc("/", r.handle_request)
	return mux
}

func (rh *RequestHandler) circuitBreakerEnabled() bool {
	return AppConfig.Server.CircuitBreaker.Enabled
}

func (rh *RequestHandler) rateLimiterEnabled() bool {
	return AppConfig.RateLimiter.Enabled
}

// resolve_path splits the path into service name and route path
func (rh *RequestHandler) resolve_path(path string) (string, []string) {
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return path, nil
	}
	return parts[1], parts[2:]
}

// create_forward_uri creates a new uri based on the resolved request
func (rh *RequestHandler) create_forward_uri(address string, route []string, query string) string {
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
	slog.Info("Received request", "req", RequestToMap(r))
	service_name, route := rh.resolve_path(r.URL.Path)
	if ok, err := rh.ServiceRegistry.IsWhitelisted(service_name, r.RemoteAddr); !ok || err != nil {
		slog.Error("Unauthorized request", "path", r.URL.Path, "method", r.Method, "ip", r.RemoteAddr, "service_name", service_name)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if rh.rateLimiterEnabled() && !rh.RateLimiter.Allow(r.RemoteAddr) {
		slog.Error("Rate limit exceeded", "path", r.URL.Path, "method", r.Method, "ip", r.RemoteAddr, "service_name", service_name)
		http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
		return
	}
	// TODO: Authenticate the request
	if err := rh.ServiceRegistry.Authenticate(service_name, r); err != nil {
		switch err {
		case ErrTokenMissing:
			slog.Error("Auth failed", "service_name", service_name, "error", err.Error())
			http.Error(w, "token missing", http.StatusUnauthorized)
			return
		case ErrInvalidToken:
			slog.Error("Auth failed", "service_name", service_name, "error", err.Error())
			http.Error(w, "invalid token", http.StatusUnauthorized)
			return
		}
	}

	slog.Info("Resolving service", "service_name", service_name)

	address := rh.ServiceRegistry.GetAddress(service_name)
	if address == "" {
		slog.Error("Service not found", "service_name", service_name)
		http.Error(w, "service not found", http.StatusNotFound)
		return
	}
	// Create a new uri based on the resolved request
	forward_uri := rh.create_forward_uri(address, route, r.URL.RawQuery)

	slog.Info("Forwarding request", "forward_uri", forward_uri, "service_name", service_name)
	service := rh.ServiceRegistry.GetService(service_name)

	var err error
	// Forward the request with or without circuit breaker
	if rh.circuitBreakerEnabled() {
		err = rh.forward_request_cb(w, r, forward_uri, service.CircuitBreaker, service_name)
	} else {
		err = rh.forward_request(w, r, forward_uri)
	}
	if err != nil {
		slog.Error("Error forwarding request", "error", err.Error(), "service_name", service_name)
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// forward_request forwards the request to the resolved service
func (rh *RequestHandler) forward_request(w http.ResponseWriter, r *http.Request, forward_uri string) error {
	req, err := http.NewRequest(r.Method, forward_uri, r.Body)
	if err != nil {
		return err
	}
	req.Header = cloneHeader(r.Header)

	// TODO: maybe attach claims from request context to the forwarded request context or header

	// add a unique trace id to every request for tracing
	req.Header.Add("X-Trace-Id", uuid.NewString())
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	// Copy the response from the resolved service
	copyResponseHeaders(w, resp)
	w.WriteHeader(resp.StatusCode)
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		return err
	}
	return nil
}

// cloneHeader clones the header
func cloneHeader(h http.Header) http.Header {
	cloned := make(http.Header, len(h))
	for k, v := range h {
		cloned[k] = append([]string(nil), v...)
	}
	return cloned
}

// copyResponseHeaders copies the response headers
func copyResponseHeaders(w http.ResponseWriter, resp *http.Response) {
	for k, v := range resp.Header {
		w.Header()[k] = v
	}
}

// forward_request_cb forwards the request to the resolved service with circuit breaker
func (rh *RequestHandler) forward_request_cb(w http.ResponseWriter, r *http.Request, forwardURI string, cb *CircuitBreaker, service string) error {
	// Define the request execution function
	executeRequest := func() ([]byte, error) {
		// Create a new request
		req, err := http.NewRequest(r.Method, forwardURI, r.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to create new request: %w", err)
		}

		// Copy headers from the original request and add a trace ID
		req.Header = cloneHeader(r.Header)
		req.Header.Add("X-Trace-Id", uuid.NewString())

		// Execute the request
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("request execution failed: %w", err)
		}
		defer resp.Body.Close()

		// Copy response headers and status code
		copyResponseHeaders(w, resp)
		w.WriteHeader(resp.StatusCode)

		// Read the response body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}
		return body, nil
	}

	// Execute the request with the circuit breaker
	body, err := cb.Execute(service, executeRequest)
	if err != nil {
		// Handle the case where the circuit is open and fallback is needed
		if cb.IsOpen() || errors.Is(err, gobreaker.ErrOpenState) {
			return rh.handleFallbackRequest(w, r, service)
		}
		return err
	}

	// Write the response body
	_, err = w.Write(body)
	if err != nil {
		return fmt.Errorf("failed to write response body: %w", err)
	}
	return nil
}

func (rh *RequestHandler) handleFallbackRequest(w http.ResponseWriter, r *http.Request, service string) error {
	slog.Error("Circuit breaker is open, making a fallback request", "service", service)
	fallbackURI := rh.ServiceRegistry.GetFallbackUri(service)
	if fallbackURI == "" {
		slog.Error("Fallback URI not found", "service_name", service)
		return nil
	}

	_, route := rh.resolve_path(r.URL.Path)
	forwardURI := rh.create_forward_uri(fallbackURI, route, r.URL.RawQuery)
	return rh.forward_request(w, r, forwardURI)
}
