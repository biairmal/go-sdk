package redis

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/redis/go-redis/v9"
)

//go:generate go run go.uber.org/mock/mockgen@v0.6.0 -destination=../mocks/redis/mock_redis.go -package=mockredis github.com/biairmal/go-sdk/redis Client,Pipeliner

// Client is the Redis client interface implemented by all connection modes
// (standalone, sentinel, cluster).
type Client interface {
	// Basic operations
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	Del(ctx context.Context, keys ...string) error
	Exists(ctx context.Context, keys ...string) (int64, error)

	// Hash operations
	HGet(ctx context.Context, key, field string) (string, error)
	HSet(ctx context.Context, key string, values ...interface{}) error
	HGetAll(ctx context.Context, key string) (map[string]string, error)
	HDel(ctx context.Context, key string, fields ...string) error

	// Set operations
	SAdd(ctx context.Context, key string, members ...interface{}) error
	SMembers(ctx context.Context, key string) ([]string, error)
	SRem(ctx context.Context, key string, members ...interface{}) error

	// List operations
	LPush(ctx context.Context, key string, values ...interface{}) error
	RPush(ctx context.Context, key string, values ...interface{}) error
	LPop(ctx context.Context, key string) (string, error)
	RPop(ctx context.Context, key string) (string, error)
	LRange(ctx context.Context, key string, start, stop int64) ([]string, error)

	// Expiration
	Expire(ctx context.Context, key string, expiration time.Duration) error
	TTL(ctx context.Context, key string) (time.Duration, error)

	// Atomic operations
	Incr(ctx context.Context, key string) (int64, error)
	Decr(ctx context.Context, key string) (int64, error)
	IncrBy(ctx context.Context, key string, value int64) (int64, error)

	// Eval executes a Lua script atomically against keys and args, returning
	// the script's result (Lua numbers/strings/tables map to int64/string/
	// []interface{}). For operations that need atomicity beyond a single
	// command, e.g. ratelimit's sliding-window check-and-increment.
	Eval(ctx context.Context, script string, keys []string, args ...interface{}) (interface{}, error)

	// Pipeline and transaction support
	Pipeline() Pipeliner
	TxPipeline() Pipeliner

	// Utilities
	Ping(ctx context.Context) error
	Close() error
}

// Pipeliner for batch operations
type Pipeliner interface {
	Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error
	Get(ctx context.Context, key string) error
	Del(ctx context.Context, keys ...string) error
	Exec(ctx context.Context) error
	Discard() error
}

// redisClient wraps the go-redis client
type redisClient struct {
	client redis.UniversalClient
}

