package cache

import (
	"context"
	"encoding/json"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/sirupsen/logrus"
)

type Config struct {
	Host     string
	Port     string
	Password string
	DB       int
	PoolSize int
}

type RedisClient struct {
	*redis.Client
	config *Config
}

func NewRedisClient(config *Config) (*RedisClient, error) {
	rdb := redis.NewClient(&redis.Options{
		Addr:     config.Host + ":" + config.Port,
		Password: config.Password,
		DB:       config.DB,
		PoolSize: config.PoolSize,
	})

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		rdb.Close()
		return nil, err
	}

	logrus.Info("Successfully connected to Redis")

	return &RedisClient{
		Client: rdb,
		config: config,
	}, nil
}

func (r *RedisClient) SetJSON(ctx context.Context, key string, value interface{}, expiration time.Duration) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return r.Set(ctx, key, data, expiration).Err()
}

func (r *RedisClient) GetJSON(ctx context.Context, key string, dest interface{}) error {
	data, err := r.Get(ctx, key).Result()
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(data), dest)
}

func (r *RedisClient) DeletePattern(ctx context.Context, pattern string) error {
	keys, err := r.Keys(ctx, pattern).Result()
	if err != nil {
		return err
	}
	if len(keys) == 0 {
		return nil
	}
	return r.Del(ctx, keys...).Err()
}

func (r *RedisClient) GetConfig() *Config {
	return r.config
}

func (r *RedisClient) Close() error {
	logrus.Info("Closing Redis connection")
	return r.Client.Close()
}