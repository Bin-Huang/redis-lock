## Redis-Lock

[中文](README.zh.md)

A small distributed lock built on a single Redis node. Use it to protect resources across multiple instances, pods, or services, and it also supports composite locks.

- Simple: ready to use; the low-level complexity is already handled.
- Lightweight: depends only on one Redis instance (sharing it with your cache is fine).

## Quick Start

### Lock a single resource

```go
// Initialize a lock named resource-key with a 1 hour expiration
lock := NewRedisLock(redisClient, "resource-key", time.Hour * 1)

// Keep retrying every 5 seconds until the lock is acquired or it errors out after 1 minute
err := lock.Lock(time.Second * 5, time.Minute * 1)

// Release the lock
err = lock.Unlock()
```

### Lock multiple resources at once (lock set)

```go
// Initialize a lock set that contains resource1, resource2, resource3 with a 1 hour expiration
lock := NewRedisLockSet(redisClient, []string{"resource1", "resource2", "resource3"}, time.Hour * 1)

// Keep retrying every 5 seconds until all locks are acquired or it errors out after 1 minute
err := lock.Lock(time.Second * 5, time.Minute)

// Release the locks
err = lock.Unlock()
```

## FAQ

### Limitations and scope

Redis-Lock aims to deliver a lightweight, easy-to-use, high-performance distributed lock. To keep that profile, it relies on a single Redis node for storage. When that Redis node goes down (for example, it crashes) the lock becomes unavailable, so it is not suited to workloads that demand strict availability.

For scenarios with hard availability requirements, such as cluster leader election, prefer a multi-node Redlock implementation or coordination systems like ZooKeeper and ETCD. They tolerate partial node failures but require more operational resources, higher development complexity, and some performance trade-offs to achieve their coordination guarantees.

## License

Redis-Lock is distributed under the MIT License. See [LICENSE](LICENSE) for details.
