// Package redissuo: Redis distributed lock implementation with Lua scripting to ensure atomic operations
// Provides consistent lock acquisition, release, and extension mechanisms with session management
// Features intelligent timeout handling, backoff logic, and comprehensive logging support
// Supports high-contention scenarios with precise timing coordination and race condition prevention
//
// redissuo: Redis 分布式锁实现，使用 Lua 脚本确保原子操作
// 提供一致的锁获取、释放和延期机制，支持会话管理
// 具有智能超时处理、退避逻辑和完整的日志支持
// 支持高竞争场景，具备精确的时间协调和竞态条件预防
package redissuo

import (
	"context"
	"reflect"
	"strconv"
	"time"

	"github.com/go-xlan/redis-go-suo/internal/logging"
	"github.com/go-xlan/redis-go-suo/internal/utils"
	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
	"github.com/yyle88/erero"
	"github.com/yyle88/must"
	"github.com/yyle88/zaplog"
	"go.uber.org/zap"
)

// Suo represents a Redis distributed lock instance with configurable TTL
// Contains Redis client connection, lock name ID, and expiration duration settings
// Provides core locking operations with atomic Lua-based commands
// Thread-safe when used across multiple goroutines
//
// Suo 代表具有可配置 TTL 的 Redis 分布式锁实例
// 包含 Redis 客户端连接、锁名标识符和过期时长设置
// 提供基于 Lua 原子操作的核心锁定命令
// 在多个 goroutine 中使用时是线程安全的
type Suo struct {
	redisClient redis.UniversalClient // Redis client connection // Redis 客户端连接
	key         string                // Unique lock name ID // 唯一锁名标识符
	ttl         time.Duration         // Lock expiration timeout // 锁过期超时时间
	logger      logging.Logger        // Logger instance used in operations // 操作中使用的日志记录器实例
}

// NewSuo creates a new Redis distributed lock instance with specified parameters
// Validates each input setting and returns configured lock instance
// Settings must be non-empty/non-blank otherwise the function can panic with must.Nice
// Returns prepared distributed lock suitable to use in production environments
//
// NewSuo 使用指定参数创建新的 Redis 分布式锁实例
// 验证每个输入设置并返回配置好的锁实例
// 设置不能为空否则函数会通过 must.Nice 触发 panic
// 返回适用于生产环境的准备就绪分布式锁
func NewSuo(rds redis.UniversalClient, key string, ttl time.Duration) *Suo {
	return &Suo{
		redisClient: must.Nice(rds),                            // Validated Redis client // 经过验证的 Redis 客户端
		key:         must.Nice(key),                            // Validated lock name // 经过验证的锁名
		ttl:         must.Nice(ttl),                            // Validated TTL duration // 经过验证的 TTL 时长
		logger:      logging.NewZapLogger(zaplog.LOGS.Skip(1)), // Default logger // 默认日志记录器
	}
}

// WithLogger sets custom logger during lock operations
// Modifies the current Suo instance and returns it to support method chaining
// Enables injection of custom logging implementation with flexible strategies
//
// WithLogger 为锁操作设置自定义日志记录器
// 修改当前 Suo 实例并返回以支持方法链式调用
// 允许注入自定义日志实现以实现灵活策略
func (o *Suo) WithLogger(logger logging.Logger) *Suo {
	o.logger = logger
	return o
}

const (
	commandAcquire = `if redis.call("GET", KEYS[1]) == ARGV[1] then
    redis.call("SET", KEYS[1], ARGV[1], "PX", ARGV[2])
    return "OK"
else
    return redis.call("SET", KEYS[1], ARGV[1], "NX", "PX", ARGV[2])
end`
)

