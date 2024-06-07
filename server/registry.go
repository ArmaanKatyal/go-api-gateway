package main

import (
	"encoding/json"
	"log/slog"
	"net"
	"net/http"
	"sync"
	"time"
)

type RegisterBody struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}

type UpdateBody struct {
	Name      string   `json:"name"`
	Addr      string   `json:"addr"`
	WhiteList []string `json:"whitelist"`
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

type Service struct {
	Addr      string       `json:"addr"`
	WhiteList *IPWhiteList `json:"ipWhiteList"`
}

type ServiceRegistry struct {
	mu       sync.RWMutex
	Services map[string]Service `json:"services"`
}

func (sr *ServiceRegistry) Register(name string, address string) {
	slog.Info("Registering service", "name", name, "address", address)
	sr.mu.Lock()
	defer sr.mu.Unlock()
	if _, ok := sr.Services[name]; ok {
		slog.Error("service already exists", "name", name)
	}
	sr.Services[name] = Service{
		Addr:      address,
		WhiteList: NewIPWhiteList(),
	}
}

func (sr *ServiceRegistry) Update(name string, updated *Service) {
	slog.Info("Updating registered service", "name", name)
	sr.mu.Lock()
	defer sr.mu.Unlock()
	if _, ok := sr.Services[name]; ok {
		sr.Services[name] = Service{
			Addr:      updated.Addr,
			WhiteList: updated.WhiteList,
		}
	}
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
	return val.Addr
}

func (sr *ServiceRegistry) GetService(name string) *Service {
	if v, ok := sr.Services[name]; ok {
		return &v
	}
	return nil
}

// CheckWhiteList checks if the ip is allowed to access the service
func (sr *ServiceRegistry) IsWhitelisted(name string, addr string) (bool, error) {
	ip, _, err := net.SplitHostPort(addr)
	if err != nil {
		return false, err
	}
	sr.mu.RLock()
	defer sr.mu.RUnlock()
	val, ok := sr.Services[name]
	if !ok {
		return false, nil
	}
	return val.WhiteList.Allowed(ip), nil
}

// populateRegistryServices populates the service registry with the services in the configuration
func populateRegistryServices(sr *ServiceRegistry) {
	for _, v := range AppConfig.Registry.Services {
		w := NewIPWhiteList()
		populateWhiteList(w, v.WhiteList)
		sr.Services[v.Name] = Service{
			Addr:      v.Addr,
			WhiteList: w,
		}
	}
}

func NewServiceRegistry() *ServiceRegistry {
	r := ServiceRegistry{
		Services: make(map[string]Service),
	}
	populateRegistryServices(&r)
	return &r
}

// Register_service registers a service with the registry
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
	if _, err := w.Write(j); err != nil {
		slog.Error("Error writing response", "error", err.Error())
	}
}

// Update_service updates a existing service in the registry
func (sr *ServiceRegistry) Update_service(w http.ResponseWriter, r *http.Request) {
	var ub UpdateBody
	err := json.NewDecoder(r.Body).Decode(&ub)
	if err != nil {
		slog.Error("Error decoding request", "error", err.Error())
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s := sr.GetService(ub.Name)
	if s == nil {
		slog.Error("Defined service doesn't exists")
		http.Error(w, "service doesn't exists", http.StatusBadRequest)
		return
	}
	// modify the address
	s.Addr = ub.Addr
	// add the new whitelisted ip
	for _, v := range ub.WhiteList {
		s.WhiteList.Whitelist[v] = true
	}
	sr.Update(ub.Name, s)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte("service updated successfully")); err != nil {
		slog.Error("Error writing response", "error", err.Error())
	}
}

// Deregister_service deregisters a service from the registry
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
	if _, err := w.Write(j); err != nil {
		slog.Error("Error writing response", "error", err.Error())
	}
}

// Get_services returns the registered services
func (sr *ServiceRegistry) Get_services(w http.ResponseWriter, r *http.Request) {
	slog.Info("Retrieved registered services")
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
		time.Sleep(time.Duration(AppConfig.Registry.HeartbeatInterval) * time.Second)
		sr.mu.RLock()
		slog.Info("Heartbeating registered services")
		for name, prop := range sr.Services {
			// TODO: /health should be replaced with a configurable endpoint
			// provided by the service in registration/config
			resp, err := http.Get("http://" + prop.Addr + "/health")
			if err != nil {
				slog.Error("Service is down", "name", name, "address", prop.Addr)
				continue
			}
			if resp.StatusCode != http.StatusOK {
				slog.Error("Service is unhealthy", "name", name, "address", prop.Addr)
			}
			resp.Body.Close()
		}
		sr.mu.RUnlock()
	}
}
