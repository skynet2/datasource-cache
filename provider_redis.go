package cache

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/vmihailenco/msgpack/v5"
)

type RedisCache[T Entity, V any] struct {
	client    redis.Cmdable
	chunkSize int
}

func NewRedisCache[T Entity, V any](
	client redis.Cmdable,
) Provider[T, V] {
	return &RedisCache[T, V]{
		client:    client,
		chunkSize: 100,
	}
}

func (r *RedisCache[T, V]) Get(ctx context.Context, key *Key[V], requiredModelVersion uint16) (*T, error) {
	cmd := r.client.Get(ctx, key.Key)

	if cmd.Err() != nil {
		if errors.Is(cmd.Err(), redis.Nil) {
			return nil, nil
		}
		return nil, errors.WithStack(cmd.Err())
	}

	bts, err := cmd.Bytes()
	if err != nil {
		return nil, errors.WithStack(err)
	}

	var item T
	if err = msgpack.Unmarshal(bts, &item); err != nil {
		return nil, errors.WithStack(err)
	}

	if item.GetCacheModelVersion() != requiredModelVersion {
		return nil, nil
	}

	return &item, nil
}

func (r *RedisCache[T, V]) chunkBy(items []*Key[V], chunkSize int) (chunks [][]*Key[V]) {
	for chunkSize < len(items) {
		items, chunks = items[chunkSize:], append(chunks, items[0:chunkSize:chunkSize])
	}
	return append(chunks, items)
}

type redisChunkResponse[T, V any] struct {
	Error   error
	Missing []*Key[V]
	Results map[*Key[V]]*T
}

func (r *RedisCache[T, V]) MGet(ctx context.Context, keys []*Key[V], requiredModelVersion uint16) (map[*Key[V]]*T, []*Key[V], error) {
	chunks := r.chunkBy(keys, r.chunkSize)

	var respChannels []chan redisChunkResponse[T, V]

	for _, chunk := range chunks {
		chCopy := chunk
		ch := make(chan redisChunkResponse[T, V])
		respChannels = append(respChannels, ch)

		go func() {
			defer func() {
				close(ch)
			}()

			strSlice := make([]string, 0, len(chCopy))

			for _, v := range chCopy {
				strSlice = append(strSlice, v.Key)
			}

			cmd := r.client.MGet(ctx, strSlice...)

			if cmd.Err() != nil {
				ch <- redisChunkResponse[T, V]{
					Error: errors.WithStack(cmd.Err()),
				}
				return
			}

			var missing []*Key[V]
			results := map[*Key[V]]*T{}

			for i, v := range cmd.Val() {
				if v == nil {
					missing = append(missing, chCopy[i])
					continue
				}

				var item T

				var toUnpack []byte

				switch val := v.(type) {
				case []byte:
					toUnpack = val
				case string:
					toUnpack = []byte(val)
				}

				if err := msgpack.Unmarshal(toUnpack, &item); err != nil {
					zerolog.Ctx(ctx).Err(err).Send() // todo looks like cache is invalid
					missing = append(missing, chCopy[i])
					continue
				}

				if item.GetCacheModelVersion() != requiredModelVersion {
					continue
				}

				results[chCopy[i]] = &item
			}

			ch <- redisChunkResponse[T, V]{
				Missing: missing,
				Results: results,
			}
		}()
	}

	var missing []*Key[V]
	results := map[*Key[V]]*T{}

	for _, ch := range respChannels {
		resp := <-ch

		if resp.Error != nil {
			zerolog.Ctx(ctx).Err(resp.Error).Send()
			continue
		}

		if len(resp.Missing) > 0 {
			missing = append(missing, resp.Missing...)
		}

		for k, v := range resp.Results {
			results[k] = v
		}
	}

	return results, missing, nil
}

func (r *RedisCache[T, V]) MSet(ctx context.Context, values map[string]*T, ttl time.Duration) error {
	_, err := r.client.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
		for k, v := range values {
			b, err := msgpack.Marshal(v)
			if err != nil {
				log.Logger.Err(err).Send()
				continue
			}
			pipe.Set(ctx, k, b, ttl)
		}

		return nil
	})

	return err
}
