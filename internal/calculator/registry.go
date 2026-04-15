package calculator

import (
	"fmt"
	"sync"
)

// Registry manages all registered prediction strategies.
type Registry struct {
	mu         sync.RWMutex
	strategies map[string]Strategy
}

// NewRegistry creates an empty Registry.
func NewRegistry() *Registry {
	return &Registry{
		strategies: make(map[string]Strategy),
	}
}

// Register adds a strategy to the registry.
// Returns an error if a strategy with the same ID is already registered.
func (r *Registry) Register(s Strategy) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.strategies[s.ID()]; exists {
		return fmt.Errorf("strategy %q already registered", s.ID())
	}
	r.strategies[s.ID()] = s
	return nil
}

// Get returns the strategy registered under id.
// Returns (nil, false) if no strategy is found.
func (r *Registry) Get(id string) (Strategy, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.strategies[id]
	return s, ok
}

// All returns all registered strategies. Order is not guaranteed.
func (r *Registry) All() []Strategy {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Strategy, 0, len(r.strategies))
	for _, s := range r.strategies {
		result = append(result, s)
	}
	return result
}
