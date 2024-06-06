package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

const (
	// should be loaded from config
	HeartbeatInterval = 10
)

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

func (sr *ServiceRegistry) Heartbeat() {
	for {
		slog.Info("Heartbeating registered services")
		time.Sleep(HeartbeatInterval * time.Second)
		sr.mu.RLock()
		for name, address := range sr.Services {
			// TODO: /health should be replaced with a configurable endpoint
			// provided by the service in registration/config
			resp, err := http.Get("http://" + address + "/health")
			if err != nil {
				slog.Error("Service is down", "name", name, "address", address)
				continue
			}
			if resp.StatusCode != http.StatusOK {
				slog.Error("Service is unhealthy", "name", name, "address", address)
			}
			resp.Body.Close()
		}
		sr.mu.RUnlock()
	}
}
