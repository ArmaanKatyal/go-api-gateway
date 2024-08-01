package main

import (
	"encoding/json"
	"errors"
	"github.com/ArmaanKatyal/go_api_gateway/server/auth"
	"github.com/ArmaanKatyal/go_api_gateway/server/config"
	"github.com/ArmaanKatyal/go_api_gateway/server/feature"
	"github.com/ArmaanKatyal/go_api_gateway/server/observability"
	"log/slog"
	"net"
	"net/http"
	"os"
	"sync"
	"time"
)

type RegisterBody struct {
	// Note: try to keep it consistent with the config.registry.services struct
	Name        string   `json:"name"`
	Address     string   `json:"addr"`
	WhiteList   []string `json:"whitelist"`
	FallbackUri string   `json:"fallbackUri,omitempty"`
	Health      struct {
		Enabled bool   `json:"enabled"`
		Uri     string `json:"uri"`
	}
	Auth *struct {
		Enabled   bool     `json:"enabled"`
		Anonymous bool     `json:"anonymous"`
		Secret    string   `json:"secret"`
		Routes    []string `json:"routes"`
	} `json:"auth,omitempty"`
	Cache *struct {
		Enabled            bool `json:"enabled"`
		ExpirationInterval uint `json:"expirationInterval"`
		CleanupInterval    uint `json:"cleanupInterval"`
	} `json:"cache,omitempty"`
	CircuitBreaker config.CircuitSettings
}

type UpdateBody struct {
	// Note: try to keep it consistent with RegisterBody
	Name        string   `json:"name"`
	Address     string   `json:"addr"`
	WhiteList   []string `json:"whitelist"`
	FallbackUri string   `json:"fallbackUri,omitempty"`
	Health      struct {
		Enabled bool   `json:"enabled"`
		Uri     string `json:"uri"`
	}
	Auth *struct {
		Enabled   bool     `json:"enabled"`
		Anonymous bool     `json:"anonymous"`
		Secret    string   `json:"secret"`
		Routes    []string `json:"routes"`
	} `json:"auth,omitempty"`
	Cache *struct {
		Enabled            bool `json:"enabled"`
		ExpirationInterval uint `json:"expirationInterval"`
		CleanupInterval    uint `json:"cleanupInterval"`
	} `json:"cache,omitempty"`
}

type RegisterResponse struct {
	Message string `json:"message"`
}

type ResponseBody struct {
	Message string `json:"message"`
}

type DeregisterBody struct {
	Name string `json:"name"`
}

type DeregisterResponse struct {
	Message string `json:"message"`
}

// Authenticater Interface for authenticating requests
type Authenticater interface {
	Authenticate(string, *http.Request) auth.AuthError
	IsEnabled() bool
}

// CircuitExecuter Interface for executing circuit breaker
type CircuitExecuter interface {
	Execute(string, func() ([]byte, error)) ([]byte, error)
	IsOpen() bool
	IsEnabled() bool
}

// IPAllower Interface for handling IP whitelist
type IPAllower interface {
	Allowed(string) bool
	GetWhitelist() map[string]bool
	UpdateWhitelist(map[string]bool)
}

type HealthCheck struct {
	Enabled bool   `json:"enabled"`
	Uri     string `json:"uri"`
}

func (h *HealthCheck) IsEnabled() bool {
	return h.Enabled
}

func (h *HealthCheck) GetUri() string {
	return h.Uri
}

func NewHealthCheck(enabled bool, uri string) *HealthCheck {
	return &HealthCheck{
		Enabled: enabled,
		Uri:     uri,
	}
}

type Service struct {
	Addr           string `json:"addr"`
	FallbackUri    string `json:"fallbackUri"`
	Health         *HealthCheck
	IPWhiteList    IPAllower       `json:"ipWhitelist"`
	CircuitBreaker CircuitExecuter `json:"circuitBreaker"`
	Auth           Authenticater   `json:"auth"`
	Cache          Cacher          `json:"cache"`
	mu             sync.Mutex
}

func (s *Service) IsAuthEnabled() bool {
	return s.Auth.IsEnabled()
}

type ServiceRegistry struct {
	mu       sync.RWMutex
	Metrics  *observability.PromMetrics
	Services map[string]*Service `json:"services"`
}

// Register registers a service with the registry
func (sr *ServiceRegistry) Register(name string, s *Service) {
	slog.Info("Registering service", "name", name, "address", s.Addr)
	sr.mu.Lock()
	defer sr.mu.Unlock()
	if _, ok := sr.Services[name]; ok {
		slog.Error("service already exists", "name", name)
	}
	sr.Services[name] = s
}

