package cache

// CacheStrategy defines how cache is updated on writes:
// - WriteAroundStrategy: invalidates cache on update, next read fetches from DB (default, safe zero value)
// - WriteThroughStrategy: re-fetches from DB after update and writes the fresh row to cache
// - WriteBehindStrategy: queues cache updates asynchronously (not recommended for most cases)
//
// WriteAroundStrategy is the zero value so that a zero-initialized Option is safe by default.
//
// See specs/REPOSITORY_SPEC.md section 8 for detailed specification.
type CacheStrategy int

const (
	// WriteAroundStrategy invalidates cache on update.
	// Next read will fetch from database and repopulate the cache.
	// This is the zero value, so it is the default for any zero-initialized Option.
	WriteAroundStrategy CacheStrategy = iota

	// WriteThroughStrategy re-fetches the entity from the database after every update
	// and writes the fresh (DB-committed) row to the cache. This ensures server-side
	// mutations (triggers, computed columns, updated_at) are reflected in cached values.
	// Note: this adds one extra DB read per write.
	WriteThroughStrategy

	// WriteBehindStrategy queues cache updates (async).
	// More complex, not recommended for most cases.
	//
	// TODO, NOT IMPLEMENTED YET — falls back to WriteAroundStrategy (cache invalidation).
	WriteBehindStrategy
)
