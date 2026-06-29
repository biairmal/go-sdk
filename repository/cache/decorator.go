package cache

import (
	"context"
	"reflect"
	"time"

	"github.com/biairmal/go-sdk/redis"
	"github.com/biairmal/go-sdk/repository"
	"github.com/biairmal/go-sdk/serializer"
)

type Option struct {
	strategy CacheStrategy
	keyGen   KeyGenerator
	ttl      time.Duration
}

type CachedRepositoryOption func(*Option)

func WithStrategy(strategy CacheStrategy) CachedRepositoryOption {
	return func(o *Option) {
		o.strategy = strategy
	}
}

func WithKeyGenerator(keyGen KeyGenerator) CachedRepositoryOption {
	return func(o *Option) {
		if keyGen != nil {
			o.keyGen = keyGen
		}
	}
}

func WithTTL(ttl time.Duration) CachedRepositoryOption {
	return func(o *Option) {
		o.ttl = ttl
	}
}

// The caching decorator will:
// 1. Wrap any repository.Repository[T] with caching functionality
// 2. Support multiple cache strategies (write-through, write-around, write-behind)
// 3. Provide cache key generation
// 4. Handle cache invalidation on writes
// 5. Support TTL configuration
//
// See specs/REPOSITORY_SPEC.md section 7 for detailed specification.
type CachedRepository[TEntity any, TID comparable] struct {
	underlying repository.Repository[TEntity, TID]
	cache      redis.Client
	opt        Option
}

func NewCachedRepository[TEntity any, TID comparable](
	underlying repository.Repository[TEntity, TID],
	cache redis.Client,
	opts ...CachedRepositoryOption,
) repository.Repository[TEntity, TID] {
	// Derive the namespace from the entity type name so that two CachedRepository
	// instances wrapping different entity types sharing the same Redis instance
	// never collide on cache keys. Falls back to "entity" for anonymous types.
	typeName := reflect.TypeOf((*TEntity)(nil)).Elem().Name()
	if typeName == "" {
		typeName = "entity"
	}

	opt := Option{
		strategy: WriteAroundStrategy,
		keyGen:   NewDefaultKeyGenerator(typeName),
		ttl:      5 * time.Minute,
	}
	for _, o := range opts {
		o(&opt)
	}
	return &CachedRepository[TEntity, TID]{
		underlying: underlying,
		cache:      cache,
		opt:        opt,
	}
}

// Create inserts a new entity.
// The entity is not proactively cached here because the typed ID is not accessible
// in generic context after create. GetByID will populate the cache on first read.
func (r *CachedRepository[TEntity, TID]) Create(ctx context.Context, entity *TEntity) error {
	return r.underlying.Create(ctx, entity)
}

// GetByID retrieves an entity by its ID.
// On a cache hit the entity is returned without touching the database.
// On a cache miss (or any Redis error) the entity is fetched from the database
// and written to the cache for future reads.
//
// Cache-aside race: a concurrent Update may invalidate the key between this
// function's DB fetch and its subsequent cache.Set, causing a stale entry to be
// re-inserted for up to opt.ttl. Eliminating this race requires distributed
// coordination (e.g. singleflight or a Redis lock) which is out of scope here.
func (r *CachedRepository[TEntity, TID]) GetByID(ctx context.Context, id TID) (*TEntity, error) {
	key := r.buildEntityCacheKey(id)

	cached, errCache := r.cache.Get(ctx, key)
	if errCache == nil {
		// cache hit — deserialise and return
		var entity TEntity
		if err := serializer.ParseJSON([]byte(cached), &entity); err == nil {
			return &entity, nil
		}
		// corrupt cache entry — fall through to DB and overwrite below
	}
	// Any Redis error (including ErrKeyNotFound) is treated as a cache miss so
	// that a Redis outage does not take down reads when the database is healthy.

	// cache miss — fetch from DB
	entity, err := r.underlying.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Do not cache a nil entity: ToJSON(nil) → "null", which would be
	// deserialized as &TEntity{} on the next hit, breaking nil-checks downstream.
	if entity != nil {
		if data, jsonErr := serializer.ToJSON(entity); jsonErr == nil {
			_ = r.cache.Set(ctx, key, data, r.opt.ttl)
		}
	}

	return entity, nil
}

// Update updates an existing entity.
// Write-through: re-fetches the DB-committed row and writes it to cache so that
// server-side mutations (triggers, computed columns, updated_at) are reflected.
// Write-around / Write-behind: invalidates the cache key; next read repopulates.
func (r *CachedRepository[TEntity, TID]) Update(ctx context.Context, id TID, entity *TEntity) error {
	if err := r.underlying.Update(ctx, id, entity); err != nil {
		return err
	}

	key := r.buildEntityCacheKey(id)

	switch r.opt.strategy {
	case WriteThroughStrategy:
		// Re-fetch from DB to capture server-side mutations (triggers, computed columns).
		// Fall back to cache invalidation if the re-fetch or serialization fails so that
		// the cache never serves a stale entry.
		if fresh, fetchErr := r.underlying.GetByID(ctx, id); fetchErr == nil && fresh != nil {
			if data, jsonErr := serializer.ToJSON(fresh); jsonErr == nil {
				_ = r.cache.Set(ctx, key, data, r.opt.ttl)
				break
			}
		}
		_ = r.cache.Del(ctx, key)

	case WriteBehindStrategy:
		// Async queuing is not yet implemented. Fall back to write-around (invalidate)
		// so reads see fresh data rather than a stale cache entry.
		fallthrough
	case WriteAroundStrategy:
		_ = r.cache.Del(ctx, key)
	}

	return nil
}

// Delete removes an entity by its ID and invalidates its cache entry.
func (r *CachedRepository[TEntity, TID]) Delete(ctx context.Context, id TID) error {
	if err := r.underlying.Delete(ctx, id); err != nil {
		return err
	}

	_ = r.cache.Del(ctx, r.buildEntityCacheKey(id))
	return nil
}

// List retrieves entities with filtering and pagination and returns total count.
func (r *CachedRepository[TEntity, TID]) List(
	ctx context.Context, opts *repository.ListOptions,
) (entities []*TEntity, total int64, err error) {
	return r.underlying.List(ctx, opts)
}

// Count returns the total number of entities matching the filter.
func (r *CachedRepository[TEntity, TID]) Count(ctx context.Context, filter repository.Filter) (int64, error) {
	return r.underlying.Count(ctx, filter)
}

// Exists checks if an entity with given ID exists.
func (r *CachedRepository[TEntity, TID]) Exists(ctx context.Context, id TID) (bool, error) {
	return r.underlying.Exists(ctx, id)
}

func (r *CachedRepository[TEntity, TID]) buildEntityCacheKey(id TID) string {
	return r.opt.keyGen.Generate("id", id)
}
