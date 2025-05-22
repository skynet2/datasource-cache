package cache

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// EntityToCache is a sample struct for testing cache implementations.
// Copied from cache_one_level_test.go for use in LRUCache tests.
type EntityToCache struct {
	Id           int
	Value        string
	ModelVersion uint16
}

// GetCacheModelVersion returns the model version of the entity.
func (e *EntityToCache) GetCacheModelVersion() uint16 {
	return e.ModelVersion
}

const (
	testModelVersion      = uint16(1)
	anotherModelVersion   = uint16(2)
	defaultTestCacheSize  = 10
	shortTTL              = 50 * time.Millisecond
	longerThanShortTTL    = 100 * time.Millisecond
	standardTestTTL       = 1 * time.Hour
)

// Helper to create a new LRU cache provider for tests
func newTestLRUCacheProvider[T Entity, V any](size int) Provider[T, V] {
	return NewLRUCache[T, V](size)
}

// TestLRUCache_Get_Miss tests getting a non-existent key.
func TestLRUCache_Get_Miss(t *testing.T) {
	ctx := context.Background()
	cache := newTestLRUCacheProvider[EntityToCache, int](defaultTestCacheSize)

	key := &Key[int]{Key: "miss_key", OriginalValue: 1}

	item, err := cache.Get(ctx, key, testModelVersion)

	assert.Nil(t, err)
	assert.Nil(t, item)
}

// TestLRUCache_Get_Hit tests getting an existing key.
func TestLRUCache_Get_Hit(t *testing.T) {
	ctx := context.Background()
	cache := newTestLRUCacheProvider[EntityToCache, int](defaultTestCacheSize)

	entity := &EntityToCache{Id: 1, Value: "data", ModelVersion: testModelVersion}
	key := &Key[int]{Key: "hit_key", OriginalValue: entity.Id}

	err := cache.MSet(ctx, map[string]*EntityToCache{key.Key: entity}, standardTestTTL)
	assert.Nil(t, err)

	item, err := cache.Get(ctx, key, testModelVersion)

	assert.Nil(t, err)
	assert.NotNil(t, item)
	assert.Equal(t, entity, item)
}

// TestLRUCache_Get_VersionMismatch tests getting a key with the wrong model version.
func TestLRUCache_Get_VersionMismatch(t *testing.T) {
	ctx := context.Background()
	cache := newTestLRUCacheProvider[EntityToCache, int](defaultTestCacheSize)

	entity := &EntityToCache{Id: 1, Value: "data", ModelVersion: testModelVersion}
	key := &Key[int]{Key: "version_mismatch_key", OriginalValue: entity.Id}

	err := cache.MSet(ctx, map[string]*EntityToCache{key.Key: entity}, standardTestTTL)
	assert.Nil(t, err)

	item, err := cache.Get(ctx, key, anotherModelVersion)

	assert.Nil(t, err)
	assert.Nil(t, item)
}

// TestLRUCache_MGet_HitAndMiss tests MGet with a mix of existing and non-existing keys.
func TestLRUCache_MGet_HitAndMiss(t *testing.T) {
	ctx := context.Background()
	cache := newTestLRUCacheProvider[EntityToCache, int](defaultTestCacheSize)

	entity1 := &EntityToCache{Id: 1, Value: "data1", ModelVersion: testModelVersion}
	key1 := &Key[int]{Key: "mget_key1", OriginalValue: entity1.Id}

	entity2 := &EntityToCache{Id: 2, Value: "data2", ModelVersion: testModelVersion}
	key2 := &Key[int]{Key: "mget_key2", OriginalValue: entity2.Id}

	key3 := &Key[int]{Key: "mget_key_miss", OriginalValue: 3} // This key won't be in cache

	err := cache.MSet(ctx, map[string]*EntityToCache{
		key1.Key: entity1,
		key2.Key: entity2,
	}, standardTestTTL)
	assert.Nil(t, err)

	keysToGet := []*Key[int]{key1, key2, key3}
	foundItems, missingKeys, err := cache.MGet(ctx, keysToGet, testModelVersion)

	assert.Nil(t, err)
	assert.NotNil(t, foundItems)
	assert.Len(t, foundItems, 2)
	assert.Contains(t, foundItems, key1)
	assert.Equal(t, entity1, foundItems[key1])
	assert.Contains(t, foundItems, key2)
	assert.Equal(t, entity2, foundItems[key2])

	assert.NotNil(t, missingKeys)
	assert.Len(t, missingKeys, 1)
	assert.Contains(t, missingKeys, key3)
}