// acquire attempts to acquire the distributed lock with given session value
// Uses atomic Lua script to prevent race conditions during lock acquisition
// Returns true if lock acquired with success, false if held from different session
// Handles Redis issues and provides detailed logging to assist debugging
//
// acquire 尝试使用给定会话值获取分布式锁
// 使用原子 Lua 脚本防止锁获取过程中的竞态条件
// 如果成功获取锁返回 true，如果被其他会话持有返回 false
// 处理 Redis 问题并提供详细日志来辅助调试
func (o *Suo) acquire(ctx context.Context, value string) (bool, error) {
	must.OK(value) // Validate session value is non-blank // 验证会话值非空

	// Create structured log manager with operation context // 创建带操作上下文的结构化日志记录器
	LOG := o.logger.WithMeta(
		zap.String("action", "申请锁"),
		zap.String("k", o.key),
		zap.String("v", value),
	)

	// Convert TTL to milliseconds to Redis PX argument
	// Redis PX expects milliseconds to set expiration time
	// 将 TTL 转换为毫秒用于 Redis PX 参数
	// Redis PX 期望用毫秒数设置过期时间
	mst := o.ttl.Milliseconds()

	// Execute atomic Lua script with lock name and session parameters
	// 执行带锁名和会话参数的原子 Lua 脚本
	resp, err := o.redisClient.Eval(ctx, commandAcquire, []string{o.key}, []string{value, strconv.FormatInt(mst, 10)}).Result()
	if errors.Is(err, redis.Nil) {
		// Lock held from different session, acquisition failed
		// 锁被其他会话持有，获取失败
		LOG.DebugLog("锁已经被占用-申请不到-请等待释放")
		return false, nil
	} else if err != nil {
		// Redis operation issue occurred during acquisition
		// Redis 操作在获取过程中发生问题
		LOG.ErrorLog("请求报错", zap.Error(err))
		return false, erero.Wro(err)
	} else if resp == nil {
		// Unexpected empty response from Redis
		// Redis 返回意外的空响应
		LOG.ErrorLog("其它错误")
		return false, nil
	}

	// Parse response from Lua script execution
	// 解析 Lua 脚本执行的响应
	msg, ok := resp.(string)
	if !ok {
		// Response type validation failed, unexpected format
		// 响应类型验证失败，格式意外
		LOG.ErrorLog("回复非预期类型", zap.Any("resp", resp), zap.String("resp_type", reflect.TypeOf(resp).String()))
		return false, nil
	}
	if msg != "OK" {
		// Lock acquisition failed, message content mismatch
		// 锁获取失败，消息内容不匹配
		LOG.ErrorLog("消息内容不匹配", zap.String("msg", msg))
		return false, nil
	}
	// Lock with success acquired from current session
	// 当前会话成功获取锁
	LOG.DebugLog("锁已成功申请")
	return true, nil
}

const (
	// 通过官方文档，在 Lua 脚本里判定 redis.call("GET", KEYS[1]) 返回是否为空值，该直接判断结果 true/false，直接不是使用空值判定不存在
	// redis.call("DEL", KEYS[1]) 只会返回 0 或 1，不会有其他返回值
	commandRelease = `local ch = redis.call("GET", KEYS[1])
if (ch == false) then
	return 2
elseif ch == ARGV[1] then
    return redis.call("DEL", KEYS[1])
else
    return 3
end`
)

