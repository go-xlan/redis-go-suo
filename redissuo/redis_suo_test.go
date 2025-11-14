// Package redissuo_test provides comprehensive testing to validate distributed lock operations
// Tests include basic lock acquisition, simultaneous access, timeout handling, and lock extension
// Uses standalone Redis instance to validate lock coordination without depending on outside services
//
// redissuo_test 为分布式锁操作提供全面的测试
// 测试涵盖基本锁获取、并发访问、超时处理和锁延期
// 使用内存 Redis 实例验证锁协调而无需外部依赖
package redissuo_test

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-xlan/redis-go-suo/internal/utils"
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
		Addrs:        []string{miniRedis.Addr()}, //[]string{"127.0.0.1:6379"},
		PoolSize:     10,
		MinIdleConns: 10,
	})
	must.Done(redisClient.Ping(context.Background()).Err())

	caseRedisClient = redisClient

	m.Run()
}

// TestSuoAcquire validates basic lock acquisition and release cycle
// Tests that lock can be obtained and then released without issues
//
// TestSuoAcquire 验证基本的锁获取和释放周期
// 测试锁可以被获取然后正常释放
func TestSuoAcquire(t *testing.T) {
	ctx := context.Background()

	suo := redissuo.NewSuo(caseRedisClient, utils.NewUUID(), 200*time.Millisecond)
	xin, err := suo.Acquire(ctx)
	require.NoError(t, err)
	require.NotNil(t, xin)

	t.Log(time.Until(xin.Expire()))

	success, err := suo.Release(ctx, xin)
	require.NoError(t, err)
	require.True(t, success)
}

// TestSuoAcquireTwice validates that the lock prevents concurrent access with same lock instance
// Tests that when one session owns the lock, a second acquire attempt on same instance gets rejected
// Confirms that just one session can own the lock at a given moment when using the same lock name
//
// TestSuoAcquireTwice 验证相同锁实例的互斥性
// 测试当一个会话拥有锁时，同一实例上的另一个获取尝试会失败
// 验证使用相同锁名时的正确互斥
func TestSuoAcquireTwice(t *testing.T) {
	ctx := context.Background()
	suo := redissuo.NewSuo(caseRedisClient, utils.NewUUID(), 5*time.Second) // Using same key causes conflict

	for i := 0; i < 2; i++ {
		xin, err := suo.Acquire(ctx)
		require.NoError(t, err)
		require.NotNil(t, xin)

		t.Run("SameLock", func(t *testing.T) {
			non, err := suo.Acquire(ctx)
			require.NoError(t, err)
			require.Nil(t, non) // Second acquire on same instance gets rejected
		})

		success, err := suo.Release(ctx, xin)
		require.Nil(t, err)
		require.True(t, success)
	}
}

// TestSuoAcquireTwo validates independent lock operations with different lock instances
// Tests that two locks with different names can be obtained at the same time
// Confirms no conflict happens when using distinct lock identities
//
// TestSuoAcquireTwo 验证不同锁实例的独立操作
// 测试具有不同名称的两个锁可以同时被获取
// 确认使用不同锁标识时不会发生冲突
func TestSuoAcquireTwo(t *testing.T) {
	ctx := context.Background()

	suo1 := redissuo.NewSuo(caseRedisClient, utils.NewUUID(), 5*time.Second)
	xin1, err := suo1.Acquire(ctx)
	require.NoError(t, err)
	require.NotNil(t, xin1)

	suo2 := redissuo.NewSuo(caseRedisClient, utils.NewUUID(), 5*time.Second) // Different key means no conflict
	xin2, err := suo2.Acquire(ctx)
	require.NoError(t, err)
	require.NotNil(t, xin2)

	t.Run("Release1", func(t *testing.T) {
		success, err := suo1.Release(ctx, xin1)
		require.Nil(t, err)
		require.True(t, success)
	})

	t.Run("Release2", func(t *testing.T) {
		success, err := suo2.Release(ctx, xin2)
		require.Nil(t, err)
		require.True(t, success)
	})
}

// TestSuoReleaseTimeout validates lock release past its expiration time
// Tests that releasing an expired lock continues to complete without problems
// Confirms smooth handling when lock has timed out ahead of explicit release
//
// TestSuoReleaseTimeout 验证在过期时间之后释放锁
// 测试释放已过期的锁仍然可以正常完成
// 验证当锁在手动释放之前已超时时的优雅处理
func TestSuoReleaseTimeout(t *testing.T) {
	ctx := context.Background()

	duration := 100 * time.Millisecond

	suo := redissuo.NewSuo(caseRedisClient, utils.NewUUID(), duration)
	xin, err := suo.Acquire(ctx)
	require.NoError(t, err)
	require.NotNil(t, xin)

	time.Sleep(duration) // Wait past lock expiration

	success, err := suo.Release(ctx, xin)
	require.Nil(t, err)
	require.True(t, success) // Release completes even when expired
}

// TestSuo_AcquireAgainExtendLock validates lock TTL extension using same session
// Tests that an active lock can be extended without releasing and re-acquiring
// Verifies the session UUID remains constant while expiration time gets updated
//
// TestSuo_AcquireAgainExtendLock 验证使用相同会话延长锁的 TTL
// 测试活动的锁可以被延长而无需释放和重新获取
// 验证会话 UUID 保持不变的同时过期时间被更新
func TestSuo_AcquireAgainExtendLock(t *testing.T) {
	ctx := context.Background()

	duration := 100 * time.Millisecond

	suo := redissuo.NewSuo(caseRedisClient, utils.NewUUID(), duration)
	xin, err := suo.Acquire(ctx)
	require.NoError(t, err)
	require.NotNil(t, xin)

	time.Sleep(duration * 1 / 3) // Wait one-third of TTL

	t.Log(xin.SessionUUID(), time.Until(xin.Expire()))

	xin, err = suo.AcquireAgainExtendLock(ctx, xin) // Extend the lock
	require.NoError(t, err)
	require.NotNil(t, xin)

	t.Log(xin.SessionUUID(), time.Until(xin.Expire())) // Should show extended time

	time.Sleep(duration * 1 / 3)

	success, err := suo.Release(ctx, xin)
	require.Nil(t, err)
	require.True(t, success)
}
