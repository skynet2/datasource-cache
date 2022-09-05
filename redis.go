package datasource_cache

import (
	"context"
	"github.com/go-redis/redis/v8"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/vmihailenco/msgpack/v5"
	"time"
)

type RedisCache[T Entity, V any] struct {
	client    *redis.Client
	chunkSize int
}

func NewRedisCache[T Entity, V any](
	client *redis.Client,
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

func (r *RedisCache[T, V]) chunkBy(items []string, chunkSize int) (chunks [][]string) {
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
	toGet1 := make([]string, 0, len(keys))
	for _, k := range keys {
		toGet1 = append(toGet1, k.Key)
	}

	chunks := r.chunkBy(toGet1, r.chunkSize)
	var respChannels []chan redisChunkResponse[T, V]

	for _, chunk := range chunks {
		chCopy := chunk
		ch := make(chan redisChunkResponse[T, V])
		respChannels = append(respChannels, ch)

		go func() {
			defer func() {
				close(ch)
			}()

			cmd := r.client.MGet(ctx, chCopy...)

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
					missing = append(missing, keys[i])
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
					missing = append(missing, keys[i])
					continue
				}

				if item.GetCacheModelVersion() != requiredModelVersion {
					continue
				}

				results[keys[i]] = &item
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
	var multiErr error
	finalArr := make([]interface{}, 0, len(values)*2)

	for k, v := range values {
		if b, err := msgpack.Marshal(v); err != nil {
			multiErr = multierror.Append(multiErr, errors.WithStack(err))
			continue
		} else {
			finalArr = append(finalArr, k, b)
		}
	}

	if len(finalArr) == 0 {
		return errors.Wrap(multiErr, "no items to continue")
	}

	if err := r.client.MSet(ctx, finalArr).Err(); err != nil {
		log.Logger.Err(err).Send()
		return errors.WithStack(err)
	}

	for key := range values {
		if err := r.client.Expire(context.Background(), key, ttl).Err(); err != nil {
			zerolog.Ctx(ctx).Err(err).Send()
		}
	}

	return nil
}
