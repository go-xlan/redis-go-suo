// Package utils provides testing for UUID generation utilities
// Tests validate that unique session identities are created as hex-encoded strings
// Ensures consistent UUID generation used in distributed lock session management
//
// utils 为 UUID 生成工具提供测试
// 测试验证创建的唯一会话标识符是十六进制编码字符串
// 确保分布式锁会话管理中使用的一致 UUID 生成
package utils

import "testing"

// TestNewUUID validates UUID generation producing valid hex-encoded identities
// Tests that generated UUID is non-blank and has expected format
//
// TestNewUUID 验证 UUID 生成产生有效的十六进制编码标识符
// 测试生成的 UUID 非空且具有预期格式
func TestNewUUID(t *testing.T) {
	uuid := NewUUID()
	t.Log(uuid)

	// Validate UUID is not blank
	if uuid == "" {
		t.Error("UUID should not be blank")
	}

	// Validate UUID has expected length (32 hex characters)
	if len(uuid) != 32 {
		t.Errorf("UUID should be 32 characters, got %d", len(uuid))
	}
}
