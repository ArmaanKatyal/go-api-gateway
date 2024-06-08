package main

type CircuitBreaker struct {
	Threshold int
	Count     int
	Open      bool
}

func (cb *CircuitBreaker) IsAllowed() bool {
	if cb.Open {
		return false
	}
	if cb.Count < cb.Threshold {
		cb.Count++
		return true
	}
	cb.Open = true
	return false
}

func (cb *CircuitBreaker) Reset() {
	cb.Count = 0
	cb.Open = false
}

func NewCircuitBreaker() *CircuitBreaker {
	return &CircuitBreaker{}
}
