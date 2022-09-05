package datasource_cache

import "time"

func NewCacheBuilder[T Entity, V any](
	modelVersion uint16,
	providers ...Provider[T, V],
) *Builder[T, V] {
	return &Builder[T, V]{
		providers:    providers,
		ttl:          5 * time.Minute,
		modelVersion: modelVersion,
	}
}

func (b *Builder[T, V]) Build() *Cache[T, V] {
	return &Cache[T, V]{
		builder: b,
	}
}

func (b *Builder[T, V]) WithTtl(ttl time.Duration) *Builder[T, V] {
	b.ttl = ttl

	return b
}
