package redislock

import (
	"context"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/go-redis/redis/v8"
)

// RedisLockSet is a composite distributed lock backed by a single Redis node.
// Compared with RedisLock, it groups multiple resources into a single lock so they can be locked together.
//
//		// Initialize a lock set containing key1, key2, key3 with a 1 hour expiration
//		lock := NewRedisLockSet(redisClient, []string{"key1", "key2", "key3"}, time.Hour)
//
//		// Retry every 5 seconds until every lock is acquired, or fail after 1 minute
//		err := lock.Lock(time.Second * 5, time.Minute)
//
//		// Release the locks
//		err = lock.Unlock()
//
type RedisLockSet struct {
	keys  []string
	locks []*RedisLock
	rand  *rand.Rand
}

// NewRedisLockSet initializes a composite distributed lock with the specified expiration (automatic release duration).
func NewRedisLockSet(redisClient *redis.Client, keys []string, expire time.Duration) *RedisLockSet {
	locks := []*RedisLock{}
	sort.Strings(keys)
	for _, key := range keys {
		locks = append(locks, NewRedisLock(redisClient, key, expire))
	}
	return &RedisLockSet{keys, locks, rand.New(rand.NewSource(time.Now().UnixNano()))}
}

// Lock retries at the provided interval until every lock is acquired or the timeout elapses.
// The interval jitters randomly within [50%, 150%) to reduce contention.
func (s *RedisLockSet) Lock(attemptInterval time.Duration, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return s.LockWithContext(ctx, attemptInterval)
}

// LockWithContext retries at the provided interval until every lock is acquired.
// The interval jitters randomly within [50%, 150%) to reduce contention.
func (s *RedisLockSet) LockWithContext(ctx context.Context, attemptInterval time.Duration) error {
	// The wait interval is randomly chosen between [interval * 50%, interval * 150%] to avoid deadlocks under heavy contention.
	ticker := time.NewTicker(attemptInterval/2 + time.Duration(s.rand.Int63n(int64(attemptInterval))))
	defer func() {
		ticker.Stop()
	}()
	for {
		success, err := s.AttemptLock(ctx)
		if err != nil {
			return fmt.Errorf("redis-lock-set: failed to acquire locks %v: %v", s.keys, err)
		}
		if success {
			return nil
		}
		<-ticker.C
	}
}

// Unlock releases all locks.
func (s *RedisLockSet) Unlock() error {
	for _, lock := range s.locks {
		err := lock.Unlock()
		if err != nil {
			return err
		}
	}
	return nil
}

// AttemptLock tries to acquire every lock once and returns immediately.
// It only succeeds when every lock is acquired.
func (s *RedisLockSet) AttemptLock(ctx context.Context) (bool, error) {
	for _, lock := range s.locks {
		success, err := lock.AttemptLock(ctx)
		if err != nil {
			s.unlockDontCareErr()
			return false, err
		}
		if !success {
			s.unlockDontCareErr()
			return false, nil
		}
	}
	return true, nil
}

func (s *RedisLockSet) unlockDontCareErr() {
	var wg sync.WaitGroup
	for _, lock := range s.locks {
		wg.Add(1)
		go func(lock *RedisLock) {
			defer wg.Done()
			_ = lock.Unlock()
		}(lock)
	}
	wg.Wait()
}
