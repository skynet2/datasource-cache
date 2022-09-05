package cache

import (
	"context"
	"github.com/hashicorp/go-multierror"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
)

func (c *Cache[T, V]) Get(ctx context.Context, key *Key[V], fn GetSingleFromSourceFn[T, V]) (*T, error) {
	var missingIn []Provider[T, V]
	var finalValue *T

	for _, provider := range c.builder.providers {
		v, err := provider.Get(ctx, key, c.builder.modelVersion)

		if err != nil {
			zerolog.Ctx(ctx).Err(err).Send() // todo looks like cache is invalid
			continue
		}

		if v != nil {
			finalValue = v
			break
		}

		missingIn = append(missingIn, provider)
	}

	if finalValue == nil {
		if fn == nil {
			return nil, errors.New("get single from source is not defined")
		}

		var err error
		finalValue, err = fn(ctx, key)

		if err != nil { // can not get from source
			return nil, errors.Wrap(err, "can not get from source")
		}
	}

	if len(missingIn) > 0 {
		setMap := map[string]*T{
			key.Key: finalValue,
		}
		for _, m := range missingIn {
			if err := m.MSet(ctx, setMap, c.builder.ttl); err != nil { // todo
				zerolog.Ctx(ctx).Err(err).Send()
			}
		}
	}

	return finalValue, nil
}

func (c *Cache[T, V]) MGet(ctx context.Context, keys []*Key[V], fn GetFromSourceFn[T, V]) (map[*Key[V]]*T, error) {
	var missingIn []missingData[T, V]

	finalResults := map[*Key[V]]*T{}
	toQuery := keys

	for _, provider := range c.builder.providers {
		found, missing, err := provider.MGet(ctx, toQuery, c.builder.modelVersion)

		if err != nil {
			zerolog.Ctx(ctx).Err(err).Send() // todo looks like cache is invalid
			continue
		}

		if len(missing) > 0 {
			missingIn = append(missingIn, missingData[T, V]{
				provider:    provider,
				missingKeys: missing,
			})
		}

		for k, v := range found {
			finalResults[k] = v
		}

		toQuery = missing

		if len(missing) == 0 {
			break
		}
	}

	var valuesFromSource map[*Key[V]]*T

	if len(toQuery) > 0 {
		if fn == nil {
			return nil, errors.New("get single from source is not defined")
		}

		newValues, err := fn(ctx, toQuery)

		if err != nil { // can not get from source
			return nil, errors.Wrap(err, "can not get from source")
		}

		valuesFromSource = newValues
		for k, v := range newValues {
			finalResults[k] = v
		}
	}

	if len(missingIn) > 0 && len(valuesFromSource) > 0 {
		go func() {
			for _, m := range missingIn {
				toSet := map[string]*T{}
				for _, k := range m.missingKeys {
					if v, ok := valuesFromSource[k]; ok {
						toSet[k.Key] = v
					}
				}

				if err := m.provider.MSet(context.Background(), toSet, c.builder.ttl); err != nil { // coz async
					zerolog.Ctx(ctx).Err(err).Send() // todo
				}
			}
		}()
	}

	return finalResults, nil
}

func (c *Cache[T, V]) MSet(ctx context.Context, records map[string]*T) error {
	var finalErr error
	for _, m := range c.builder.providers {
		if err := m.MSet(ctx, records, c.builder.ttl); err != nil {
			finalErr = multierror.Append(finalErr, err)
		}
	}

	return finalErr
}
