// Package example1 demonstrates basic distributed lock usage with automatic release
// Shows simple lock acquisition, protected code execution, and guaranteed cleanup
// Illustrates the essential lock workflow in production applications
//
// example1 演示带自动释放的基本分布式锁用法
// 展示简单的锁获取、受保护代码执行和保证的清理
// 说明实际应用中的基本锁工作流程
package example1

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-xlan/redis-go-suo/redissuo"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/yyle88/must"
	"github.com/yyle88/rese"
)

var caseRedisClient redis.UniversalClient

func TestMain(m *testing.M) {
	miniRedis := rese.P1(miniredis.Run())
	defer miniRedis.Close()

	redisClient := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:        []string{miniRedis.Addr()},
		PoolSize:     10,
		MinIdleConns: 10,
	})
	must.Done(redisClient.Ping(context.Background()).Err())

	caseRedisClient = redisClient

	m.Run()
}

// TestBasicLockUsage demonstrates the basic lock acquisition and release pattern
// Shows how to obtain a lock, execute protected code, and release it
//
// TestBasicLockUsage 演示基本的锁获取和释放模式
// 展示如何获取锁、执行受保护代码并释放它
func TestBasicLockUsage(t *testing.T) {
	ctx := context.Background()

	// Create a distributed lock with 5-second TTL
	// 创建一个具有 5 秒 TTL 的分布式锁
	lock := redissuo.NewSuo(caseRedisClient, "example-lock-1", 5*time.Second)

	// Acquire the lock
	// 获取锁
	session, err := lock.Acquire(ctx)
	require.NoError(t, err)
	require.NotNil(t, session)

	t.Logf("Lock obtained with session: %s", session.SessionUUID())
	t.Logf("Lock expires at: %s", session.Expire().Format(time.RFC3339))

	// Execute protected code
	// 执行受保护的代码
	t.Log("Executing protected operation...")
	time.Sleep(100 * time.Millisecond) // Simulate work

	// Release the lock
	// 释放锁
	success, err := lock.Release(ctx, session)
	require.NoError(t, err)
	require.True(t, success)

	t.Log("Lock released")
}

// TestLockWithDefer demonstrates using defer to guarantee lock release
// Shows the recommended pattern to ensure cleanup even when panics happen
//
// TestLockWithDefer 演示使用 defer 保证锁释放
// 展示推荐的模式以确保即使发生 panic 也能清理
func TestLockWithDefer(t *testing.T) {
	ctx := context.Background()

	lock := redissuo.NewSuo(caseRedisClient, "example-lock-2", 5*time.Second)

	session, err := lock.Acquire(ctx)
	require.NoError(t, err)
	require.NotNil(t, session)

	// Use defer to guarantee lock release
	// 使用 defer 保证锁释放
	defer func() {
		success, err := lock.Release(context.Background(), session)
		require.NoError(t, err)
		require.True(t, success)
		t.Log("Lock cleanup completed")
	}()

	t.Log("Working with the lock...")
	time.Sleep(100 * time.Millisecond)

	t.Log("Protected operation finished")
}

// TestLockContention demonstrates what occurs when two processes compete to get the same lock
// Shows that the second acquire attempt gets nothing when the lock is taken
//
// TestLockContention 演示两个进程竞争获取同一个锁时会发生什么
// 展示当锁被占用时第二个获取尝试得不到锁
func TestLockContention(t *testing.T) {
	ctx := context.Background()

	lock := redissuo.NewSuo(caseRedisClient, "example-lock-3", 5*time.Second)

	// First process obtains the lock
	// 第一个进程获取锁
	session1, err := lock.Acquire(ctx)
	require.NoError(t, err)
	require.NotNil(t, session1)
	t.Log("First process obtained the lock")

	// Second process attempts to obtain the same lock
	// 第二个进程尝试获取同一个锁
	session2, err := lock.Acquire(ctx)
	require.NoError(t, err)
	require.Nil(t, session2) // Should be nil because lock is taken
	t.Log("Second process failed to obtain the lock (expected)")

	// Release the first lock
	// 释放第一个锁
	success, err := lock.Release(ctx, session1)
	require.NoError(t, err)
	require.True(t, success)
	t.Log("First process released the lock")

	// Now the second process can obtain the lock
	// 现在第二个进程可以获取锁了
	session3, err := lock.Acquire(ctx)
	require.NoError(t, err)
	require.NotNil(t, session3)
	t.Log("Second process obtained the lock once first released")

	// Cleanup
	// 清理
	success, err = lock.Release(ctx, session3)
	require.NoError(t, err)
	require.True(t, success)
}

// TestLockReleaseAndReacquire demonstrates lock release and subsequent acquisition
// Shows that once a lock is released, it becomes available to get again
// Illustrates the complete lifecycle: acquire -> work -> release -> acquire again
//
// TestLockReleaseAndReacquire 演示锁释放和后续获取
// 展示一旦锁被释放，它就可以被再次获取
// 说明完整的生命周期：获取 -> 工作 -> 释放 -> 再次获取
func TestLockReleaseAndReacquire(t *testing.T) {
	ctx := context.Background()
	lockName := "example-lock-4"

	var session1UUID string

	t.Run("FirstAcquisition", func(t *testing.T) {
		lock := redissuo.NewSuo(caseRedisClient, lockName, 5*time.Second)

		// Acquire the lock
		// 获取锁
		session, err := lock.Acquire(ctx)
		require.NoError(t, err)
		require.NotNil(t, session)
		session1UUID = session.SessionUUID()
		t.Logf("Lock obtained with session: %s", session1UUID)

		// Perform work
		// 执行工作
		time.Sleep(50 * time.Millisecond)
		t.Log("Work completed")

		// Release the lock
		// 释放锁
		success, err := lock.Release(ctx, session)
		require.NoError(t, err)
		require.True(t, success)
		t.Log("Lock released")
	})

	var session2UUID string

	t.Run("SecondAcquisition", func(t *testing.T) {
		lock := redissuo.NewSuo(caseRedisClient, lockName, 5*time.Second)

		// Acquire the same lock again (should succeed because previous lock was released)
		// 再次获取同一个锁（应该成功因为之前的锁已释放）
		session, err := lock.Acquire(ctx)
		require.NoError(t, err)
		require.NotNil(t, session)
		session2UUID = session.SessionUUID()
		t.Logf("Lock acquired again with session: %s", session2UUID)

		// Release the lock
		// 释放锁
		success, err := lock.Release(ctx, session)
		require.NoError(t, err)
		require.True(t, success)
		t.Log("Lock released")
	})

	// Check we have a different session
	// 验证我们有一个不同的会话
	require.NotEqual(t, session1UUID, session2UUID)
	t.Logf("Session changed: %s -> %s", session1UUID, session2UUID)
}
