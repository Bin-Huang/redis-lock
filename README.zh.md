## Redis-Lock

[English](README.md)

一个简洁的基于 Redis 单节点的分布式锁实现，可以用在多个实例、多个 pods、多个服务之间并发冲突的加锁场景，也支持组合锁。

- 简单：直接拿来用，底层的复杂性都封装好了
- 轻量：不依赖太多外部设施资源（只需要一个 Redis，和缓存共用也行）

## 快速上手

### 给单个资源加锁（单锁）

```go
// 初始化一个名为 resource-key 的锁，指定过期时间为 1 小时
lock := NewRedisLock(redisClient, "resource-key", time.Hour * 1)

// 堵塞当前协程、直至加锁成功或超时。这里以 5 秒为间隔、不断地尝试加锁，直到加锁成功、或超过 1 分钟报错
err := lock.Lock(time.Second * 5, time.Minute * 1)

// 释放锁
err = lock.Unlock()
```

### 同时给多个资源加锁（组合锁）

```go
// 初始化一个包含 resource1, resource2, resource3 的组合锁，指定过期时间为 1 小时
lock := NewRedisLockSet(redisClient, []string{"resource1", "resource2", "resource3"}, time.Hour * 1)

// 堵塞当前协程、直至加锁成功或超时。这里以 5 秒为间隔、不断地尝试加锁，直到加锁成功、或超过 1 分钟报错
err := lock.Lock(time.Second * 5, time.Minute)

// 释放锁
err = lock.Unlock()
```

## 常见问题

### 局限和适用范围

Redis-Lock 希望提供一个简单、轻量、高性能的分布式锁实现。为了满足轻量、易用、高性能的定位，这个分布式锁在存储上依赖了 Redis 单节点。当 Redis 节点不可用（例如宕机）时，分布式锁也会不可用，因此不适用于对可用性有较高要求的场景。

对于有严苛可用性标准的场景，比如集群选举，基于 Redis 多节点的 Redlock 算法实现、或者 ZooKeeper、ETCD 等基础设施更加合适，它们可以承受少数节点的宕机问题。当然，这些方案对开发复杂度、外部设施资源也有更多的要求，并且为了实现分布式协调算法而损耗一定的性能。

## 许可证

Redis-Lock 以 MIT License 授权，详见 [LICENSE](LICENSE)。
