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

// RedisLockSet 是基于 Redis 单节点的分布式组合锁。和 RedisLock 相比，RedisLockSet 将多个资源组合为同一个锁，可以方便地同时锁住多个资源。
//
//		// 初始化一个包含 key1、key2、key3 的组合锁，指定过期时间为 1 小时
//		lock := NewRedisLockSet(redisClient, []string{"key1", "key2", "key3"}, time.Hour)
//
//		// 尝试加锁并堵塞。以 5 秒为间隔、不断地尝试加锁，直到加锁成功、或超过 1 分钟报错
//		err := lock.Lock(time.Second * 5, time.Minute)
//
//		// 释放锁
//		err = lock.Unlock()
//
type RedisLockSet struct {
	keys  []string
	locks []*RedisLock
	rand  *rand.Rand
}

// NewRedisLockSet 初始化一个分布式组合锁，并指定过期时间（即加锁后的自动释放时间）
func NewRedisLockSet(redisClient *redis.Client, keys []string, expire time.Duration) *RedisLockSet {
	locks := []*RedisLock{}
	sort.Strings(keys)
	for _, key := range keys {
		locks = append(locks, NewRedisLock(redisClient, key, expire))
	}
	return &RedisLockSet{keys, locks, rand.New(rand.NewSource(time.Now().UnixNano()))}
}

// Lock 以特定的时间间隔、不断地尝试加锁，直到加锁成功或者超时报错。其中，为了错开竞争，时间间隔会在 [50%, 150%) 区间随机波动。
func (s *RedisLockSet) Lock(attemptInterval time.Duration, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return s.LockWithContext(ctx, attemptInterval)
}

// LockWithContext 以特定的时间间隔、不断地尝试加锁，直到加锁成功。其中，为了错开竞争，时间间隔会在 [50%, 150%) 区间随机波动。
func (s *RedisLockSet) LockWithContext(ctx context.Context, attemptInterval time.Duration) error {
	// 等待时间在 [ interval * 50%, interval * 150% ] 区间之间随机获得，以防止多线程同时竞争导致的死锁
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

// Unlock 释放锁
func (s *RedisLockSet) Unlock() error {
	for _, lock := range s.locks {
		err := lock.Unlock()
		if err != nil {
			return err
		}
	}
	return nil
}

// AttemptLock 尝试加锁，并立即返回结果。只有在返回 true 的情况下，才认为加锁成功。
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
