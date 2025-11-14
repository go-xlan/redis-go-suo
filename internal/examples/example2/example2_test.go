// Package example2 demonstrates advanced distributed lock usage with automatic reattempt
// Shows concurrent goroutine coordination, lock extension, and context management
// Illustrates the advanced lock workflow in complex scenarios
//
// example2 演示带自动重试的高级分布式锁用法
// 展示并发 goroutine 协调、锁延期和上下文管理
// 说明复杂场景中的高级锁工作流程
package example2

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-xlan/redis-go-suo/internal/utils"
	"github.com/go-xlan/redis-go-suo/redissuo"
	"github.com/go-xlan/redis-go-suo/redissuorun"
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

// TestLockWithAutomaticReattempt demonstrates the SuoLockRun pattern
// Shows how multiple goroutines can execute protected code blocks in sequence
// Each goroutine waits to obtain the lock, runs its task, then releases the lock
//
// TestLockWithAutomaticReattempt 演示 SuoLockRun 模式
// 展示多个 goroutine 如何按顺序执行受保护的代码块
// 每个 goroutine 等待获取锁、运行其任务、然后释放锁
func TestLockWithAutomaticReattempt(t *testing.T) {
	suo := redissuo.NewSuo(caseRedisClient, utils.NewUUID(), 50*time.Millisecond)
	var since = time.Now()
	var wg sync.WaitGroup

	for idx := 0; idx < 5; idx++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			run := func(ctx context.Context) error {
				require.NoError(t, ctx.Err())
				t.Logf("Goroutine %d started at %v", id, time.Since(since))
				time.Sleep(20 * time.Millisecond) // Simulate work
				t.Logf("Goroutine %d finished at %v", id, time.Since(since))
				return nil
			}

			// SuoLockRun handles lock acquisition with automatic reattempt
			// SuoLockRun 处理带自动重试的锁获取
			require.NoError(t, redissuorun.SuoLockRun(context.Background(), suo, run, 20*time.Millisecond))
		}(idx)
	}

	wg.Wait()
	t.Log("Each goroutine completed its task")
}

// TestLockExtension demonstrates extending a lock before its expiration
// Shows how to keep a lock active while long-running operations execute
// Prevents premature lock expiration during extended processing
//
// TestLockExtension 演示在过期之前延长锁
// 展示如何在长时间运行的操作执行时保持锁活跃
// 防止在扩展处理期间锁过早过期
func TestLockExtension(t *testing.T) {
	ctx := context.Background()

	// Create a lock with short TTL
	// 创建一个具有短 TTL 的锁
	lock := redissuo.NewSuo(caseRedisClient, "example-lock-extension", 100*time.Millisecond)

	session, err := lock.Acquire(ctx)
	require.NoError(t, err)
	require.NotNil(t, session)

	t.Logf("Lock obtained, expires at: %s", session.Expire().Format(time.RFC3339))

	// Perform some work
	// 执行一些工作
	time.Sleep(40 * time.Millisecond)
	t.Log("Completed first part of work")

	// Extend the lock before it expires
	// 在锁过期之前延长它
	session, err = lock.AcquireAgainExtendLock(ctx, session)
	require.NoError(t, err)
	require.NotNil(t, session)

	t.Logf("Lock extended, new expiration: %s", session.Expire().Format(time.RFC3339))

	// Perform more work with extended time
	// 使用延长的时间执行更多工作
	time.Sleep(40 * time.Millisecond)
	t.Log("Completed second part of work")

	// Release the lock
	// 释放锁
	success, err := lock.Release(ctx, session)
	require.NoError(t, err)
	require.True(t, success)

	t.Log("Lock released following extension")
}

// TestContextCancellation demonstrates handling of context cancellation
// Shows how to respect context timeouts and cancellations in lock operations
// Ensures that operations stop when context is cancelled
//
// TestContextCancellation 演示上下文取消的正确处理
// 展示如何在锁操作中尊重上下文超时和取消
// 确保在上下文被取消时操作停止
func TestContextCancellation(t *testing.T) {
	// Create a context with timeout
	// 创建一个带超时的上下文
	ctx, can := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer can()

	lock := redissuo.NewSuo(caseRedisClient, "example-lock-timeout", 5*time.Second)

	session, err := lock.Acquire(ctx)
	require.NoError(t, err)
	require.NotNil(t, session)

	t.Log("Lock obtained")

	// Wait past context timeout
	// 等待超过上下文超时
	time.Sleep(100 * time.Millisecond)

	// Check that context is cancelled
	// 检查上下文是否被取消
	require.Error(t, ctx.Err())
	t.Log("Context cancelled as expected")

	// Release with background context (since request context is cancelled)
	// 使用后台上下文释放（因为请求上下文已取消）
	success, err := lock.Release(context.Background(), session)
	require.NoError(t, err)
	require.True(t, success)

	t.Log("Lock released using background context")
}

// TestConcurrentLockCoordination demonstrates multiple goroutines coordinating through locks
// Shows that goroutines execute in sequence when competing to get the same lock
// Verifies that protected operations execute without concurrent access
//
// TestConcurrentLockCoordination 演示多个 goroutine 通过锁进行协调
// 展示当竞争获取同一个锁时 goroutine 按顺序执行
// 验证受保护操作在没有并发访问的情况下执行
func TestConcurrentLockCoordination(t *testing.T) {
	suo := redissuo.NewSuo(caseRedisClient, "example-lock-concurrent", 100*time.Millisecond)

	var counter int
	var mu sync.Mutex
	var wg sync.WaitGroup

	// Launch multiple goroutines
	// 启动多个 goroutine
	for idx := 0; idx < 3; idx++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			run := func(ctx context.Context) error {
				// Increment counter (protected code section)
				// 增加计数器（受保护代码段）
				mu.Lock()
				counter++
				current := counter
				mu.Unlock()

				t.Logf("Goroutine %d executing with counter=%d", id, current)
				time.Sleep(30 * time.Millisecond)
				return nil
			}

			require.NoError(t, redissuorun.SuoLockRun(context.Background(), suo, run, 20*time.Millisecond))
		}(idx)
	}

	wg.Wait()

	// Check that each goroutine completed
	// 验证每个 goroutine 都已完成
	require.Equal(t, 3, counter)
	t.Logf("Each of %d goroutines completed with coordination", counter)
}
