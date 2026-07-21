package redis

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

type pipeline struct {
	pipe redis.Pipeliner
}

func (p *pipeline) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	p.pipe.Set(ctx, key, value, expiration)
	return nil
}

func (p *pipeline) Get(ctx context.Context, key string) error {
	p.pipe.Get(ctx, key)
	return nil
}

func (p *pipeline) Del(ctx context.Context, keys ...string) error {
	p.pipe.Del(ctx, keys...)
	return nil
}

func (p *pipeline) Exec(ctx context.Context) error {
	_, err := p.pipe.Exec(ctx)
	return err
}

func (p *pipeline) Discard() error {
	p.pipe.Discard()
	return nil
}
