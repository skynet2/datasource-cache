package main

import (
	"context"
	"fmt"
	"log"
	"time"
	// "strings" // No longer needed for this simplified example

	// "github.com/redis/go-redis/v9" // Keep if you want to show Redis comparison
	cache "github.com/skynet2/datasource-cache"
	// "github.com/rs/zerolog" // No longer needed for this simplified example
)

const ModelVersion = uint16(1) // currentModelVersion

// MyEntity is a sample struct that implements cache.Entity.
type MyEntity struct {
	ID           string
	Value        string
	ModelVersion uint16
}

// GetCacheModelVersion returns the model version of the entity.
func (e *MyEntity) GetCacheModelVersion() uint16 {
	return e.ModelVersion
}

func main() {
	ctx := context.Background()
	log.Println("--- Starting LRU Cache Example ---")

	// 1. Instantiate LRUCache
	// Using string as the original value type V for the key, matching previous example structure
	lruCacheProvider := cache.NewLRUCache[MyEntity, string](100) // Cache size 100
	log.Println("LRU Cache Provider instantiated.")

	// 2. (Optional) Instantiate Redis Cache for comparison or multi-level setup
	/*
		redisClient := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
		redisCacheProvider := cache.NewRedisCache[MyEntity, string](redisClient)
		log.Println("Redis Cache Provider instantiated (optional).")
	*/

	// 3. Create Cache Builder
	// Using only LRU provider for this example
	cacheClient := cache.NewCacheBuilder[MyEntity, string](ModelVersion, lruCacheProvider).
		WithTtl(5 * time.Minute). // Default TTL for items set through the cache client
		Build()
	log.Println("Cache Client built with LRU provider.")

	// 4. Demonstrate Get operation
	log.Println("\n--- Demonstrating Get Operation ---")
	entityKey1 := &cache.Key[string]{
		Key:           "myEntity:1",
		OriginalValue: "original_1", // This would be the raw ID or value used to fetch from source
	}

	getFromSourceFn := func(ctx context.Context, key *cache.Key[string]) (*MyEntity, error) {
		log.Printf("Get SourceFn called for key: %s (Original: %s)\n", key.Key, key.OriginalValue)
		// Simulate fetching data from a data source
		return &MyEntity{
			ID:           key.OriginalValue,
			Value:        fmt.Sprintf("Data for %s", key.OriginalValue),
			ModelVersion: ModelVersion,
		}, nil
	}

	// First Get: Cache miss, sourceFn should be called
	log.Println("First Get call (expect cache miss):")
	retrievedEntity, err := cacheClient.Get(ctx, entityKey1, getFromSourceFn)
	if err != nil {
		log.Fatalf("Error on first Get: %v", err)
	}
	log.Printf("Retrieved (1st call): %+v\n", retrievedEntity)

	// Second Get: Cache hit, sourceFn should NOT be called
	log.Println("\nSecond Get call (expect cache hit):")
	retrievedEntity, err = cacheClient.Get(ctx, entityKey1, getFromSourceFn)
	if err != nil {
		log.Fatalf("Error on second Get: %v", err)
	}
	log.Printf("Retrieved (2nd call): %+v\n", retrievedEntity)

	// 5. Demonstrate MSet and MGet operations
	log.Println("\n--- Demonstrating MSet and MGet Operations ---")

	// MSet
	itemsToSet := map[string]*MyEntity{
		"myEntity:2": {ID: "original_2", Value: "Data for original_2", ModelVersion: ModelVersion},
		"myEntity:3": {ID: "original_3", Value: "Data for original_3", ModelVersion: ModelVersion},
	}
	err = cacheClient.MSet(ctx, itemsToSet) // MSet in builder/cache uses the default TTL
	if err != nil {
		log.Fatalf("Error on MSet: %v", err)
	}
	log.Printf("MSet called for keys: myEntity:2, myEntity:3\n")

	// MGet
	keysToGet := []*cache.Key[string]{
		{Key: "myEntity:1", OriginalValue: "original_1"}, // Should be a hit from previous Get
		{Key: "myEntity:2", OriginalValue: "original_2"}, // Should be a hit from MSet
		{Key: "myEntity:3", OriginalValue: "original_3"}, // Should be a hit from MSet
		{Key: "myEntity:4", OriginalValue: "original_4"}, // Should be a miss
	}

	mGetFromSourceFn := func(ctx context.Context, keys []*cache.Key[string]) (map[*cache.Key[string]]*MyEntity, error) {
		log.Printf("MGet SourceFn called for %d keys:\n", len(keys))
		results := make(map[*cache.Key[string]]*MyEntity)
		for _, k := range keys {
			log.Printf("  - MGet SourceFn processing key: %s (Original: %s)\n", k.Key, k.OriginalValue)
			results[k] = &MyEntity{
				ID:           k.OriginalValue,
				Value:        fmt.Sprintf("Data for %s (from MGet sourceFn)", k.OriginalValue),
				ModelVersion: ModelVersion,
			}
		}
		return results, nil
	}

	log.Println("\nMGet call:")
	retrievedItems, err := cacheClient.MGet(ctx, keysToGet, mGetFromSourceFn)
	if err != nil {
		log.Fatalf("Error on MGet: %v", err)
	}

	log.Println("MGet retrieved items:")
	for key, entity := range retrievedItems {
		log.Printf("  - Key: %s, Entity: %+v\n", key.Key, entity)
	}

	log.Println("\n--- LRU Cache Example Finished ---")
}

