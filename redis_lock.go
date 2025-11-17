package redislock

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisLock is a distributed lock backed by a single Redis node.
//
//		// Initialize a lock named lock-key with a 1 hour expiration
//		lock := NewRedisLock(redisClient, 'lock-key', time.Hour)
//
//		// Retry every 5 seconds until the lock is acquired, or fail after 1 minute
//		err := lock.Lock(time.Second * 5, time.Minute)
//
//		// Release the lock
//		err = lock.Unlock()
//
type RedisLock struct {
	client *redis.Client
	key    string
	expire time.Duration
	random string
	rand   *rand.Rand
}

// NewRedisLock initializes a distributed lock with the specified expiration (automatic release duration).
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

// Lock retries at the provided interval until the lock is acquired or the timeout elapses.
// The interval jitters randomly within [50%, 150%) to reduce contention.
func (l *RedisLock) Lock(attemptInterval time.Duration, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return l.LockWithContext(ctx, attemptInterval)
}

// LockWithContext retries at the provided interval until the lock is acquired.
// The interval jitters randomly within [50%, 150%) to reduce contention.
func (l *RedisLock) LockWithContext(ctx context.Context, attemptInterval time.Duration) error {
	// The wait interval is randomly chosen between [interval * 50%, interval * 150%] to avoid deadlocks under heavy contention.
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

// Unlock releases the lock.
func (l *RedisLock) Unlock() error {
	release := redis.NewScript(`
		if redis.call("get",KEYS[1]) == ARGV[1]
		then
			return redis.call("del",KEYS[1])
		else
			return 0
		end
	`)
	ctx := context.Background() // Unlocking shouldn't be cancelable or timed out here, so use Background directly.
	_, err := release.Run(ctx, l.client, []string{l.key}, l.random).Result()
	if err != nil {
		return err
	}
	return nil
}

// AttemptLock tries to acquire the lock once and returns immediately.
// It only succeeds when the return value is true.
func (l *RedisLock) AttemptLock(ctx context.Context) (bool, error) {
	return l.client.SetNX(ctx, l.key, l.random, l.expire).Result()
}