// release attempts to release the distributed lock using given session value
// Uses atomic Lua script to with safe check ownership before deletion
// Returns true if lock released with success, false if owned from different session
// Provides detailed status codes to distinguish various release scenarios
//
// release 尝试使用给定会话值释放分布式锁
// 使用原子 Lua 脚本在删除前安全检查所有权
// 如果成功释放锁返回 true，如果被不同会话拥有返回 false
// 提供详细状态码以区分各种释放场景
func (o *Suo) release(ctx context.Context, value string) (bool, error) {
	must.OK(value) // Validate session value is non-blank // 验证会话值非空

	// Create structured log manager to release operation // 为释放操作创建结构化日志记录器
	LOG := o.logger.WithMeta(
		zap.String("action", "释放锁"),
		zap.String("k", o.key),
		zap.String("v", value),
	)

	// Execute atomic Lua script to safe lock release
	// 执行原子 Lua 脚本进行安全锁释放
	resp, err := o.redisClient.Eval(ctx, commandRelease, []string{o.key}, []string{value}).Result()
	if err != nil {
		// Redis operation issue during release attempt
		// 释放尝试过程中的 Redis 操作问题
		LOG.ErrorLog("请求报错", zap.Error(err))
		return false, erero.Wro(err)
	} else if resp == nil {
		// Unexpected empty response from Redis
		// Redis 返回意外的空响应
		LOG.ErrorLog("其它错误")
		return false, nil
	}

	// Parse numeric response code from Lua script
	// 解析来自 Lua 脚本的数字响应代码
	num, ok := resp.(int64)
	if !ok {
		// Response type validation failed to release operation
		// 释放操作的响应类型验证失败
		LOG.DebugLog("回复非预期类型", zap.Any("resp", resp), zap.String("resp_type", reflect.TypeOf(resp).String()))
		return false, nil
	}
	// Handle different release status codes from Lua script
	// 处理来自 Lua 脚本的不同释放状态码
	switch num {
	case 0: // Lock was found during GET but failed to DELETE (rare edge case)
		// 在 GET 时找到锁但 DELETE 失败（罕见边缘情况）
		LOG.DebugLog("锁已自动释放")
		return true, nil
	case 1: // standard with success deletion of lock
		// 正常成功删除锁
		LOG.DebugLog("锁已成功释放")
		return true, nil
	case 2: // Key expired auto, lock held too long before release
		// 键自动过期，释放前锁持有时间过长
		LOG.DebugLog("锁不存在-或者锁已自动释放")
		return true, nil
	case 3: // Release failed, lock owned from different session
		// 释放失败，锁被不同会话拥有
		LOG.DebugLog("释放出错-锁被其它线程占用")
		return false, nil
	default: // Unexpected response code from Lua script
		// 来自 Lua 脚本的意外响应码
		LOG.DebugLog("其它错误", zap.Int64("num", num))
		return false, nil
	}
}

// Xin represents an acquired distributed lock session with expiration tracking
// Contains lock identification, session UUID, and conservative expiration estimate
// Provides session management to ensure safe lock operations and extension
// Immutable once created, ensuring consistent lock state throughout usage
//
// Xin 代表具有过期时间跟踪的已获取分布式锁会话
// 包含锁标识、会话 UUID 和保守的过期时间估算
// 提供会话管理来确保安全锁操作和延期
// 创建后不可变，确保使用过程中锁状态的一致性
type Xin struct {
	key         string    // Lock name ID // 锁名标识符
	sessionUUID string    // Current lock session UUID // 当前锁会话 UUID
	expire      time.Time // Conservative expiration estimate // 保守的过期时间估算
}

// SessionUUID returns the unique session ID of this lock instance
// Used during lock ownership checks within release and extension operations
// required to prevent unintended release from different sessions
//
// SessionUUID 返回此锁实例的唯一会话标识符
// 在释放和延期操作时用于锁所有权检查
// 对防止不同会话意外释放锁至关重要
func (s *Xin) SessionUUID() string {
	return s.sessionUUID
}

// Expire returns the conservative expiration time estimate of this lock
// Calculated through subtracting acquisition time from the TTL duration
// Provides safe timing reference when making lock extension decisions
//
// Expire 返回此锁的保守过期时间估算
// 通过从 TTL 时长中减去获取时间来计算
// 在做出锁延期决策时提供安全的时间参考
func (s *Xin) Expire() time.Time {
	return s.expire
}

