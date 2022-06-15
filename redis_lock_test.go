package redislock

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
)

func TestSmoking(t *testing.T) {
	key := "redis-lock-test-1"
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	_, err := client.Del(context.Background(), key).Result()
	if err != nil {
		t.Fatal(err)
	}

	lock := NewRedisLock(client, key, time.Hour)

	err = lock.Lock(time.Millisecond*50, time.Second)
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(time.Second)

	err = lock.Unlock()
	if err != nil {
		t.Fatal(err)
	}
}

func TestConcurrencySafety(t *testing.T) {
	key := "redis-lock-test-2"
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	_, err := client.Del(context.Background(), key).Result()
	if err != nil {
		t.Fatal(err)
	}

	expected := 100
	actual := 0 // 用多个协程不断地加1

	var wg sync.WaitGroup
	for i := 0; i < expected; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
			lock := NewRedisLock(client, key, time.Hour)

			err := lock.Lock(time.Millisecond*50, time.Minute)
			if err != nil {
				t.Error(err)
			}

			actual++

			err = lock.Unlock()
			if err != nil {
				t.Error(err)
			}
		}()
	}
	wg.Wait()

	if actual != expected {
		t.Fatalf("expect %v, but got %v", expected, actual)
	}
}

func TestExpire(t *testing.T) {
	key := "redis-lock-test-3"
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	_, err := client.Del(context.Background(), key).Result()
	if err != nil {
		t.Fatal(err)
	}

	lock1 := NewRedisLock(client, key, time.Second*3)

	err = lock1.Lock(time.Millisecond*50, time.Millisecond*500)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err := lock1.Unlock()
		if err != nil {
			t.Fatal(err)
		}
	}()

	time.Sleep(time.Second * 4)

	lock2 := NewRedisLock(client, key, time.Second*3)

	success, err := lock2.AttemptLock(context.Background())
	defer func() {
		err := lock2.Unlock()
		if err != nil {
			t.Fatal(err)
		}
	}()
	if err != nil {
		t.Fatal(err)
	}
	if !success {
		t.Fatalf("lock-expires no works")
	}
}

func TestTimeout(t *testing.T) {
	key := "redis-lock-test-4"
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	_, err := client.Del(context.Background(), key).Result()
	if err != nil {
		t.Fatal(err)
	}

	lock1 := NewRedisLock(client, key, time.Hour)
	err = lock1.Lock(time.Millisecond*50, time.Millisecond*500)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err := lock1.Unlock()
		if err != nil {
			t.Fatal(err)
		}
	}()

	lock2 := NewRedisLock(client, key, time.Hour)
	err = lock2.Lock(time.Millisecond*50, time.Second*3)
	defer func() {
		err := lock2.Unlock()
		if err != nil {
			t.Fatal(err)
		}
	}()
	if err == nil {
		t.Fatalf("lock-timeout no works")
	}
}
