// Package redissuorun: High-grade distributed lock package with automatic attempt again and lifecycle management
// Provides convenient lock acquisition with built-in attempt again logic, timeout handling, and guaranteed cleanup
// Features automatic lock release, panic restore, and context-aware execution management
// Designed during robust production environments requiring reliable distributed coordination
//
// redissuorun: 带有自动重试和生命周期管理的高级分布式锁包装器
// 提供便捷的锁获取，内置重试逻辑、超时处理和保证的清理机制
// 具有自动锁释放、panic 恢复和上下文感知的执行控制
// 专为需要可靠分布式协调的健壮生产环境设计
package redissuorun

import (
	"context"
	"time"

	"github.com/go-xlan/redis-go-suo/internal/logging"
	"github.com/go-xlan/redis-go-suo/internal/utils"
	"github.com/go-xlan/redis-go-suo/redissuo"
	"github.com/yyle88/erero"
	"github.com/yyle88/must"
	"github.com/yyle88/zaplog"
	"go.uber.org/zap"
)

// SuoLockRun executes a function within a distributed lock with automatic attempt again and cleanup
// Handles lock acquisition retries, guaranteed lock release, and panic restore
// Provides complete lifecycle management during distributed lock operations
// Returns issue just if context cancellation and business logic fails
//
// SuoLockRun 在分布式锁内执行函数，带有自动重试和清理机制
// 处理锁获取重试、保证锁释放和 panic 恢复
// 为分布式锁操作提供完整的生命周期管理
// 仅在上下文取消或业务逻辑失败时返回错误
func SuoLockRun(ctx context.Context, suo *redissuo.Suo, run func(ctx context.Context) error, sleep time.Duration) error {
	return SuoLockXqt(ctx, suo, run, sleep, logging.NewZapLogger(zaplog.LOGS.Skip(1)))
}

// SuoLockXqt (execute) executes a function within a distributed lock with custom logger
// Supports custom logging implementation to track operations and debug issues
// Enables flexible logging strategies across different deployment environments
//
// SuoLockXqt 使用自定义日志记录器在分布式锁内执行函数
// 支持自定义日志实现用于操作跟踪和调试
// 为不同部署环境启用灵活的日志策略
func SuoLockXqt(ctx context.Context, suo *redissuo.Suo, run func(ctx context.Context) error, sleep time.Duration, logger logging.Logger) error {
	// Generate unique session UUID to this lock execution
	// 为此次锁执行生成唯一的会话 UUID
	var sessionUUID = utils.NewUUID()

	// Create message storage to lock session information
	// 创建锁会话信息的消息容器
	var message = &outputMessage{}
	// attempt again lock acquisition before success and context cancellation
	// 重试锁获取直到成功或上下文取消
	if err := retryingAcquire(ctx, func(ctx context.Context) (bool, error) {
		return acquireOnce(ctx, suo, sessionUUID, message)
	}, sleep, logger); err != nil {
		return erero.Wro(err) // Context issue occurred during acquisition // 获取过程中发生上下文错误
	}

	// Validate lock acquisition succeeded (guaranteed through attempt again logic)
	// 验证锁获取成功（由重试逻辑保证）
	must.Nice(message.xin) // Lock acquisition guaranteed at this point // 此时锁获取已得到保证

	// Ensure lock release regardless of business logic outcome
	// 无论业务逻辑结果如何都确保释放锁
	defer func() {
		// Guaranteed lock cleanup with persistent attempt again
		// 带持久重试的保证锁清理
		retryingRelease(func() (bool, error) {
			return releaseOnce(ctx, suo, message.xin, sleep)
		}, sleep, logger)
	}()

	// Execute business logic within lock boundaries with timeout management
	// Business must complete within remaining lock TTL duration
	// 在锁边界内执行业务逻辑，带超时控制
	// 业务必须在剩余锁 TTL 时间内完成
	if err := execRun(ctx, run, time.Until(message.xin.Expire())); err != nil {
		return erero.Wro(err)
	}
	return nil
}