// AcquireLockWithSession attempts to acquire lock using specified session UUID
// Calculates conservative expiration time through accounting during acquisition duration
// Returns lock session object on success, none if lock unavailable, issue on failure
// Provides precise timing coordination when managing high-performance distributed systems
//
// AcquireLockWithSession 尝试使用指定会话 UUID 获取锁
// 在获取过程中通过考虑耗时计算保守的过期时间
// 成功时返回锁会话对象，锁不可用时返回空值，失败时返回问题
// 在管理高性能分布式系统时提供精确的时间协调
func (o *Suo) AcquireLockWithSession(ctx context.Context, sessionUUID string) (*Xin, error) {
	// Record lock acquisition start time during duration calculation
	// 在耗时计算过程中记录锁获取开始时间
	var startTime = time.Now()
	// Attempt to acquire lock with provided session ID
	// 使用提供的会话标识符尝试获取锁
	if ok, err := o.acquire(ctx, sessionUUID); err != nil {
		return nil, erero.Wro(err)
	} else if !ok {
		return nil, nil
	} else {
		// Calculate conservative expiration time accounting during acquisition overhead
		// 在获取开销过程中计算保守过期时间
		now := time.Now()                  // Current time during conservative calculation // 在保守计算过程中的当前时间
		timeSpent := time.Since(startTime) // Time consumed during acquisition // 获取过程消耗的时间
		remain := o.ttl - timeSpent        // Remaining TTL following acquisition overhead // 减去获取开销后的剩余 TTL
		expire := now.Add(remain)          // Conservative expiration estimate // 保守的过期时间估算
		return &Xin{key: o.key, sessionUUID: sessionUUID, expire: expire}, nil
	}
}

// Acquire attempts to acquire the distributed lock with auto-generated session UUID
// Creates random session ID to enable lock ownership verification
// Convenient method when doing standard lock acquisition without session management
// Returns lock session object on success, none if unavailable, issue on failure
//
// Acquire 尝试使用自动生成的会话 UUID 获取分布式锁
// 创建随机会话标识符来启用锁所有权验证
// 在进行无需会话管理的标准锁获取时使用的便捷方法
// 成功时返回锁会话对象，不可用时返回空值，失败时返回问题
func (o *Suo) Acquire(ctx context.Context) (*Xin, error) {
	// Generate random session UUID to enable lock ownership
	// 生成随机会话 UUID 来启用锁所有权
	var sessionUUID = utils.NewUUID()
	// Acquire lock using generated session ID
	// 使用生成的会话标识符获取锁
	return o.AcquireLockWithSession(ctx, sessionUUID)
}

// Release attempts to release the distributed lock using session information
// Validates lock name consistent state and uses session UUID when checking ownership
// Returns true if released with success, false if owned from different session
// required to ensure safe cleanup and prevent unintended lock interference
//
// Release 尝试使用会话信息释放分布式锁
// 验证锁名一致性并在检查所有权时使用会话 UUID
// 成功释放时返回 true，被不同会话拥有时返回 false
// 对确保安全清理和防止意外锁干扰至关重要
func (o *Suo) Release(ctx context.Context, xin *Xin) (bool, error) {
	// Validate lock name consistent state to ensure safe
	// 验证锁名一致性来确保安全
	must.Equals(xin.key, o.key)
	// Release lock using session UUID when checking ownership
	// 在检查所有权时使用会话 UUID 来释放锁
	return o.release(ctx, xin.sessionUUID)
}

// AcquireAgainExtendLock extends the lock through re-acquiring with same session UUID
// Validates lock name consistent state and extends TTL using existing session ID
// Returns new lock session with updated expiration time when extension succeeds
// important when managing long-running operations that need extended lock duration
//
// AcquireAgainExtendLock 通过使用相同会话 UUID 重新获取来延期锁
// 验证锁名一致性并使用现有会话标识符扩展 TTL
// 延期成功时返回具有更新过期时间的新锁会话
// 在管理需要延长锁持有时间的长期运行操作时至关重要
func (o *Suo) AcquireAgainExtendLock(ctx context.Context, xin *Xin) (*Xin, error) {
	// Validate lock name consistent state to ensure extension safe
	// 验证锁名一致性来确保延期安全
	must.Equals(xin.key, o.key)
	// Re-acquire lock with same session UUID to extend expiration
	// 使用相同会话 UUID 重新获取锁以延长过期时间
	return o.AcquireLockWithSession(ctx, xin.sessionUUID)
}