// Update updates a service in the registry
func (sr *ServiceRegistry) Update(name string, updated *Service) {
	slog.Info("Updating registered service", "name", name)
	sr.mu.Lock()
	defer sr.mu.Unlock()
	if _, ok := sr.Services[name]; ok {
		sr.Services[name] = updated
	}
}

// Deregister removes a service from the registry
func (sr *ServiceRegistry) Deregister(name string) {
	slog.Info("Deregistering service", "name", name)
	sr.mu.Lock()
	defer sr.mu.Unlock()
	delete(sr.Services, name)
}

// GetAddress returns the address of the service with the given name
func (sr *ServiceRegistry) GetAddress(name string) string {
	s := sr.GetService(name)
	if s == nil {
		return ""
	}
	return s.Addr
}

// GetService returns the service with the given name
func (sr *ServiceRegistry) GetService(name string) *Service {
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	if v, ok := sr.Services[name]; ok {
		return v
	}
	return nil
}

// GetFallbackUri returns the fallback uri of the service with the given name
func (sr *ServiceRegistry) GetFallbackUri(name string) string {
	s := sr.GetService(name)
	if s == nil {
		return ""
	}
	return s.FallbackUri
}

// IsWhitelisted checks if the ip is allowed to access the service
func (sr *ServiceRegistry) IsWhitelisted(name string, addr string) (bool, error) {
	ip, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false, err
	}
	s := sr.GetService(name)
	if s == nil {
		return false, errors.New("service not found")
	}
	return s.IPWhiteList.Allowed(ip), nil
}

func (sr *ServiceRegistry) Authenticate(name string, r *http.Request) error {
	s := sr.GetService(name)
	if s == nil {
		return errors.New("service not found")
	}
	return s.Auth.Authenticate(name, r)
}

// populateRegistryServices populates the service registry with the services in the configuration
func populateRegistryServices(sr *ServiceRegistry) {
	slog.Info("Populating registry services")
	for _, v := range config.AppConfig.Registry.Services {
		w := feature.NewIPWhiteList()
		feature.PopulateIPWhiteList(w, v.WhiteList)
		// Note: new fields for service in the config must be added here
		file, err := os.Open(v.Auth.Secret)
		if err != nil {
			slog.Error("failed to read service secret", "service", v.Name, "path", v.Auth.Secret)
		}
		sr.Services[v.Name] = &Service{
			Addr:           v.Addr,
			FallbackUri:    v.FallbackUri,
			Health:         NewHealthCheck(v.Health.Enabled, v.Health.Uri),
			IPWhiteList:    w,
			CircuitBreaker: feature.NewCircuitBreaker(v.Name, v.CircuitBreaker),
			Auth:           auth.NewJwtAuth(v.Auth.Enabled, v.Auth.Anonymous, v.Auth.Routes, file),
			Cache:          feature.NewCacheHandler(v.Cache.Enabled, v.Cache.ExpirationInterval, v.Cache.CleanupInterval),
		}
	}
}

func NewServiceRegistry(metrics *observability.PromMetrics) *ServiceRegistry {
	r := ServiceRegistry{
		Services: make(map[string]*Service),
		Metrics:  metrics,
	}
	populateRegistryServices(&r)
	return &r
}

