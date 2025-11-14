[![GitHub Workflow Status (branch)](https://img.shields.io/github/actions/workflow/status/go-xlan/redis-go-suo/release.yml?branch=main&label=BUILD)](https://github.com/go-xlan/redis-go-suo/actions/workflows/release.yml?query=branch%3Amain)
[![GoDoc](https://pkg.go.dev/badge/github.com/go-xlan/redis-go-suo)](https://pkg.go.dev/github.com/go-xlan/redis-go-suo)
[![Coverage Status](https://img.shields.io/coveralls/github/go-xlan/redis-go-suo/main.svg)](https://coveralls.io/github/go-xlan/redis-go-suo?branch=main)
[![Supported Go Versions](https://img.shields.io/badge/Go-1.22--1.25-lightgrey.svg)](https://github.com/go-xlan/redis-go-suo)
[![GitHub Release](https://img.shields.io/github/release/go-xlan/redis-go-suo.svg)](https://github.com/go-xlan/redis-go-suo/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/go-xlan/redis-go-suo)](https://goreportcard.com/report/github.com/go-xlan/redis-go-suo)

# redis-go-suo

基于 Lua 脚本的 Redis 分布式锁实现，确保原子操作。

---

<!-- TEMPLATE (ZH) BEGIN: LANGUAGE NAVIGATION -->
## 英文文档

[ENGLISH README](README.md)
<!-- TEMPLATE (ZH) END: LANGUAGE NAVIGATION -->

## 核心特性

🔐 **原子锁操作**: 基于 Lua 脚本的锁获取和释放，防止竞态条件
⚡ **智能会话管理**: 基于 UUID 的会话跟踪和所有权验证
🔄 **自动重复机制**: 内置重复逻辑，支持渐进退避策略应对高竞争场景
🛡️ **生命周期管理**: 保证锁清理，支持 panic 处理和超时管理
📊 **灵活日志系统**: 可插拔日志接口，支持自定义实现

## 安装

```bash
go get github.com/go-xlan/redis-go-suo
```

## 快速开始

### 基础用法

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-xlan/redis-go-suo/redissuo"
	"github.com/redis/go-redis/v9"
	"github.com/yyle88/rese"
)

func main() {
	// Start Redis instance to show demo
	miniRedis := rese.P1(miniredis.Run())
	defer miniRedis.Close()

	// Setup Redis connection
	redisClient := redis.NewClient(&redis.Options{
		Addr: miniRedis.Addr(),
	})
	defer rese.F0(redisClient.Close)

	// Init shared lock
	lock := redissuo.NewSuo(redisClient, "demo-lock", time.Minute*5)

	// Get lock
	ctx := context.Background()
	session, err := lock.Acquire(ctx)
	if err != nil {
		panic(err)
	}
	if session == nil {
		fmt.Println("Lock taken - used in different process")
		return
	}

	fmt.Printf("Lock acquired! Session: %s\n", session.SessionUUID())
	fmt.Printf("Lock timeout at: %s\n", session.Expire().Format(time.RFC3339))

	// Run protected code
	fmt.Println("Running protected zone...")
	time.Sleep(time.Second * 2) // Mock task

	// Free lock
	success, err := lock.Release(ctx, session)
	if err != nil {
		panic(err)
	}

	if success {
		fmt.Println("Lock released!")
	} else {
		fmt.Println("Lock release failed - might be released via timeout in different session")
	}
}
```

⬆️ **源码:** [源码](internal/demos/demo1x/main.go)

### 高端接口用法

```go
package main

import (
	"context"
	"fmt"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/go-xlan/redis-go-suo/redissuo"
	"github.com/go-xlan/redis-go-suo/redissuorun"
	"github.com/redis/go-redis/v9"
	"github.com/yyle88/rese"
)

func main() {
	// Start Redis instance to show demo
	miniRedis := rese.P1(miniredis.Run())
	defer miniRedis.Close()

	// Setup Redis connection
	redisClient := redis.NewClient(&redis.Options{
		Addr: miniRedis.Addr(),
	})
	defer rese.F0(redisClient.Close)

	// Init shared lock
	lock := redissuo.NewSuo(redisClient, "app-lock", time.Minute*2)

	fmt.Println("Beginning high-level lock operation...")

	// Run function with auto lock handling
	err := redissuorun.SuoLockRun(context.Background(), lock, func(ctx context.Context) error {
		fmt.Println("Running protected zone with lock shield")
		fmt.Println("Handling main business code...")

		// Mock task that needs exclusive access
		for i := 1; i <= 5; i++ {
			fmt.Printf("Phase %d/5 working...\n", i)
			time.Sleep(time.Millisecond * 300)
		}

		fmt.Println("Business code finished!")
		return nil
	}, time.Millisecond*100) // Wait time

	if err != nil {
		fmt.Printf("Lock action failed: %v\n", err)
		return
	}

	fmt.Println("Lock action finished!")
}
```

⬆️ **源码:** [源码](internal/demos/demo2x/main.go)

## 核心组件

### redissuo 包

提供核心分布式锁操作的基础包：

- **`Suo`**: 主要锁结构，包含 Redis 客户端、键和 TTL 配置
- **`Xin`**: 锁会话表示，包含 UUID 和过期时间跟踪
- **原子操作**: 基于 Lua 脚本的获取/释放操作，确保一致性

### redissuorun 包

提供生命周期管理的高端接口：

- **`SuoLockRun`**: 在锁边界内执行函数，支持自动重复
- **`SuoLockXqt`**: 支持自定义日志记录器的扩展版本
- **Panic 处理**: 自动 panic 处理和锁清理
- **上下文管理**: 超时和取消支持

## 高级功能

### 锁延期

```go
// 延期现有锁会话
extendedSession, err := lock.AcquireAgainExtendLock(ctx, session)
if err != nil {
    // 处理延期失败
}
// 使用 extendedSession 继续操作
```

### 自定义日志

```go
// 创建自定义日志记录器
customLogger := logging.NewZapLogger(yourZapLogger)

// 使用自定义日志记录器
err := redissuorun.SuoLockXqt(ctx, lock, businessLogic, retryInterval, customLogger)
```

### 会话管理

```go
// 使用特定会话 UUID 获取锁
sessionUUID := "your-custom-session-id"
session, err := lock.AcquireLockWithSession(ctx, sessionUUID)

// 访问会话信息
fmt.Printf("会话 UUID: %s\n", session.SessionUUID())
fmt.Printf("过期时间: %s\n", session.Expire())
```

## 配置示例

### Redis 集群设置

```go
rdb := redis.NewClusterClient(&redis.ClusterOptions{
    Addrs: []string{"localhost:7000", "localhost:7001", "localhost:7002"},
})

lock := redissuo.NewSuo(rdb, "cluster-lock", time.Minute*10)
```

### 自定义日志配置

```go
import "go.uber.org/zap"

// 创建自定义 zap 日志记录器
logger, _ := zap.NewProduction()
customLogger := logging.NewZapLogger(logger)

// 在锁操作中使用
lock := redissuo.NewSuo(rdb, "logged-lock", time.Minute).
    WithLogger(customLogger)
```

## 测试

项目包含使用内存 Redis (miniredis) 的完整测试：

```bash
# 运行所有测试
go test ./...

# 详细输出运行测试
go test -v ./...

# 运行竞态检测
go test -race ./...

# 生成覆盖率报告
go test -cover ./...
```

## 架构

```
redis-go-suo/
├── redissuo/           # 核心锁实现
│   ├── redis_suo.go    # 主要锁操作
│   └── redis_suo_test.go
├── redissuorun/        # 高端接口
│   ├── redis_suo_run.go # 生命周期管理
│   └── redis_suo_run_test.go
└── internal/           # 内部工具
    ├── logging/        # 可插拔日志接口
    └── utils/          # UUID 生成工具
```

## 使用示例

### 锁延期

**延长现有锁持续时间:**
```go
extendedSession, err := lock.AcquireAgainExtendLock(ctx, session)
if err != nil {
    log.Printf("锁延期失败: %v", err)
}
```

**自定义会话 UUID:**
```go
sessionUUID := "my-custom-session-123"
session, err := lock.AcquireLockWithSession(ctx, sessionUUID)
```

### 日志配置

**测试时静默日志:**
```go
nopLogger := logging.NewNopLogger()
lock := redissuo.NewSuo(rdb, "test-lock", time.Minute).
    WithLogger(nopLogger)
```

**自定义 Zap 日志记录器:**
```go
logger, _ := zap.NewDevelopment()
customLogger := logging.NewZapLogger(logger)
err := redissuorun.SuoLockXqt(ctx, lock, businessFunc, retryInterval, customLogger)
```

### 错误处理

**处理锁获取超时:**
```go
ctx, can := context.WithTimeout(context.Background(), time.Second*10)
defer can()

session, err := lock.Acquire(ctx)
if err != nil {
    if errors.Is(err, context.DeadlineExceeded) {
        fmt.Println("锁获取超时")
        return
    }
    // 处理其他错误
}
```

**锁释放处理:**
```go
defer func() {
    if session != nil {
        if success, err := lock.Release(context.Background(), session); err != nil {
            log.Printf("警告: 锁释放失败: %v", err)
        } else if !success {
            log.Println("警告: 锁已被释放或过期")
        }
    }
}()
```

## 最佳实践

1. **始终释放锁**: 使用 defer 或保证清理机制
2. **处理锁失败**: 检查 nil 会话并适当处理  
3. **设置合适的 TTL**: 在安全和性能之间平衡
4. **使用重复逻辑**: 为高竞争场景实现退避策略
5. **监控锁使用**: 为生产系统实现日志和指标

<!-- TEMPLATE (ZH) BEGIN: STANDARD PROJECT FOOTER -->
<!-- VERSION 2025-09-06 04:53:24.895249 +0000 UTC -->

## 📄 许可证类型

MIT 许可证。详见 [LICENSE](LICENSE)。

---

## 🤝 项目贡献

非常欢迎贡献代码！报告 BUG、建议功能、贡献代码：

- 🐛 **发现问题？** 在 GitHub 上提交问题并附上重现步骤
- 💡 **功能建议？** 创建 issue 讨论您的想法
- 📖 **文档疑惑？** 报告问题，帮助我们改进文档
- 🚀 **需要功能？** 分享使用场景，帮助理解需求
- ⚡ **性能瓶颈？** 报告慢操作，帮助我们优化性能
- 🔧 **配置困扰？** 询问复杂设置的相关问题
- 📢 **关注进展？** 关注仓库以获取新版本和功能
- 🌟 **成功案例？** 分享这个包如何改善工作流程
- 💬 **反馈意见？** 欢迎提出建议和意见

---

## 🔧 代码贡献

新代码贡献，请遵循此流程：

1. **Fork**：在 GitHub 上 Fork 仓库（使用网页界面）
2. **克隆**：克隆 Fork 的项目（`git clone https://github.com/yourname/repo-name.git`）
3. **导航**：进入克隆的项目（`cd repo-name`）
4. **分支**：创建功能分支（`git checkout -b feature/xxx`）
5. **编码**：实现您的更改并编写全面的测试
6. **测试**：（Golang 项目）确保测试通过（`go test ./...`）并遵循 Go 代码风格约定
7. **文档**：为面向用户的更改更新文档，并使用有意义的提交消息
8. **暂存**：暂存更改（`git add .`）
9. **提交**：提交更改（`git commit -m "Add feature xxx"`）确保向后兼容的代码
10. **推送**：推送到分支（`git push origin feature/xxx`）
11. **PR**：在 GitHub 上打开 Pull Request（在 GitHub 网页上）并提供详细描述

请确保测试通过并包含相关的文档更新。

---

## 🌟 项目支持

非常欢迎通过提交 Pull Request 和报告问题来为此项目做出贡献。

**项目支持：**

- ⭐ **给予星标**如果项目对您有帮助
- 🤝 **分享项目**给团队成员和（golang）编程朋友
- 📝 **撰写博客**关于开发工具和工作流程 - 我们提供写作支持
- 🌟 **加入生态** - 致力于支持开源和（golang）开发场景

**使用这个包编程快乐！** 🎉

<!-- TEMPLATE (ZH) END: STANDARD PROJECT FOOTER -->

---

## GitHub 标星点赞

[![Stargazers](https://starchart.cc/go-xlan/redis-go-suo.svg?variant=adaptive)](https://starchart.cc/go-xlan/redis-go-suo)