func NewClient(cfg *Config) (Client, error) {
	var client redis.UniversalClient
	var tlsCfg *tls.Config

	if cfg.TLSEnabled {
		var err error
		tlsCfg, err = createTLSConfig(cfg)
		if err != nil {
			return nil, fmt.Errorf("tls config: %w", err)
		}
	}

	switch cfg.Mode {
	case ModeStandalone:
		opts := &redis.Options{
			Addr:         cfg.Address,
			Username:     cfg.Username,
			Password:     cfg.Password,
			DB:           cfg.DB,
			PoolSize:     cfg.PoolSize,
			MinIdleConns: cfg.MinIdleConns,
			MaxRetries:   cfg.MaxRetries,
			DialTimeout:  cfg.DialTimeout,
			ReadTimeout:  cfg.ReadTimeout,
			WriteTimeout: cfg.WriteTimeout,
			PoolTimeout:  cfg.PoolTimeout,
			TLSConfig:    tlsCfg,
		}
		client = redis.NewClient(opts)

	case ModeSentinel:
		if cfg.MasterName == "" || len(cfg.SentinelAddrs) == 0 {
			return nil, fmt.Errorf("sentinel mode requires master name and sentinel addresses")
		}

		opts := &redis.FailoverOptions{
			MasterName:       cfg.MasterName,
			SentinelAddrs:    cfg.SentinelAddrs,
			SentinelPassword: cfg.SentinelPassword,
			Username:         cfg.Username,
			Password:         cfg.Password,
			DB:               cfg.DB,
			PoolSize:         cfg.PoolSize,
			MinIdleConns:     cfg.MinIdleConns,
			MaxRetries:       cfg.MaxRetries,
			DialTimeout:      cfg.DialTimeout,
			ReadTimeout:      cfg.ReadTimeout,
			WriteTimeout:     cfg.WriteTimeout,
			PoolTimeout:      cfg.PoolTimeout,
			TLSConfig:        tlsCfg,
		}

		client = redis.NewFailoverClient(opts)

	case ModeCluster:
		if len(cfg.ClusterAddrs) == 0 {
			return nil, fmt.Errorf("cluster mode requires at least one cluster address")
		}

		opts := &redis.ClusterOptions{
			Addrs:          cfg.ClusterAddrs,
			Username:       cfg.Username,
			Password:       cfg.Password,
			PoolSize:       cfg.PoolSize,
			MinIdleConns:   cfg.MinIdleConns,
			MaxRetries:     cfg.MaxRetries,
			DialTimeout:    cfg.DialTimeout,
			ReadTimeout:    cfg.ReadTimeout,
			WriteTimeout:   cfg.WriteTimeout,
			PoolTimeout:    cfg.PoolTimeout,
			ReadOnly:       cfg.ReadOnly,
			RouteByLatency: cfg.RouteByLatency,
			RouteRandomly:  cfg.RouteRandomly,
			MaxRedirects:   cfg.MaxRedirects,
			TLSConfig:      tlsCfg,
		}

		client = redis.NewClusterClient(opts)

	default:
		return nil, fmt.Errorf("unsupported redis mode: %s", cfg.Mode)
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Ping(ctx).Err(); err != nil {
		return nil, fmt.Errorf("failed to connect to redis: %w", err)
	}

	return &redisClient{client: client}, nil
}

// createTLSConfig builds a tls.Config from cfg. TLSCert/TLSKey are file paths
// for client certificate (mTLS); TLSCA is the path to the CA certificate used
// to verify the server. If TLSCA is empty, the system default CAs are used.
func createTLSConfig(cfg *Config) (*tls.Config, error) {
	base := &tls.Config{
		MinVersion: tls.VersionTLS12,
		MaxVersion: tls.VersionTLS13,
		CurvePreferences: []tls.CurveID{
			tls.X25519,
			tls.CurveP256,
		},
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
		},
	}

	if cfg.TLSCert != "" && cfg.TLSKey != "" {
		cert, err := tls.LoadX509KeyPair(cfg.TLSCert, cfg.TLSKey)
		if err != nil {
			return nil, fmt.Errorf("load client cert/key: %w", err)
		}
		base.Certificates = []tls.Certificate{cert}
	}

	if cfg.TLSCA != "" {
		caPEM, err := os.ReadFile(cfg.TLSCA)
		if err != nil {
			return nil, fmt.Errorf("read CA file: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caPEM) {
			return nil, fmt.Errorf("no valid CA certificates in %s", cfg.TLSCA)
		}
		base.RootCAs = pool
	}

	return base, nil
}

// Get retrieves a value by key
func (r *redisClient) Get(ctx context.Context, key string) (string, error) {
	val, err := r.client.Get(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", ErrKeyNotFound
	}
	return val, err
}

// Set stores a value with optional expiration
func (r *redisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	return r.client.Set(ctx, key, value, expiration).Err()
}

// Del deletes one or more keys
func (r *redisClient) Del(ctx context.Context, keys ...string) error {
	return r.client.Del(ctx, keys...).Err()
}

// Exists checks if keys exist
func (r *redisClient) Exists(ctx context.Context, keys ...string) (int64, error) {
	return r.client.Exists(ctx, keys...).Result()
}

// HGet gets a field from a hash
func (r *redisClient) HGet(ctx context.Context, key, field string) (string, error) {
	val, err := r.client.HGet(ctx, key, field).Result()
	if errors.Is(err, redis.Nil) {
		return "", ErrKeyNotFound
	}
	return val, err
}

// HSet sets fields in a hash
func (r *redisClient) HSet(ctx context.Context, key string, values ...interface{}) error {
	return r.client.HSet(ctx, key, values...).Err()
}

// HGetAll gets all fields from a hash
func (r *redisClient) HGetAll(ctx context.Context, key string) (map[string]string, error) {
	return r.client.HGetAll(ctx, key).Result()
}

// HDel deletes fields from a hash
func (r *redisClient) HDel(ctx context.Context, key string, fields ...string) error {
	return r.client.HDel(ctx, key, fields...).Err()
}

// SAdd adds members to a set
func (r *redisClient) SAdd(ctx context.Context, key string, members ...interface{}) error {
	return r.client.SAdd(ctx, key, members...).Err()
}

// SMembers gets all members of a set
func (r *redisClient) SMembers(ctx context.Context, key string) ([]string, error) {
	return r.client.SMembers(ctx, key).Result()
}

// SRem removes members from a set
func (r *redisClient) SRem(ctx context.Context, key string, members ...interface{}) error {
	return r.client.SRem(ctx, key, members...).Err()
}

// LPush prepends values to a list
func (r *redisClient) LPush(ctx context.Context, key string, values ...interface{}) error {
	return r.client.LPush(ctx, key, values...).Err()
}

// RPush appends values to a list
func (r *redisClient) RPush(ctx context.Context, key string, values ...interface{}) error {
	return r.client.RPush(ctx, key, values...).Err()
}

// LPop removes and returns the first element
func (r *redisClient) LPop(ctx context.Context, key string) (string, error) {
	val, err := r.client.LPop(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", ErrKeyNotFound
	}
	return val, err
}

// RPop removes and returns the last element
func (r *redisClient) RPop(ctx context.Context, key string) (string, error) {
	val, err := r.client.RPop(ctx, key).Result()
	if errors.Is(err, redis.Nil) {
		return "", ErrKeyNotFound
	}
	return val, err
}

// LRange gets a range of elements from a list
func (r *redisClient) LRange(ctx context.Context, key string, start, stop int64) ([]string, error) {
	return r.client.LRange(ctx, key, start, stop).Result()
}

// Expire sets expiration on a key
func (r *redisClient) Expire(ctx context.Context, key string, expiration time.Duration) error {
	return r.client.Expire(ctx, key, expiration).Err()
}

// TTL gets the time to live for a key
func (r *redisClient) TTL(ctx context.Context, key string) (time.Duration, error) {
	return r.client.TTL(ctx, key).Result()
}

// Incr increments a key
func (r *redisClient) Incr(ctx context.Context, key string) (int64, error) {
	return r.client.Incr(ctx, key).Result()
}

// Decr decrements a key
func (r *redisClient) Decr(ctx context.Context, key string) (int64, error) {
	return r.client.Decr(ctx, key).Result()
}

// IncrBy increments a key by a value
func (r *redisClient) IncrBy(ctx context.Context, key string, value int64) (int64, error) {
	return r.client.IncrBy(ctx, key, value).Result()
}

// Eval executes a Lua script atomically against keys and args.
func (r *redisClient) Eval(
	ctx context.Context, script string, keys []string, args ...interface{},
) (interface{}, error) {
	return r.client.Eval(ctx, script, keys, args...).Result()
}

// Pipeline creates a pipeline for batch operations
func (r *redisClient) Pipeline() Pipeliner {
	return &pipeline{pipe: r.client.Pipeline()}
}

// TxPipeline creates a transactional pipeline
func (r *redisClient) TxPipeline() Pipeliner {
	return &pipeline{pipe: r.client.TxPipeline()}
}

// Ping checks the connection
func (r *redisClient) Ping(ctx context.Context) error {
	return r.client.Ping(ctx).Err()
}

// Close closes the client
func (r *redisClient) Close() error {
	return r.client.Close()
}