// Dummy dbRepo and translateService for completeness, can be removed if not used by cache directly.
// The example focuses on direct cache interaction.
/*
type dbRepo struct {}

func (d *dbRepo) GetTokens(ctx context.Context, tokens []string) (map[string]map[string]string, error) {
	log.Printf("dbRepo.GetTokens called with: %v (should ideally not be hit if cache works for all)", tokens)
	// Simulate DB lookup
	data := make(map[string]map[string]string)
	for _, token := range tokens {
		data[token] = map[string]string{"en": fmt.Sprintf("Translated %s", token)}
	}
	return data, nil
}

type translateService struct {
	cacheManager *cache.Cache[MyEntity, string]
	dbRepo       *dbRepo
}

func (t *translateService) GetTranslation(
	ctx context.Context,
	language string,
	inputTokens []string,
) (map[string]string, error) {
	log.Printf("translateService.GetTranslation for language '%s', tokens: %v", language, inputTokens)
	cacheTokens := make([]*cache.Key[string], 0, len(inputTokens))
	for _, token := range inputTokens {
		cacheTokens = append(cacheTokens, &cache.Key[string]{
			OriginalValue: token,
			Key:           fmt.Sprintf("%s:%s", language, token),
		})
	}

	getFromSourceFn := func(ctx context.Context, keys []*cache.Key[string]) (map[*cache.Key[string]]*MyEntity, error) {
		log.Printf("translateService SourceFn called for %d keys", len(keys))
		// ... (similar logic to the original, adapted for MyEntity)
		// This part needs careful reimplementation if you keep translateService
		sourceData := make(map[*cache.Key[string]]*MyEntity)
		
		// Simplified: just create dummy entities
		for _, k := range keys {
			log.Printf("  - translateService SourceFn processing key: %s", k.Key)
			sourceData[k] = &MyEntity{
				ID: k.OriginalValue,
				Value: fmt.Sprintf("Translated %s for %s", k.OriginalValue, language),
				ModelVersion: ModelVersion,
			}
		}
		return sourceData, nil
	}
	
	result, err := t.cacheManager.MGet(ctx, cacheTokens, getFromSourceFn)
	if err != nil {
		return nil, fmt.Errorf("MGet in translateService failed: %w", err)
	}

	final := make(map[string]string)
	for k, v := range result {
		final[k.OriginalValue] = v.Value
	}
	log.Printf("translateService.GetTranslation result: %v", final)
	return final, nil
}
*/
