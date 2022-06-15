package redislock

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/go-redis/redis/v8"
)

func TestSetSmoking(t *testing.T) {
	keyPrefix := "redis-lock-test-1-"
	keys := []string{}
	for i := 0; i < 100; i++ {
		keys = append(keys, keyPrefix+strconv.Itoa(i))
	}
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	_, err := client.Del(context.Background(), keyPrefix+"*").Result()
	if err != nil {
		t.Fatal(err)
	}

	lock := NewRedisLockSet(client, keys, time.Hour)

	err = lock.Lock(time.Millisecond*50, time.Second)
	if err != nil {
		t.Fatal(err)
	}

	err = lock.Unlock()
	if err != nil {
		t.Fatal(err)
	}
}

func TestSetExpire(t *testing.T) {
	keyPrefix := "redis-lock-test-3-"
	keys := []string{}
	for i := 0; i < 100; i++ {
		keys = append(keys, keyPrefix+strconv.Itoa(i))
	}
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	_, err := client.Del(context.Background(), keyPrefix+"*").Result()
	if err != nil {
		t.Fatal(err)
	}

	lock1 := NewRedisLockSet(client, keys, time.Second*3)

	err = lock1.Lock(time.Millisecond*500, time.Millisecond*500)
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

	lock2 := NewRedisLockSet(client, keys, time.Second*3)

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

func TestSetTimeout(t *testing.T) {
	keyPrefix := "redis-lock-test-4-"
	keys := []string{}
	for i := 0; i < 100; i++ {
		keys = append(keys, keyPrefix+strconv.Itoa(i))
	}
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	_, err := client.Del(context.Background(), keyPrefix+"*").Result()
	if err != nil {
		t.Fatal(err)
	}

	lock1 := NewRedisLockSet(client, keys, time.Hour)
	err = lock1.Lock(time.Millisecond*500, time.Millisecond*500)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		err := lock1.Unlock()
		if err != nil {
			t.Fatal(err)
		}
	}()
	lock2 := NewRedisLockSet(client, keys, time.Hour)
	err = lock2.Lock(time.Second, time.Second*3)
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

func TestSetConcurrencySafety(t *testing.T) {
	keySet1 := []string{}
	keySet2 := []string{}
	for i := 0; i < 10; i++ {
		keySet1 = append(keySet1, fmt.Sprintf("redis-lock-test-3-%v", i))
		keySet2 = append(keySet2, fmt.Sprintf("redis-lock-test-3-%v", i+5))
	}
	client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
	for _, key := range append(keySet1, keySet2...) {
		_, err := client.Del(context.Background(), key).Result()
		if err != nil {
			t.Fatal(err)
		}
	}

	expected := 100
	actual := 0

	var wg sync.WaitGroup
	for i := 0; i < expected/2; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()

			client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
			lock := NewRedisLockSet(client, keySet1, time.Hour)

			err := lock.Lock(time.Millisecond*50, time.Second*100)
			if err != nil {
				t.Error(err)
			}

			actual++

			err = lock.Unlock()
			if err != nil {
				t.Error(err)
			}
		}()
		go func() {
			defer wg.Done()

			client := redis.NewClient(&redis.Options{Addr: "localhost:6379"})
			lock := NewRedisLockSet(client, keySet2, time.Hour)

			err := lock.Lock(time.Millisecond*50, time.Second*100)
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
