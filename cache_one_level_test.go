package datasource_cache

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"math/rand"
	"testing"
	"time"
)

type EntityToCache struct {
	Id           int
	Value        string
	ModelVersion uint16
}

func (e EntityToCache) GetCacheModelVersion() uint16 {
	return e.ModelVersion
}

// mockery --name="Provider" --case underscore --dir --output "." --with-expecter --inpackage --structname "mockProvider"
func TestOneLevelCacheSingleToDataSource(t *testing.T) {
	rand.Seed(time.Now().Unix())

	currentModelVersion := uint16(7)

	mockCacheProvider := newMockProvider[EntityToCache, int](t)

	randId := rand.Int()
	key := &Key[int]{
		Key:           fmt.Sprintf("totaly_random_prefix_with_key_%v", randId),
		OriginalValue: randId,
	}

	mockCacheProvider.EXPECT().Get(context.TODO(), key, currentModelVersion).
		Return(nil, nil)

	mockCacheProvider.EXPECT().MSet(context.TODO(), mock.Anything, mock.Anything).
		Run(func(ctx context.Context, values map[string]*EntityToCache, ttl time.Duration) {
			assert.Equal(t, 1, len(values))
			assert.Equal(t, values[key.Key].Value, "random_content")
		}).Return(nil)

	ch := NewCacheBuilder[EntityToCache, int](currentModelVersion, mockCacheProvider).
		Build()

	called := false

	result, err := ch.Get(context.TODO(), key, func(ctx context.Context, key *Key[int]) (*EntityToCache, error) {
		called = true

		return &EntityToCache{
			Id:           key.OriginalValue,
			Value:        "random_content",
			ModelVersion: currentModelVersion,
		}, nil
	})

	assert.Nil(t, err)
	assert.True(t, called)
	mockCacheProvider.AssertExpectations(t)

	assert.Equal(t, "random_content", result.Value)
	assert.Equal(t, key.OriginalValue, result.Id)
	assert.Equal(t, currentModelVersion, result.GetCacheModelVersion())
}

func TestOneLevelCacheSingleWithoutDataSource(t *testing.T) {
	rand.Seed(time.Now().Unix())

	currentModelVersion := uint16(7)

	mockCacheProvider := newMockProvider[EntityToCache, int](t)

	randId := rand.Int()
	key := &Key[int]{
		Key:           fmt.Sprintf("totaly_random_prefix_with_key_%v", randId),
		OriginalValue: randId,
	}

	mockCacheProvider.EXPECT().Get(context.TODO(), key, currentModelVersion).
		Return(&EntityToCache{
			Id:           key.OriginalValue,
			Value:        "random_content",
			ModelVersion: currentModelVersion,
		}, nil)

	ch := NewCacheBuilder[EntityToCache, int](currentModelVersion, mockCacheProvider).
		Build()

	called := false

	result, err := ch.Get(context.TODO(), key, func(ctx context.Context, key *Key[int]) (*EntityToCache, error) {
		called = true

		return &EntityToCache{
			Id:           key.OriginalValue,
			Value:        "random_content",
			ModelVersion: currentModelVersion,
		}, nil
	})

	assert.Nil(t, err)
	assert.False(t, called)
	mockCacheProvider.AssertExpectations(t)

	assert.Equal(t, "random_content", result.Value)
	assert.Equal(t, key.OriginalValue, result.Id)
	assert.Equal(t, currentModelVersion, result.GetCacheModelVersion())
}

func TestOneLevelCacheMultiRecord(t *testing.T) {
	rand.Seed(time.Now().Unix())

	currentModelVersion := uint16(7)

	mockCacheProvider := newMockProvider[EntityToCache, int](t)

	randId := rand.Int()
	key := &Key[int]{
		Key:           fmt.Sprintf("totaly_random_prefix_with_key_%v", randId),
		OriginalValue: randId,
	}

	randId2 := randId + 555
	key2 := &Key[int]{
		Key:           fmt.Sprintf("totaly_random_prefix_with_key_%v", randId2),
		OriginalValue: randId2,
	}

	keysArr := []*Key[int]{key, key2}

	mockCacheProvider.EXPECT().MGet(context.TODO(), keysArr, currentModelVersion).
		Return(nil, keysArr, nil)

	mockCacheProvider.EXPECT().MSet(context.TODO(), mock.Anything, mock.Anything).
		Run(func(ctx context.Context, values map[string]*EntityToCache, ttl time.Duration) {
			assert.Equal(t, 2, len(values))
			assert.Equal(t, values[key.Key].Value, "random_content")
			assert.Equal(t, values[key2.Key].Value, "random_content2")
		}).Return(nil)

	ch := NewCacheBuilder[EntityToCache, int](currentModelVersion, mockCacheProvider).
		Build()

	called := false

	result, err := ch.MGet(context.TODO(), keysArr, func(ctx context.Context, keys []*Key[int]) (map[*Key[int]]*EntityToCache, error) {
		called = true

		return map[*Key[int]]*EntityToCache{
			key: &EntityToCache{
				Id:           key.OriginalValue,
				Value:        "random_content",
				ModelVersion: currentModelVersion,
			},
			key2: &EntityToCache{
				Id:           key2.OriginalValue,
				Value:        "random_content2",
				ModelVersion: currentModelVersion,
			},
		}, nil
	})

	time.Sleep(300 * time.Millisecond) // set to cache is async
	assert.Nil(t, err)
	assert.True(t, called)
	mockCacheProvider.AssertExpectations(t)

	assert.Equal(t, 2, len(result))
	assert.Equal(t, "random_content", result[key].Value)
	assert.Equal(t, key.OriginalValue, result[key].Id)
	assert.Equal(t, currentModelVersion, result[key].GetCacheModelVersion())
}

