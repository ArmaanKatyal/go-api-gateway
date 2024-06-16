package main

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sony/gobreaker/v2"
)

type RequestHandler struct {
	ServiceRegistry *ServiceRegistry
	RateLimiter     *RateLimiter
	Metrics         *PromMetrics
}

func NewRequestHandler() *RequestHandler {
	m := NewPromMetrics()
	return &RequestHandler{
		ServiceRegistry: NewServiceRegistry(m),
		RateLimiter:     NewRateLimiter(m),
		Metrics:         m,
	}
}

// RequestToMap converts the request to a map
func RequestToMap(r *http.Request) map[string]interface{} {
	result := make(map[string]interface{})

	result["method"] = r.Method

	result["url"] = r.URL.String()

	// Use the first value for each header, query parameter, and form field
	headers := make(map[string]string)
	for name, values := range r.Header {
		headers[name] = values[0]
	}
	result["headers"] = headers

	queryParams := make(map[string]string)
	for name, values := range r.URL.Query() {
		queryParams[name] = values[0]
	}
	result["query_params"] = queryParams

	if err := r.ParseForm(); err == nil {
		formValues := make(map[string]string)
		for name, values := range r.Form {
			formValues[name] = values[0]
		}
		result["form_values"] = formValues
	}

	return result
}

func GetStatusCode(statusCode int) string {
	return http.StatusText(statusCode)
}

// health is a simple health check endpoint
func Health(w http.ResponseWriter, r *http.Request) {
	slog.Info("Health check", "req", RequestToMap(r))
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("OK")); err != nil {
		slog.Error("Error writing response", "error", err.Error())
	}
}

// config returns the application configuration
func Config(w http.ResponseWriter, r *http.Request) {
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
	mux.HandleFunc("POST /services/register", r.ServiceRegistry.RegisterService)
	mux.HandleFunc("POST /services/deregister", r.ServiceRegistry.DeregisterService)
	mux.HandleFunc("GET /services", r.ServiceRegistry.GetServices)
	mux.HandleFunc("POST /services/update", r.ServiceRegistry.UpdateService)
	mux.HandleFunc("GET /health", Health)
	mux.HandleFunc("GET /config", Config)
	mux.HandleFunc("/", r.HandleRequest)
	mux.Handle("GET /metrics", promhttp.Handler())
	return mux
}

func (rh *RequestHandler) circuitBreakerEnabled() bool {
	return AppConfig.Server.CircuitBreaker.Enabled
}

func (rh *RequestHandler) rateLimiterEnabled() bool {
	return AppConfig.RateLimiter.Enabled
}

func (rh *RequestHandler) CollectMetrics(input *MetricsInput, t time.Time) {
	rh.Metrics.Collect(input, t)
}

// resolvePath splits the path into service name and route path
func (rh *RequestHandler) resolvePath(path string) (string, []string) {
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return path, nil
	}
	return parts[1], parts[2:]
}

// createForwardURI creates a new uri based on the resolved request
func (rh *RequestHandler) createForwardURI(address string, route []string, query string) string {
	if !strings.HasPrefix(address, "http://") && !strings.HasPrefix(address, "https://") {
		address = "http://" + address
	}
	forward_uri := address + "/" + strings.Join(route, "/")
	if query != "" {
		forward_uri = forward_uri + "?" + query
	}
	return forward_uri
}