// TestLRUCache_MGet_VersionMismatch tests MGet where all keys have version mismatches.
func TestLRUCache_MGet_VersionMismatch(t *testing.T) {
	ctx := context.Background()
	cache := newTestLRUCacheProvider[EntityToCache, int](defaultTestCacheSize)

	entityA := &EntityToCache{Id: 10, Value: "dataA", ModelVersion: testModelVersion}
	keyA := &Key[int]{Key: "mget_v_mismatch_A", OriginalValue: entityA.Id}

	entityB := &EntityToCache{Id: 11, Value: "dataB", ModelVersion: testModelVersion}
	keyB := &Key[int]{Key: "mget_v_mismatch_B", OriginalValue: entityB.Id}

	err := cache.MSet(ctx, map[string]*EntityToCache{
		keyA.Key: entityA,
		keyB.Key: entityB,
	}, standardTestTTL)
	assert.Nil(t, err)

	keysToGet := []*Key[int]{keyA, keyB}
	foundItems, missingKeys, err := cache.MGet(ctx, keysToGet, anotherModelVersion)

	assert.Nil(t, err)
	assert.NotNil(t, foundItems)
	assert.Len(t, foundItems, 0)

	assert.NotNil(t, missingKeys)
	assert.Len(t, missingKeys, 2)
	assert.Contains(t, missingKeys, keyA)
	assert.Contains(t, missingKeys, keyB)
}

// TestLRUCache_MGet_PartialVersionMismatch tests MGet with some keys having version mismatches.
func TestLRUCache_MGet_PartialVersionMismatch(t *testing.T) {
	ctx := context.Background()
	cache := newTestLRUCacheProvider[EntityToCache, int](defaultTestCacheSize)

	entityX := &EntityToCache{Id: 20, Value: "dataX", ModelVersion: testModelVersion} // Will match
	keyX := &Key[int]{Key: "mget_partial_X", OriginalValue: entityX.Id}

	entityY := &EntityToCache{Id: 21, Value: "dataY", ModelVersion: anotherModelVersion} // Will mismatch
	keyY := &Key[int]{Key: "mget_partial_Y", OriginalValue: entityY.Id}

	err := cache.MSet(ctx, map[string]*EntityToCache{
		keyX.Key: entityX,
		keyY.Key: entityY,
	}, standardTestTTL)
	assert.Nil(t, err)

	keysToGet := []*Key[int]{keyX, keyY}
	foundItems, missingKeys, err := cache.MGet(ctx, keysToGet, testModelVersion)

	assert.Nil(t, err)
	assert.NotNil(t, foundItems)
	assert.Len(t, foundItems, 1)
	assert.Contains(t, foundItems, keyX)
	assert.Equal(t, entityX, foundItems[keyX])

	assert.NotNil(t, missingKeys)
	assert.Len(t, missingKeys, 1)
	assert.Contains(t, missingKeys, keyY)
}