func TestOneLevelCacheMultiRecordWithoutDataSource(t *testing.T) {
	rand.Seed(time.Now().Unix())

	currentModelVersion := uint16(7)

	mockCacheProvider := newMockProvider[EntityToCache, int](t)

	randId := rand.Int()
	key := &Key[int]{
		Key:           fmt.Sprintf("totaly_random_prefix_with_key_%v", randId),
		OriginalValue: randId,
	}

	randId2 := randId + 555
	key2 := &Key[int]{
		Key:           fmt.Sprintf("totaly_random_prefix_with_key_%v", randId2),
		OriginalValue: randId2,
	}

	keysArr := []*Key[int]{key, key2}

	mockCacheProvider.EXPECT().MGet(context.TODO(), keysArr, currentModelVersion).
		Return(map[*Key[int]]*EntityToCache{
			key: &EntityToCache{
				Id:           key.OriginalValue,
				Value:        "random_content",
				ModelVersion: currentModelVersion,
			},
			key2: &EntityToCache{
				Id:           key2.OriginalValue,
				Value:        "random_content2",
				ModelVersion: currentModelVersion,
			},
		}, nil, nil)

	ch := NewCacheBuilder[EntityToCache, int](currentModelVersion, mockCacheProvider).
		Build()

	called := false

	result, err := ch.MGet(context.TODO(), keysArr, func(ctx context.Context, keys []*Key[int]) (map[*Key[int]]*EntityToCache, error) {
		called = true

		return nil, errors.New("should not be called")
	})

	time.Sleep(300 * time.Millisecond) // set to cache is async
	assert.Nil(t, err)
	assert.False(t, called)
	mockCacheProvider.AssertExpectations(t)

	assert.Equal(t, 2, len(result))
	assert.Equal(t, "random_content", result[key].Value)
	assert.Equal(t, key.OriginalValue, result[key].Id)
	assert.Equal(t, currentModelVersion, result[key].GetCacheModelVersion())
}

func TestOneLevelCacheMultiRecordWitPartialDataSource(t *testing.T) {
	rand.Seed(time.Now().Unix())

	currentModelVersion := uint16(7)

	mockCacheProvider := newMockProvider[EntityToCache, int](t)

	randId := rand.Int()
	key := &Key[int]{
		Key:           fmt.Sprintf("totaly_random_prefix_with_key_%v", randId),
		OriginalValue: randId,
	}

	randId2 := randId + 555
	key2 := &Key[int]{
		Key:           fmt.Sprintf("totaly_random_prefix_with_key_%v", randId2),
		OriginalValue: randId2,
	}

	keysArr := []*Key[int]{key, key2}

	mockCacheProvider.EXPECT().MGet(context.TODO(), keysArr, currentModelVersion).
		Return(map[*Key[int]]*EntityToCache{
			key2: &EntityToCache{
				Id:           key2.OriginalValue,
				Value:        "random_content2",
				ModelVersion: currentModelVersion,
			},
		}, []*Key[int]{key}, nil)

	mockCacheProvider.EXPECT().MSet(context.TODO(), mock.Anything, mock.Anything).
		Run(func(ctx context.Context, values map[string]*EntityToCache, ttl time.Duration) {
			assert.Equal(t, 1, len(values))
			assert.Equal(t, "random_content", values[key.Key].Value)
			assert.Equal(t, key.OriginalValue, values[key.Key].Id)
		}).Return(nil)

	ch := NewCacheBuilder[EntityToCache, int](currentModelVersion, mockCacheProvider).
		Build()

	called := false

	result, err := ch.MGet(context.TODO(), keysArr, func(ctx context.Context, keys []*Key[int]) (map[*Key[int]]*EntityToCache, error) {
		called = true
		assert.Equal(t, 1, len(keys))
		assert.Equal(t, key, keys[0])

		return map[*Key[int]]*EntityToCache{
			key: &EntityToCache{
				Id:           key.OriginalValue,
				Value:        "random_content",
				ModelVersion: currentModelVersion,
			},
		}, nil
	})

	time.Sleep(300 * time.Millisecond) // set to cache is async
	assert.Nil(t, err)
	assert.True(t, called)
	mockCacheProvider.AssertExpectations(t)

	assert.Equal(t, 2, len(result))
	assert.Equal(t, "random_content", result[key].Value)
	assert.Equal(t, key.OriginalValue, result[key].Id)
	assert.Equal(t, currentModelVersion, result[key].GetCacheModelVersion())
}