// HandleRequest handles the incoming request and forwards it to the resolved service
func (rh *RequestHandler) HandleRequest(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	slog.Info("Received request", "req", RequestToMap(r))
	service_name, route := rh.resolvePath(r.URL.Path)
	if ok, err := rh.ServiceRegistry.IsWhitelisted(service_name, r.RemoteAddr); !ok || err != nil {
		slog.Error("Unauthorized request", "path", r.URL.Path, "method", r.Method, "ip", r.RemoteAddr, "service_name", service_name)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		rh.CollectMetrics(&MetricsInput{Code: GetStatusCode(http.StatusUnauthorized), Method: r.Method, Route: r.URL.String()}, start)
		return
	}
	if rh.rateLimiterEnabled() && !rh.RateLimiter.Allow(r.RemoteAddr) {
		slog.Error("Rate limit exceeded", "path", r.URL.Path, "method", r.Method, "ip", r.RemoteAddr, "service_name", service_name)
		http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
		rh.CollectMetrics(&MetricsInput{Code: GetStatusCode(http.StatusTooManyRequests), Method: r.Method, Route: r.URL.String()}, start)
		return
	}

	if err := rh.ServiceRegistry.Authenticate(service_name, r); err != nil {
		// If Auth fails reject the request with an appropriate message and status code
		switch err {
		case ErrTokenMissing:
			slog.Error("Auth failed", "service_name", service_name, "error", err.Error())
			http.Error(w, "token missing", http.StatusUnauthorized)
			rh.CollectMetrics(&MetricsInput{Code: GetStatusCode(http.StatusUnauthorized), Method: r.Method, Route: r.URL.String()}, start)
			return
		case ErrInvalidToken:
			slog.Error("Auth failed", "service_name", service_name, "error", err.Error())
			http.Error(w, "invalid token", http.StatusUnauthorized)
			rh.CollectMetrics(&MetricsInput{Code: GetStatusCode(http.StatusUnauthorized), Method: r.Method, Route: r.URL.String()}, start)
			return
		default:
			slog.Error("Auth failed", "service_name", service_name, "error", err.Error())
			http.Error(w, "auth failed", http.StatusUnauthorized)
			rh.CollectMetrics(&MetricsInput{Code: GetStatusCode(http.StatusUnauthorized), Method: r.Method, Route: r.URL.String()}, start)
			return
		}
	}

	slog.Info("Resolving service", "service_name", service_name)

	address := rh.ServiceRegistry.GetAddress(service_name)
	if address == "" {
		slog.Error("Service not found", "service_name", service_name)
		http.Error(w, "service not found", http.StatusNotFound)
		rh.CollectMetrics(&MetricsInput{Code: GetStatusCode(http.StatusNotFound), Method: r.Method, Route: r.URL.String()}, start)
		return
	}
	// Create a new uri based on the resolved request
	forward_uri := rh.createForwardURI(address, route, r.URL.RawQuery)

	slog.Info("Forwarding request", "forward_uri", forward_uri, "service_name", service_name)
	service := rh.ServiceRegistry.GetService(service_name)

	var err error
	// Forward the request with or without circuit breaker
	if rh.circuitBreakerEnabled() {
		err = rh.forwardRequestCB(w, r, forward_uri, service.CircuitBreaker, service_name, start)
	} else {
		err = rh.forwardRequest(w, r, forward_uri, start)
	}
	if err != nil {
		slog.Error("Error forwarding request", "error", err.Error(), "service_name", service_name)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		rh.CollectMetrics(&MetricsInput{Code: GetStatusCode(http.StatusInternalServerError), Method: r.Method, Route: r.URL.String()}, start)
	}
}

// forwardRequest forwards the request to the resolved service
func (rh *RequestHandler) forwardRequest(w http.ResponseWriter, r *http.Request, forward_uri string, t time.Time) error {
	req, err := http.NewRequest(r.Method, forward_uri, r.Body)
	if err != nil {
		rh.CollectMetrics(&MetricsInput{Code: GetStatusCode(http.StatusInternalServerError), Method: r.Method, Route: r.URL.String()}, t)
		return err
	}
	req.Header = cloneHeader(r.Header)

	// add a unique trace id to every request for tracing
	req.Header.Add("X-Trace-Id", uuid.NewString())
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		rh.CollectMetrics(&MetricsInput{Code: GetStatusCode(http.StatusInternalServerError), Method: r.Method, Route: r.URL.String()}, t)
		return err
	}
	defer resp.Body.Close()
	// Copy the response from the resolved service
	copyResponseHeaders(w, resp)
	w.WriteHeader(resp.StatusCode)
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		rh.CollectMetrics(&MetricsInput{Code: GetStatusCode(http.StatusInternalServerError), Method: r.Method, Route: r.URL.String()}, t)
		return err
	}
	rh.CollectMetrics(&MetricsInput{Code: GetStatusCode(resp.StatusCode), Method: r.Method, Route: r.URL.String()}, t)
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

// forwardRequestCB forwards the request to the resolved service with circuit breaker
func (rh *RequestHandler) forwardRequestCB(w http.ResponseWriter, r *http.Request, forwardURI string, cb CircuitExecuter, service string, t time.Time) error {
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
			return rh.handleFallbackRequest(w, r, service, t)
		}
		return err
	}

	// Write the response body
	_, err = w.Write(body)
	if err != nil {
		return fmt.Errorf("failed to write response body: %w", err)
	}
	rh.CollectMetrics(&MetricsInput{Code: GetStatusCode(http.StatusOK), Method: r.Method, Route: r.URL.String()}, t)
	return nil
}

// handleFallbackRequest handles the case where the circuit breaker is open and a fallback request is needed
func (rh *RequestHandler) handleFallbackRequest(w http.ResponseWriter, r *http.Request, service string, t time.Time) error {
	slog.Error("Circuit breaker is open, making a fallback request", "service", service)
	fallbackURI := rh.ServiceRegistry.GetFallbackUri(service)
	if fallbackURI == "" {
		slog.Error("Fallback URI not found", "service_name", service)
		http.Error(w, "fallback uri not found", http.StatusNotFound)
		rh.CollectMetrics(&MetricsInput{Code: GetStatusCode(http.StatusNotFound), Method: r.Method, Route: r.URL.String()}, t)
		return nil
	}

	// Resolve the path and create a new URI
	_, route := rh.resolvePath(r.URL.Path)
	forwardURI := rh.createForwardURI(fallbackURI, route, r.URL.RawQuery)
	// Forward the request
	return rh.forwardRequest(w, r, forwardURI, t)
}