// RegisterService registers a service with the registry
func (sr *ServiceRegistry) RegisterService(w http.ResponseWriter, r *http.Request) {
	slog.Info("Registering service", "req", RequestToMap(r))
	var rb RegisterBody
	err := json.NewDecoder(r.Body).Decode(&rb)
	if err != nil {
		slog.Error("Error decoding request", "error", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// TODO: do a schema validation before actually adding the service. duh ¯\_(ツ)_/¯

	wl := feature.NewIPWhiteList()
	feature.PopulateIPWhiteList(wl, rb.WhiteList)

	var na *auth.JwtAuth
	if rb.Auth != nil {
		file, err := os.Open(rb.Auth.Secret)
		if err != nil {
			slog.Error("failed to read service secret", "service", rb.Name, "path", rb.Auth.Secret)
		}
		na = auth.NewJwtAuth(rb.Auth.Enabled, rb.Auth.Anonymous, rb.Auth.Routes, file)
	} else {
		na = auth.NewJwtAuth(false, false, []string{}, nil)
	}

	sr.Register(rb.Name, &Service{
		Addr:           rb.Address,
		FallbackUri:    rb.FallbackUri,
		IPWhiteList:    wl,
		CircuitBreaker: feature.NewCircuitBreaker(rb.Name, rb.CircuitBreaker),
		Auth:           na,
		Cache:          feature.NewCacheHandler(rb.Cache.Enabled, rb.Cache.ExpirationInterval, rb.Cache.CleanupInterval),
	})
	j, err := json.Marshal(RegisterResponse{Message: "service " + rb.Name + " registered"})
	if err != nil {
		slog.Error("Error marshalling response", "error", err.Error())
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(j); err != nil {
		slog.Error("Error writing response", "error", err.Error())
	}
}

// UpdateService updates an existing service in the registry
func (sr *ServiceRegistry) UpdateService(w http.ResponseWriter, r *http.Request) {
	slog.Info("Updating service", "req", RequestToMap(r))
	// TODO: only provide the fields that need to be updated, instead of the whole schema
	var ub UpdateBody
	err := json.NewDecoder(r.Body).Decode(&ub)
	if err != nil {
		slog.Error("Error decoding request", "error", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// TODO: do a schema validation before actually changing something. duh ¯\_(ツ)_/¯

	s := sr.GetService(ub.Name)
	if s == nil {
		slog.Error("Defined service doesn't exists")
		http.Error(w, "service doesn't exists", http.StatusBadRequest)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// modify the address
	s.Addr = ub.Address
	// add the new whitelisted ip
	existingLists := s.IPWhiteList.GetWhitelist()
	for _, v := range ub.WhiteList {
		existingLists[v] = true
	}
	s.IPWhiteList.UpdateWhitelist(existingLists)
	s.FallbackUri = ub.FallbackUri

	// Update auth
	var na *auth.JwtAuth
	if ub.Auth != nil {
		file, err := os.Open(ub.Auth.Secret)
		if err != nil {
			slog.Error("failed to read service secret", "service", ub.Name, "path", ub.Auth.Secret)
		}
		na = auth.NewJwtAuth(ub.Auth.Enabled, ub.Auth.Anonymous, ub.Auth.Routes, file)
	} else {
		na = auth.NewJwtAuth(false, false, []string{}, nil)
	}
	s.Auth = na

	// Update cache
	var ch *feature.CacheHandler
	if ub.Cache != nil {
		ch = feature.NewCacheHandler(ub.Cache.Enabled, ub.Cache.ExpirationInterval, ub.Cache.CleanupInterval)
	} else {
		ch = feature.NewCacheHandler(false, 0, 0)
	}
	s.Cache = ch

	// Update the service in the registry
	sr.Update(ub.Name, s)

	j, err := json.Marshal(ResponseBody{Message: "service " + ub.Name + " updated"})
	if err != nil {
		slog.Error("Error marshalling response", "error", err.Error(), "service", ub.Name)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(j); err != nil {
		slog.Error("Error writing response", "error", err.Error())
	}
}

// DeregisterService deregisters a service from the registry
func (sr *ServiceRegistry) DeregisterService(w http.ResponseWriter, r *http.Request) {
	slog.Info("Deregistering service", "req", RequestToMap(r))
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
	if _, err := w.Write(j); err != nil {
		slog.Error("Error writing response", "error", err.Error())
	}
}

// GetServices returns the registered services
func (sr *ServiceRegistry) GetServices(w http.ResponseWriter, r *http.Request) {
	slog.Info("Retrieved registered services", "req", RequestToMap(r))
	j, err := json.Marshal(sr.Services)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(j); err != nil {
		slog.Error("Error writing response", "error", err.Error())
	}
}

// Heartbeat checks the health of the registered services
func (sr *ServiceRegistry) Heartbeat() {
	for {
		time.Sleep(time.Duration(config.AppConfig.Registry.HeartbeatInterval) * time.Second)
		sr.mu.RLock()
		slog.Info("Heartbeat registered services")
		for name, v := range sr.Services {
			if v.Health.IsEnabled() {
				resp, err := http.Get("http://" + v.Addr + v.Health.GetUri())
				if err != nil {
					slog.Error("Service is down", "name", name, "address", v.Addr)
					continue
				}
				if resp.StatusCode != http.StatusOK {
					slog.Warn("Service is unhealthy", "name", name, "address", v.Addr)
				}
				_ = resp.Body.Close()
			}
		}
		sr.mu.RUnlock()
	}
}

type Cacher interface {
	Get(string) (interface{}, bool)
	Set(string, interface{})
	IsEnabled() bool
}

func (sr *ServiceRegistry) GetCache(name string, key string) (interface{}, bool) {
	s := sr.GetService(name)
	if s == nil {
		return nil, false
	}
	return s.Cache.Get(key)
}

func (sr *ServiceRegistry) SetCache(name string, key string, value interface{}) bool {
	s := sr.GetService(name)
	if s == nil {
		return false
	}
	s.Cache.Set(key, value)
	return true
}

func (sr *ServiceRegistry) IsCacheEnabled(name string) bool {
	s := sr.GetService(name)
	if s == nil {
		return false
	}
	return s.Cache.IsEnabled()
}
