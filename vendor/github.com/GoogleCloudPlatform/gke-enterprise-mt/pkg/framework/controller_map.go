package framework

import "sync"

// ControllerSet holds controller-specific resources for a ProviderConfig.
// It contains the stop channel used to signal controller shutdown.
type ControllerSet struct {
	stopCh chan<- struct{}
}

// ControllerMap is a thread-safe map for storing ControllerSet instances.
// It uses read-write locking to allow concurrent read operations.
type ControllerMap struct {
	mu   sync.RWMutex
	data map[string]*ControllerSet
}

// NewControllerMap creates a new thread-safe ControllerMap.
func NewControllerMap() *ControllerMap {
	return &ControllerMap{
		data: make(map[string]*ControllerSet),
	}
}

// Get retrieves a ControllerSet for the given key.
// Returns the ControllerSet and a boolean indicating whether it exists.
func (cm *ControllerMap) Get(key string) (*ControllerSet, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	cs, exists := cm.data[key]
	return cs, exists
}

// GetOrCreate retrieves the ControllerSet for the given key, creating a new entry when absent.
// The second return value indicates whether the ControllerSet already existed.
func (cm *ControllerMap) GetOrCreate(key string) (*ControllerSet, bool) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	if cs, exists := cm.data[key]; exists {
		return cs, true
	}
	cs := &ControllerSet{}
	cm.data[key] = cs
	return cs, false
}

// Delete removes the ControllerSet for the given key.
func (cm *ControllerMap) Delete(key string) {
	cm.mu.Lock()
	defer cm.mu.Unlock()
	delete(cm.data, key)
}
