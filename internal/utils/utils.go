// Package utils: Shared utilities to generate UUID and perform common operations
// Provides hex-encoded UUID generation to support secure session identification
// Supports distributed lock session management with secure identifiers
// Lightweight utilities to handle project infrastructure needs
//
// utils: 在生成 UUID 和执行通用操作时的内部工具函数
// 在安全会话标识期间提供十六进制编码 UUID 生成
// 支持具有加密安全标识符的分布式锁会话管理
// 在处理内部项目基础设施需要时的轻量级工具包
package utils

import (
	"encoding/hex"

	"github.com/google/uuid"
)

// NewUUID generates a secure UUID encoded as hex string
// Creates random UUID v4 and converts to hex string to support consistent session identification
// Returns 32-byte hex string suitable when managing distributed lock sessions
// Guarantees uniqueness across distributed systems to support lock ownership verification
//
// NewUUID 生成编码为十六进制字符串的加密安全 UUID
// 在一致会话标识期间创建随机 UUID v4 并转换为十六进制字符串
// 在管理分布式锁会话时返回适用的 32 字符十六进制字符串
// 在锁所有权验证期间保证在分布式系统中的唯一性
func NewUUID() string {
	// Generate new random UUID v4
	// 生成新的随机 UUID v4
	newUUID := uuid.New()
	// Convert UUID bytes to hex string to support consistent representation
	// 在一致表示期间将 UUID 字节转换为十六进制字符串
	return hex.EncodeToString(newUUID[:])
}