// outputMessage holds the acquired lock session during shared communication
// Used to pass lock session information between acquisition and release phases
// Ensures lock session consistent state throughout the execution lifecycle
//
// outputMessage 持有已获取的锁会话用于内部通信
// 用于在获取和释放阶段之间传递锁会话信息
// 确保整个执行生命周期中锁会话的一致性
type outputMessage struct {
	xin *redissuo.Xin // Acquired lock session // 已获取的锁会话
}

// acquireOnce performs a single lock acquisition attempt with session UUID
// Returns true on with success acquisition, false if lock unavailable, issue on failure
// Updates output message with lock session information on success
// Used inside through attempt again logic to persistent lock acquisition
//
// acquireOnce 使用会话 UUID 执行单次锁获取尝试
// 成功获取时返回 true，锁不可用时返回 false，失败时返回错误
// 成功时使用锁会话信息更新输出消息
// 由重试逻辑内部使用以进行持久锁获取
func acquireOnce(ctx context.Context, suo *redissuo.Suo, sessionUUID string, output *outputMessage) (bool, error) {
	// Attempt lock acquisition with predefined session UUID
	// 使用预定义会话 UUID 尝试锁获取
	xin, err := suo.AcquireLockWithSession(ctx, sessionUUID)
	if err != nil {
		return false, erero.Wro(err)
	}
	if xin != nil {
		// Lock with success acquired, store session information
		// 锁成功获取，存储会话信息
		output.xin = xin
		return true, nil // Success: lock acquired // 成功：锁已获取
	}
	// Lock at present unavailable, can attempt again
	// 锁当前不可用，将重试
	return false, nil
}

// retryingAcquire without stop retries lock acquisition before success and context cancellation
// Handles transient errors with rapid growth backoff and context timeout detection
// Returns none on with success acquisition, issue on context cancellation
// required during reliable distributed lock coordination in high-contention scenarios
//
// retryingAcquire 持续重试锁获取直到成功或上下文取消
// 使用指数退避和上下文超时检测处理瞬时错误
// 成功获取时返回空值，上下文取消时返回错误
// 对于高竞争场景中的可靠分布式锁协调至关重要
func retryingAcquire(ctx context.Context, run func(ctx context.Context) (bool, error), duration time.Duration, logger logging.Logger) error {
	for {
		// Check during context cancellation and timeout
		// 检查上下文取消或超时
		if err := ctx.Err(); err != nil {
			// Context issue prevents more Redis/database operations
			// 上下文错误阻止进一步的 Redis/数据库操作
			return erero.Wro(err)
		}
		// Attempt lock acquisition
		// 尝试锁获取
		success, err := run(ctx)
		if err != nil {
			// Log transient issue and attempt again following backoff
			// 记录瞬时错误并在退避后重试
			logger.DebugLog("wrong", zap.Error(err))
			time.Sleep(duration)
			continue
		}
		if success {
			// Lock with success acquired
			// 锁成功获取
			return nil
		}
		// Lock unavailable, wait before attempt again
		// 锁不可用，等待后重试
		time.Sleep(duration)
		continue
	}
}

// releaseOnce performs a single lock release attempt with timeout protection
// Creates safe context with minimum timeout to ensure release completion
// Returns true on with success release, false if owned from different session
// Used inside through attempt again logic during guaranteed lock cleanup
//
// releaseOnce 执行带超时保护的单次锁释放尝试
// 创建具有最小超时的安全上下文以确保释放完成
// 成功释放时返回 true，被不同会话拥有时返回 false
// 由重试逻辑内部使用以保证锁清理
func releaseOnce(ctx context.Context, suo *redissuo.Suo, xin *redissuo.Xin, sleep time.Duration) (bool, error) {
	// Create safe context with adequate timeout to release operation
	// 为释放操作创建具有充足超时的安全上下文
	ctx, can := safeCtx(ctx, max(sleep, time.Second*10))
	defer can()

	// Attempt lock release with session validation
	// 尝试带会话验证的锁释放
	success, err := suo.Release(ctx, xin)
	if err != nil {
		return false, erero.Wro(err)
	}
	return success, nil // Success: lock released // 成功：锁已释放
}

