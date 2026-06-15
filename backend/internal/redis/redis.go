package redis

import (
	"context"
	"pipe-monitor/internal/config"
	"time"

	goredis "github.com/redis/go-redis/v9"
)

type Client struct {
	*goredis.Client
}

func Init(cfg *config.Config) (*Client, error) {
	rdb := goredis.NewClient(&goredis.Options{
		Addr:     cfg.Redis.Addr,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
		PoolSize: 50,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		return nil, err
	}

	return &Client{Client: rdb}, nil
}

func (c *Client) GetPipelineStatus(ctx context.Context, pipelineID uint) (string, error) {
	key := "pipeline:status:" + uintToStr(pipelineID)
	return c.Get(ctx, key).Result()
}

func (c *Client) SetPipelineStatus(ctx context.Context, pipelineID uint, status string, ttl time.Duration) error {
	key := "pipeline:status:" + uintToStr(pipelineID)
	return c.Set(ctx, key, status, ttl).Err()
}

func (c *Client) SetRunStatus(ctx context.Context, runID uint, status interface{}, ttl time.Duration) error {
	key := "run:status:" + uintToStr(runID)
	return c.Set(ctx, key, status, ttl).Err()
}

func uintToStr(v uint) string {
	return string(rune(v))
}
