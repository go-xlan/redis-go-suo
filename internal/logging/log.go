// Package logging: Flexible logging interface of distributed lock operations
// Provides pluggable logging with support on custom implementations
// Enables context-aware logging with structured fields and graded output
// Designed to serve production environments requiring flexible logging strategies
//
// logging: 分布式锁操作的灵活日志接口
// 提供可插拔的日志记录，支持自定义实现
// 支持带结构化字段和分级输出的上下文感知日志
// 专为需要灵活日志策略的生产环境设计
package logging

import (
	"go.uber.org/zap"
)

// Logger defines the interface for lock operation logging
// Provides structured logging methods with field support
// Enables custom implementations across different logging backends
// Supports both debug and error-level logging in lock operations
//
// Logger 定义锁操作日志记录的接口
// 提供带字段支持的结构化日志方法
// 支持不同日志后端的自定义实现
// 支持锁操作的调试和错误级别日志
type Logger interface {
	// DebugLog logs debug-level messages with optional fields
	// 记录带可选字段的调试级别消息
	DebugLog(msg string, fields ...zap.Field)

	// ErrorLog logs error-level messages with optional fields
	// 记录带可选字段的错误级别消息
	ErrorLog(msg string, fields ...zap.Field)

	// WithMeta creates a new logger with additional fields
	// 创建带附加字段的新日志记录器
	WithMeta(fields ...zap.Field) Logger
}

// zapLogger implements Logger using zaplog in standard operations
// Wraps zaplog functions to provide consistent logging interface
// Supports structured logging with contextual fields
//
// zapLogger 使用 zaplog 实现 Logger 用于标准操作
// 包装 zaplog 功能以提供一致的日志接口
// 支持带上下文字段的结构化日志
type zapLogger struct {
	logger *zap.Logger
}

// NewZapLogger creates a logger with a custom zap.Logger instance
// Enables complete control over logging configuration
// Supports custom encoders, outputs, and filtering rules
//
// NewZapLogger 使用自定义 zap.Logger 实例创建日志记录器
// 实现对日志配置的完全控制
// 支持自定义编码器、输出和过滤规则
func NewZapLogger(logger *zap.Logger) Logger {
	return &zapLogger{
		logger: logger,
	}
}

// DebugLog logs debug-level messages with structured fields
// Used to show detailed operation info during development
//
// DebugLog 记录带结构化字段的调试级别消息
// 用于开发期间的详细操作信息
func (l *zapLogger) DebugLog(msg string, fields ...zap.Field) {
	l.logger.Debug(msg, fields...)
}

// ErrorLog logs error-level messages with structured fields
// Used when operation errors require attention
//
// ErrorLog 记录带结构化字段的错误级别消息
// 用于需要关注的操作错误
func (l *zapLogger) ErrorLog(msg string, fields ...zap.Field) {
	l.logger.Error(msg, fields...)
}

// WithMeta creates a new logger with additional context fields
// Returns a new Logger instance with fields applied to all messages
// Convenient for adding operation context like lock keys and session IDs
//
// WithMeta 创建带附加上下文字段的新日志记录器
// 返回将字段应用于所有消息的新 Logger 实例
// 用于添加操作上下文如锁键和会话 ID
func (l *zapLogger) WithMeta(fields ...zap.Field) Logger {
	return &zapLogger{
		logger: l.logger.With(fields...),
	}
}

// NopLogger implements Logger with no-operation methods
// Provides silent logging when testing and disabled logging scenarios
// All methods are no-ops, producing no output
//
// NopLogger 使用无操作方法实现 Logger
// 为测试或禁用日志场景提供静默日志记录
// 所有方法都是无操作，不产生输出
type NopLogger struct{}

// NewNopLogger creates a logger that discards all messages
// Returns a Logger that performs no logging operations
// Convenient for tests or when logging should be disabled
//
// NewNopLogger 创建一个丢弃所有消息的日志记录器
// 返回不执行日志操作的 Logger
// 用于测试或需要禁用日志时
func NewNopLogger() Logger {
	return NewZapLogger(zap.NewNop())
}
