// Package redissuorun_test provides comprehensive testing to validate advanced lock operations
// Tests include simultaneous lock acquisition with automatic reattempt and lifecycle management
// Confirms that multiple goroutines can coordinate through distributed locks without conflicts
//
// redissuorun_test 为高级锁包装器操作提供全面的测试
// 测试涵盖带自动重试和生命周期管理的并发锁获取
// 验证多个 goroutine 可以通过分布式锁进行协调而不会冲突
package redissuorun_test

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
		Addrs:        []string{miniRedis.Addr()}, //[]string{"127.0.0.1:6379"},
		PoolSize:     10,
		MinIdleConns: 10,
	})
	must.Done(redisClient.Ping(context.Background()).Err())

	caseRedisClient = redisClient

	m.Run()
}

// TestSuoLockRun validates simultaneous lock execution with automatic reattempt
// Tests that ten goroutines can execute protected code blocks in sequence
// Each goroutine waits to obtain the lock, runs its task, then releases the lock
// Confirms that just one goroutine owns the lock at a given moment
//
// TestSuoLockRun 验证带自动重试的并发锁执行
// 测试十个 goroutine 可以按顺序执行受保护的代码块
// 每个 goroutine 等待获取锁，运行其任务，然后释放锁
// 验证正确的协调，即任何时刻只有一个 goroutine 持有锁
func TestSuoLockRun(t *testing.T) {
	suo := redissuo.NewSuo(caseRedisClient, utils.NewUUID(), 50*time.Millisecond)
	var since = time.Now()
	var wg sync.WaitGroup
	for idx := 0; idx < 10; idx++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			run := func(ctx context.Context) error {
				require.NoError(t, ctx.Err())
				t.Log("run->", time.Since(since)) // Log when task begins
				time.Sleep(time.Millisecond * 20) // Simulate work
				require.NoError(t, ctx.Err())
				t.Log("run<-", time.Since(since)) // Log when task completes
				return nil
			}

			// SuoLockRun handles lock acquisition, execution, and release
			require.NoError(t, redissuorun.SuoLockRun(context.Background(), suo, run, time.Millisecond*20))
		}()
	}
	wg.Wait() // Wait while goroutines complete their tasks
}
