package logging_test

import (
	"testing"

	"github.com/go-xlan/redis-go-suo/internal/logging"
	"github.com/stretchr/testify/require"
	"github.com/yyle88/zaplog"
	"go.uber.org/zap"
)

// testLogger implements logging.Logger for testing purposes
// Adds prefix to messages for identification during testing
//
// testLogger 为测试目的实现 logging.Logger
// 为消息添加前缀以便测试期间识别
type testLogger struct {
	prefix string
}

// newTestLogger creates a test logger with specified prefix
// 创建带指定前缀的测试日志记录器
func newTestLogger(prefix string) *testLogger {
	return &testLogger{prefix: prefix}
}

// DebugLog logs debug messages with prefix for testing
// 记录带前缀的调试消息用于测试
func (e *testLogger) DebugLog(msg string, fields ...zap.Field) {
	zaplog.LOGS.Skip(1).Debug(e.prefix+":"+msg, fields...)
}

// ErrorLog logs error messages with prefix for testing
// 记录带前缀的错误消息用于测试
func (e *testLogger) ErrorLog(msg string, fields ...zap.Field) {
	zaplog.LOGS.Skip(1).Error(e.prefix+":"+msg, fields...)
}

// WithMeta creates new test logger with additional fields
// 创建带附加字段的新测试日志记录器
func (e *testLogger) WithMeta(fields ...zap.Field) logging.Logger {
	// For test purposes, return same logger with modified prefix
	// 测试目的，返回带修改前缀的相同日志记录器
	newPrefix := e.prefix + "-with-meta"
	return newTestLogger(newPrefix)
}

// TestNewZapLogger tests the creation of zap-based logger
// 测试基于 zap 的日志记录器创建
func TestNewZapLogger(t *testing.T) {
	logger := logging.NewZapLogger(zaplog.LOGS.Skip(1))
	require.NotNil(t, logger)

	// Test basic logging operations
	// 测试基本日志操作
	logger.DebugLog("test debug message")
	logger.ErrorLog("test error message", zap.String("key", "value"))

	// Test WithMeta functionality
	// 测试 WithMeta 功能
	metaLogger := logger.WithMeta(zap.String("session", "test-session"))
	require.NotNil(t, metaLogger)

	metaLogger.DebugLog("debug with meta")
	metaLogger.ErrorLog("error with meta", zap.Int("code", 500))
}

// TestNewNopLogger tests the creation of no-operation logger
// 测试无操作日志记录器创建
func TestNewNopLogger(t *testing.T) {
	logger := logging.NewNopLogger()
	require.NotNil(t, logger)

	// These should not produce any output
	// 这些不应该产生任何输出
	logger.DebugLog("this should be silent")
	logger.ErrorLog("this should also be silent", zap.String("error", "ignored"))

	// Test WithMeta on nop logger
	// 测试 nop 日志记录器上的 WithMeta
	metaLogger := logger.WithMeta(zap.String("meta", "ignored"))
	require.NotNil(t, metaLogger)

	metaLogger.DebugLog("still silent")
	metaLogger.ErrorLog("still silent too")
}

// TestCustomLoggerImplementation tests custom logger implementation
// 测试自定义日志记录器实现
func TestCustomLoggerImplementation(t *testing.T) {
	customLogger := newTestLogger("custom-prefix")
	require.NotNil(t, customLogger)

	// Test basic operations
	// 测试基本操作
	customLogger.DebugLog("custom debug message")
	customLogger.ErrorLog("custom error message", zap.String("source", "test"))

	// Test WithMeta
	// 测试 WithMeta
	metaLogger := customLogger.WithMeta(zap.String("context", "testing"))
	require.NotNil(t, metaLogger)

	metaLogger.DebugLog("debug with custom meta")
	metaLogger.ErrorLog("error with custom meta", zap.Int("attempt", 1))
}
