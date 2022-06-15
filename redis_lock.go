package redislock

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisLock 是基于 Redis 单节点的分布式锁
//
//		// 初始化一个名为 lock-key 的锁，指定过期时间为 1 小时
//		lock := NewRedisLock(redisClient, 'lock-key', time.Hour)
//
//		// 尝试加锁并堵塞。以 5 秒为间隔、不断地尝试加锁，直到加锁成功、或超过 1 分钟报错
//		err := lock.Lock(time.Second * 5, time.Minute)
//
//		// 释放锁
//		err = lock.Unlock()
//
type RedisLock struct {
	client *redis.Client
	key    string
	expire time.Duration
	random string
	rand   *rand.Rand
}

// NewRedisLock 初始化一个分布式锁，并指定过期时间（即加锁后的自动释放时间）
func NewRedisLock(client *redis.Client, key string, expire time.Duration) *RedisLock {
	seed := time.Now().UnixNano()
	return &RedisLock{
		client: client,
		key:    key,
		expire: expire,
		random: fmt.Sprintf("%v", seed),
		rand:   rand.New(rand.NewSource(seed)),
	}
}

// Lock 以特定的时间间隔、不断地尝试加锁，直到加锁成功或者超时报错。其中，为了错开竞争，时间间隔会在 [50%, 150%) 区间随机波动。
func (l *RedisLock) Lock(attemptInterval time.Duration, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return l.LockWithContext(ctx, attemptInterval)
}

// LockWithContext 以特定的时间间隔、不断地尝试加锁，直到加锁成功。其中，为了错开竞争，时间间隔会在 [50%, 150%) 区间随机波动。
func (l *RedisLock) LockWithContext(ctx context.Context, attemptInterval time.Duration) error {
	// 等待时间在 [ interval * 50%, interval * 150% ] 区间之间随机获得，以防止多线程同时竞争导致的死锁
	ticker := time.NewTicker(attemptInterval/2 + time.Duration(l.rand.Int63n(int64(attemptInterval))))
	defer func() {
		ticker.Stop()
	}()
	for {
		success, err := l.AttemptLock(ctx)
		if err != nil {
			return fmt.Errorf("redis-lock: failed to acquire lock '%v': %v", l.key, err)
		}
		if success {
			return nil
		}
		<-ticker.C
	}
}

// Unlock 释放锁
func (l *RedisLock) Unlock() error {
	release := redis.NewScript(`
		if redis.call("get",KEYS[1]) == ARGV[1]
		then
			return redis.call("del",KEYS[1])
		else
			return 0
		end
	`)
	ctx := context.Background() // 在这个场景，解锁操作应该是不允许取消和超时的，所以直接使用 Backgroud
	_, err := release.Run(ctx, l.client, []string{l.key}, l.random).Result()
	if err != nil {
		return err
	}
	return nil
}

// AttemptLock 尝试加锁，并立即返回结果。只有在返回 true 的情况下，才认为加锁成功。
func (l *RedisLock) AttemptLock(ctx context.Context) (bool, error) {
	return l.client.SetNX(ctx, l.key, l.random, l.expire).Result()
}
