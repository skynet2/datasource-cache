package cache

import (
	"context"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
)

const (
	// DefaultLRUCacheSize is the default size for the LRU cache.
	DefaultLRUCacheSize = 10000
	// DefaultLRUCacheTTL is the default time-to-live for items in the LRU cache.
	DefaultLRUCacheTTL = 1 * time.Hour
	// DefaultChunkSize is the default chunk size for MGet/MSet operations.
	DefaultChunkSize = 100
)

// LRUCache represents an LRU cache provider.
// T is constrained by the Entity interface, V is the type of the original value for a cache key.
type LRUCache[T Entity, V any] struct {
	lru       *expirable.LRU[string, *T]
	chunkSize int
}

// NewLRUCache creates a new LRUCache instance and returns it as a Provider.
// size specifies the maximum number of items the cache can hold.
// If size is 0 or negative, DefaultLRUCacheSize is used.
func NewLRUCache[T Entity, V any](size int) Provider[T, V] {
	if size <= 0 {
		size = DefaultLRUCacheSize
	}

	// expirable.NewLRU currently does not return an error.
	// If the underlying library changes to return an error, it should be handled here.
	lruInstance := expirable.NewLRU[string, *T](size, nil, DefaultLRUCacheTTL)

	return &LRUCache[T, V]{
		lru:       lruInstance,
		chunkSize: DefaultChunkSize,
	}
}

// Get retrieves a value from the cache.
// It matches the Provider interface signature.
func (c *LRUCache[T, V]) Get(ctx context.Context, key *Key[V], requiredModelVersion uint16) (*T, error) {
	// The context parameter is not used by this provider but is part of the interface.
	_ = ctx

	item, found := c.lru.Get(key.Key)
	if !found {
		return nil, nil // Cache miss
	}

	// item is of type *T because LRU is expirable.LRU[string, *T]
	// T is constrained by Entity, so (*item) has GetCacheModelVersion()
	if (*item).GetCacheModelVersion() != requiredModelVersion {
		return nil, nil // Version mismatch, treat as cache miss
	}

	return item, nil
}

// MGet retrieves multiple values from the cache.
// It matches the Provider interface signature.
func (c *LRUCache[T, V]) MGet(ctx context.Context, keys []*Key[V], requiredModelVersion uint16) (map[*Key[V]]*T, []*Key[V], error) {
	// The context parameter is not used by this provider but is part of the interface.
	_ = ctx

	foundItems := make(map[*Key[V]]*T)
	var missingKeys []*Key[V]

	for _, keyEntry := range keys {
		item, found := c.lru.Get(keyEntry.Key)
		if !found {
			missingKeys = append(missingKeys, keyEntry)
			continue
		}

		// item is of type *T
		if (*item).GetCacheModelVersion() != requiredModelVersion {
			missingKeys = append(missingKeys, keyEntry) // Version mismatch
			continue
		}

		foundItems[keyEntry] = item
	}

	return foundItems, missingKeys, nil
}

// MSet stores multiple values in the cache.
// It matches the Provider interface signature.
func (c *LRUCache[T, V]) MSet(ctx context.Context, values map[string]*T, ttl time.Duration) error {
	// The context parameter is not used by this provider but is part of the interface.
	_ = ctx

	for k, v := range values {
		// The Add method of expirable.LRU (from hashicorp/golang-lru/v2/expirable)
		// takes (key, value) and uses the global TTL set during NewLRU.
		// The ttl parameter from MSet is ignored here.
		// It returns a boolean indicating if an item was evicted,
		// but the Provider interface MSet method does not require us to return it.
		_ = ttl // Acknowledge ttl parameter is unused for this specific implementation
		c.lru.Add(k, v)
	}
	return nil
}

// The following methods (Invalidate, Clear, GetType) are commented out
// as they are not part of the Provider[T, V] interface defined in types.go.
// If they are needed for a different interface or extended functionality,
// they can be uncommented and adjusted.

/*
// Invalidate invalidates cache entries based on options
func (c *LRUCache[T, V]) Invalidate(options ...store.InvalidateOption) error {
	// Stubbed implementation
	return nil
}

// Clear clears all cache entries
func (c *LRUCache[T, V]) Clear() error {
	// Stubbed implementation
	return nil
}

// GetType returns the cache type
func (c *LRUCache[T, V]) GetType() string {
	return "lru"
}
*/
