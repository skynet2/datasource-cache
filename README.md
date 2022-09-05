# Datasource Cache

![build workflow](https://github.com/skynet2/datasource-cache/actions/workflows/build.yaml/badge.svg?branch=master)
[![codecov](https://codecov.io/gh/skynet2/datasource-cache/branch/master/graph/badge.svg?token=5QV4Z8NR6V)](https://codecov.io/gh/skynet2/datasource-cache)
[![go-report](https://img.shields.io/badge/go%20report-A+-brightgreen.svg?style=flat)](https://img.shields.io/badge/go%20report-A+-brightgreen.svg?style=flat)
[![PkgGoDev](https://pkg.go.dev/badge/github.com/skynet2/datasource-cache)](https://pkg.go.dev/github.com/skynet2/datasource-cache?tab=doc)

## Installation
```shell
go get github.com/skynet2/datasource-cache
```

## Quickstart
```go
package main

import (
	"context"
	"fmt"
	"github.com/go-redis/redis/v8"
	"github.com/rs/zerolog"
	cache "github.com/skynet2/datasource-cache"
	"strings"
)

func main() {
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	redisCacheProvider := cache.NewRedisCache[TranslatedEntity, string](
		redisClient)

	cacheManager := cache.NewCacheBuilder[TranslatedEntity, string](
		ModelVersion, redisCacheProvider).Build()

	tr := &translateService{cacheManager: cacheManager, dbRepo: &dbRepo{}}

	tr.GetTranslation(context.TODO(), "en", []string{"web:data1", "web:data2", "web:data3"})
}

const ModelVersion = uint16(1)

type dbRepo struct {
}

func (d *dbRepo) GetTokens(ctx context.Context, tokens []string) (map[string]map[string]string, error) {
	// retrieve data from db or any other source
    // for example if we requested 10 tokens, but 8 was in cache, here we`ll have only 2 tokens to request
	return map[string]map[string]string{}, nil
}

type TranslationToken struct {
	TokenKey     string
	Translations map[string]string
}

type TranslatedEntity struct {
	Value        string
	ModelVersion uint16
}

func (t TranslatedEntity) GetCacheModelVersion() uint16 {
	return t.ModelVersion
}

type translateService struct {
	cacheManager *cache.Cache[TranslatedEntity, string]
	dbRepo       *dbRepo
}

func (t *translateService) GetTranslation(
	ctx context.Context,
	language string,
	inputTokens []string,
) (map[string]string, error) {
	cacheTokens := make([]*cache.Key[string], 0)

	for _, token := range inputTokens {
		tok := strings.ToLower(token)

		cacheTokens = append(cacheTokens, &cache.Key[string]{
			OriginalValue: tok,                                 // here it will be as web:data1
			Key:           fmt.Sprintf("%v:%v", language, tok), // en:web:data1
		})
	}

	result, err := t.cacheManager.MGet(ctx, cacheTokens, func(ctx context.Context, keys []*cache.Key[string]) (map[*cache.Key[string]]*TranslatedEntity, error) {
		translations := make(map[*cache.Key[string]]*TranslatedEntity)
		missingTranslations := make(map[string]TranslationToken)

		toRequest := make([]string, 0, len(keys))
		reverseMap := map[string]*cache.Key[string]{}

		for _, k := range keys {
			toRequest = append(toRequest, k.OriginalValue)
			reverseMap[k.OriginalValue] = k
		}

		foundDbTokens, err := t.dbRepo.GetTokens(ctx, toRequest)

		if err != nil {
			return nil, err
		}

		for _, key := range toRequest {
			v, ok := reverseMap[key]

			if !ok {
				zerolog.Ctx(ctx).Warn().Msgf("not found in reverse map key %v", key)
				continue
			}

			if dbKey, ok := foundDbTokens[key]; ok {
				translations[v] = &TranslatedEntity{
					Value:        dbKey["default"], // find proper value
					ModelVersion: ModelVersion,
				}
			} else {
				translations[v] = &TranslatedEntity{
					ModelVersion: ModelVersion,
					Value:        key,
				}
				missingTranslations[key] = TranslationToken{
					TokenKey: key,
					Translations: map[string]string{
						"en": strings.ToLower(key),
					},
				}
			}
		}

		return translations, nil
	})

	if err != nil {
		return nil, err
	}

	final := make(map[string]string)

	for k, v := range result {
		final[k.OriginalValue] = v.Value
	}

	return final, nil
}
```