// TestLRUCache_MSet_And_Expiration tests item expiration.
func TestLRUCache_MSet_And_Expiration(t *testing.T) {
	ctx := context.Background()
	cache := newTestLRUCacheProvider[EntityToCache, int](defaultTestCacheSize)

	entity := &EntityToCache{Id: 100, Value: "expire_me", ModelVersion: testModelVersion}
	key := &Key[int]{Key: "expire_key", OriginalValue: entity.Id}

	err := cache.MSet(ctx, map[string]*EntityToCache{key.Key: entity}, shortTTL)
	assert.Nil(t, err)

	// Check immediately, should be there
	item, err := cache.Get(ctx, key, testModelVersion)
	assert.Nil(t, err)
	assert.NotNil(t, item)
	assert.Equal(t, entity, item)

	// Wait for longer than TTL
	time.Sleep(longerThanShortTTL)

	// Check again, should be gone
	item, err = cache.Get(ctx, key, testModelVersion)
	assert.Nil(t, err)
	assert.Nil(t, item)
}

// TestLRUCache_Eviction tests cache eviction based on size.
func TestLRUCache_Eviction(t *testing.T) {
	ctx := context.Background()
	cache := newTestLRUCacheProvider[EntityToCache, int](1) // Cache with size 1

	entity1 := &EntityToCache{Id: 1, Value: "item1", ModelVersion: testModelVersion}
	key1 := &Key[int]{Key: "evict_key1", OriginalValue: entity1.Id}

	entity2 := &EntityToCache{Id: 2, Value: "item2", ModelVersion: testModelVersion}
	key2 := &Key[int]{Key: "evict_key2", OriginalValue: entity2.Id}

	// Set item1
	err := cache.MSet(ctx, map[string]*EntityToCache{key1.Key: entity1}, standardTestTTL)
	assert.Nil(t, err)

	// Get item1, should be there
	item, err := cache.Get(ctx, key1, testModelVersion)
	assert.Nil(t, err)
	assert.NotNil(t, item)
	assert.Equal(t, entity1, item)

	// Set item2, this should evict item1
	err = cache.MSet(ctx, map[string]*EntityToCache{key2.Key: entity2}, standardTestTTL)
	assert.Nil(t, err)

	// Try to get item1, should be nil (evicted)
	item, err = cache.Get(ctx, key1, testModelVersion)
	assert.Nil(t, err)
	assert.Nil(t, item)

	// Get item2, should be there
	item, err = cache.Get(ctx, key2, testModelVersion)
	assert.Nil(t, err)
	assert.NotNil(t, item)
	assert.Equal(t, entity2, item)

	// Test with a slightly larger cache to ensure it's not just replacing the most recent
	cacheSize2 := newTestLRUCacheProvider[EntityToCache, int](2)
	entity3 := &EntityToCache{Id: 3, Value: "item3", ModelVersion: testModelVersion}
	key3 := &Key[int]{Key: "evict_key3", OriginalValue: entity3.Id}

	// Fill the cache
	err = cacheSize2.MSet(ctx, map[string]*EntityToCache{key1.Key: entity1}, standardTestTTL)
	assert.Nil(t, err)
	err = cacheSize2.MSet(ctx, map[string]*EntityToCache{key2.Key: entity2}, standardTestTTL)
	assert.Nil(t, err)

	// Access key1 to make it most recently used
	_, _ = cacheSize2.Get(ctx, key1, testModelVersion)

	// Add entity3, this should evict entity2 (least recently used)
	err = cacheSize2.MSet(ctx, map[string]*EntityToCache{key3.Key: entity3}, standardTestTTL)
	assert.Nil(t, err)

	// Check key1 (should be present)
	item, err = cacheSize2.Get(ctx, key1, testModelVersion)
	assert.Nil(t, err)
	assert.NotNil(t, item, "Key1 should still be in cache")

	// Check key2 (should be evicted)
	item, err = cacheSize2.Get(ctx, key2, testModelVersion)
	assert.Nil(t, err)
	assert.Nil(t, item, "Key2 should have been evicted")

	// Check key3 (should be present)
	item, err = cacheSize2.Get(ctx, key3, testModelVersion)
	assert.Nil(t, err)
	assert.NotNil(t, item, "Key3 should be in cache")

}

// Helper to make sure pointer to EntityToCache implements Entity
var _ Entity = (*EntityToCache)(nil)
var _ Entity = &EntityToCache{}
