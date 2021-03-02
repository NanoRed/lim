package container

import "github.com/lrita/cmap"

// SafeMap goroutine-safe map
type SafeMap struct {
	cmap.Cmap
}

// NewSafeMap create a new safe map
func NewSafeMap() *SafeMap {
	return &SafeMap{}
}

// Store store a key/value
func (m *SafeMap) Store(key, value interface{}) {
	m.Cmap.Store(key, value)
}

// Load load the value corresponding to the key
func (m *SafeMap) Load(key interface{}) (value interface{}, ok bool) {
	return m.Cmap.Load(key)
}

// LoadOrStore returns the existing value for the key if present.
// Otherwise, it stores and returns the given value.
// The loaded result is true if the value was loaded, false if stored.
func (m *SafeMap) LoadOrStore(key, value interface{}) (actual interface{}, loaded bool) {
	return m.Cmap.LoadOrStore(key, value)
}

// Delete delete a key/value by key
func (m *SafeMap) Delete(key interface{}) {
	m.Cmap.Delete(key)
}
