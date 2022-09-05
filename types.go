package cache

import (
	"context"
	"time"
)

type Provider[T, V any] interface {
	Get(ctx context.Context, key *Key[V], requiredModelVersion uint16) (*T, error)
	MGet(ctx context.Context, keys []*Key[V], requiredModelVersion uint16) (map[*Key[V]]*T, []*Key[V], error)
	MSet(ctx context.Context, values map[string]*T, ttl time.Duration) error
}

type Builder[T, V any] struct {
	providers    []Provider[T, V]
	ttl          time.Duration
	modelVersion uint16
}

type Cache[T any, V any] struct {
	builder *Builder[T, V]
}

type Key[V any] struct {
	Key           string
	OriginalValue V
}

type GetFromSourceFn[T, V any] func(ctx context.Context, key []*Key[V]) (map[*Key[V]]*T, error)
type GetSingleFromSourceFn[T, V any] func(ctx context.Context, key *Key[V]) (*T, error)

type Entity interface {
	GetCacheModelVersion() uint16
}

type missingData[T, V any] struct {
	provider    Provider[T, V]
	missingKeys []*Key[V]
}