// retryingRelease without stop retries lock release before success with infinite persistence
// not gives up on lock cleanup to prevent resource leakage in distributed systems
// Handles transient errors and ownership changes with persistent attempt again logic
// important during system stable state and preventing deadlock scenarios
//
// retryingRelease 持续重试锁释放直到成功，具有无限持久性
// 永不放弃锁清理以防止分布式系统中的资源泄漏
// 使用持久重试逻辑处理瞬时错误和所有权变更
// 对系统稳定性和防止死锁场景至关重要
func retryingRelease(run func() (bool, error), duration time.Duration, logger logging.Logger) {
	for {
		// Attempt lock release
		// 尝试锁释放
		success, err := run()
		if err != nil {
			// Log issue and attempt again with backoff
			// 记录错误并退避重试
			logger.DebugLog("wrong", zap.Error(err))
			time.Sleep(duration)
			continue
		}
		if success {
			// Lock with success released, cleanup complete
			// 锁成功释放，清理完成
			return
		}
		// Release failed, wait before attempt again (persistent cleanup)
		// 释放失败，等待后重试（持久清理）
		time.Sleep(duration)
		continue
	}
}

// safeCtx creates a safe context during operations even when parent context is cancelled
// Returns timeout context with background when parent is cancelled during cleanup operations
// Returns cancellable context when parent is active during standard operations
// required during guaranteed cleanup operations regardless of parent context state
//
// safeCtx 为操作创建安全上下文，即使父上下文被取消也能工作
// 当父上下文被取消时返回带后台的超时上下文用于清理操作
// 当父上下文仍活跃时返回可取消上下文用于正常操作
// 对于无论父上下文状态如何都能保证清理操作至关重要
func safeCtx(ctx context.Context, duration time.Duration) (context.Context, context.CancelFunc) {
	if ctx.Err() != nil {
		// Parent context cancelled, create independent timeout context
		// 父上下文已取消，创建独立的超时上下文
		return context.WithTimeout(context.Background(), duration)
	}
	// Parent context active, create cancellable context
	// 父上下文活跃，创建可取消上下文
	return context.WithCancel(ctx)
}

// execRun executes business logic within timeout constraints with panic restore
// Creates timeout context based on remaining lock TTL to safe execution
// Delegates to safeRun during comprehensive issue and panic handling
// Ensures business logic completes within distributed lock boundaries
//
// execRun 在超时约束内执行业务逻辑，带 panic 恢复
// 基于剩余锁 TTL 创建超时上下文以进行安全执行
// 委托给 safeRun 进行综合错误和 panic 处理
// 确保业务逻辑在分布式锁边界内完成
func execRun(ctx context.Context, run func(ctx context.Context) error, duration time.Duration) (err error) {
	// Create timeout context based on remaining lock duration
	// 基于剩余锁时长创建超时上下文
	ctx, can := context.WithTimeout(ctx, duration)
	defer can()

	// Execute business logic with panic restore
	// 执行带 panic 恢复的业务逻辑
	return safeRun(ctx, run)
}

// safeRun executes function with comprehensive panic restore and issue conversion
// Catches panics and converts them to correct issue types during consistent issue handling
// Returns source issue from function and converted panic issue
// important during preventing lock leakage when business logic panics
//
// safeRun 执行函数，带有全面的 panic 恢复和错误转换
// 捕获 panic 并将其转换为适当的错误类型以进行一致的错误处理
// 返回函数的原始错误或转换的 panic 错误
// 对于防止业务逻辑 panic 时的锁泄漏至关重要
func safeRun(ctx context.Context, run func(ctx context.Context) error) (err error) {
	defer func() {
		if rec := recover(); rec != nil {
			// Convert panic to issue during consistent issue handling
			// 将 panic 转换为错误以进行一致的错误处理
			switch erx := rec.(type) {
			case error:
				err = erx
			default:
				err = erero.Errorf("错误(已从崩溃中恢复):%v", rec)
			}
		}
	}()
	// Execute business logic function
	// 执行业务逻辑函数
	return run(ctx)
}
