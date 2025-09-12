[![GitHub Workflow Status (branch)](https://img.shields.io/github/actions/workflow/status/go-xlan/redis-go-suo/release.yml?branch=main&label=BUILD)](https://github.com/go-xlan/redis-go-suo/actions/workflows/release.yml?query=branch%3Amain)
[![GoDoc](https://pkg.go.dev/badge/github.com/go-xlan/redis-go-suo)](https://pkg.go.dev/github.com/go-xlan/redis-go-suo)
[![Coverage Status](https://img.shields.io/coveralls/github/go-xlan/redis-go-suo/main.svg)](https://coveralls.io/github/go-xlan/redis-go-suo?branch=main)
[![Supported Go Versions](https://img.shields.io/badge/Go-1.22--1.25-lightgrey.svg)](https://go.dev/)
[![GitHub Release](https://img.shields.io/github/release/go-xlan/redis-go-suo.svg)](https://github.com/go-xlan/redis-go-suo/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/go-xlan/redis-go-suo)](https://goreportcard.com/report/github.com/go-xlan/redis-go-suo)

# redis-go-suo

Redis distributed lock implementation with Lua scripting to enable atomic operations.

---

<!-- TEMPLATE (EN) BEGIN: LANGUAGE NAVIGATION -->
## CHINESE README

[中文说明](README.zh.md)
<!-- TEMPLATE (EN) END: LANGUAGE NAVIGATION -->

## Main Features

🔐 **Atomic Lock Operations**: Lua script-based lock acquisition and release to prevent race conditions  
⚡ **Smart Session Management**: UUID-based session tracking with ownership validation  
🔄 **Auto Repeat Mechanism**: Built-in repeat logic with progressive backoff in high-contention scenarios  
🛡️ **Lifecycle Management**: Guaranteed lock cleanup with panic restoration and timeout handling  
📊 **Flexible Logging**: Pluggable logging interface with custom implementation support  
🎯 **Two-Part Design**: Core lock operations (`redissuo`) with high-grade enclosure (`redissuorun`)

## Installation

```bash
go get github.com/go-xlan/redis-go-suo
```

## Quick Start

### Basic Usage

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
	mrd := rese.P1(miniredis.Run())
	defer mrd.Close()

	// Setup Redis connection
	rdb := redis.NewClient(&redis.Options{
		Addr: mrd.Addr(),
	})
	defer rese.F0(rdb.Close)

	// Init shared lock
	lock := redissuo.NewSuo(rdb, "demo-lock", time.Minute*5)

	// Get lock
	ctx := context.Background()
	session, err := lock.Acquire(ctx)
	if err != nil {
		panic(err)
	}
	if session == nil {
		fmt.Println("Lock taken - used in different app")
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
		fmt.Println("Lock freed!")
	} else {
		fmt.Println("Lock release failed - might be freed via timeout in different session")
	}
}
```

⬆️ **Source:** [Source](internal/demos/demo1x/main.go)

### High-Grade Enclosure Usage

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
	mrd := rese.P1(miniredis.Run())
	defer mrd.Close()

	// Setup Redis connection
	rdb := redis.NewClient(&redis.Options{
		Addr: mrd.Addr(),
	})
	defer rese.F0(rdb.Close)

	// Init shared lock
	lock := redissuo.NewSuo(rdb, "app-lock", time.Minute*2)

	fmt.Println("Beginning top-grade lock action...")

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

⬆️ **Source:** [Source](internal/demos/demo2x/main.go)

## Core Components

### redissuo Package

The foundation package providing core distributed lock operations:

- **`Suo`**: Main lock structure with Redis client, name, and TTL configuration
- **`Xin`**: Lock session representation with UUID and expiration tracking
- **Atomic Operations**: Lua script-based acquire/release to maintain data coherence

### redissuorun Package

High-grade enclosure providing lifecycle management:

- **`SuoLockRun`**: Execute function within lock boundaries with auto repeat
- **`SuoLockXqt`**: Extended version with custom logging support
- **Panic Restoration**: Automatic panic handling and lock cleanup
- **Context Management**: Timeout and cancellation support

## Advanced Features

### Lock Extension

```go
// Extend existing lock session
extendedSession, err := lock.AcquireAgainExtendLock(ctx, session)
if err != nil {
    // Handle extension failure
}
// Use extendedSession to enable continued operations
```

### Custom Logging

```go
// Create custom logger
customLogger := logging.NewZapLogger(yourZapLogger)

// Use with custom logger
err := redissuorun.SuoLockXqt(ctx, lock, businessLogic, retryInterval, customLogger)
```

### Session Management

```go
// Acquire with specific session UUID
sessionUUID := "your-custom-session-id"
session, err := lock.AcquireLockWithSession(ctx, sessionUUID)

// Access session information
fmt.Printf("Session UUID: %s\n", session.SessionUUID())
fmt.Printf("Expires at: %s\n", session.Expire())
```

## Configuration Examples

### Redis Node Setup

```go
rdb := redis.NewClusterClient(&redis.ClusterOptions{
    Addrs: []string{"localhost:7000", "localhost:7001", "localhost:7002"},
})

lock := redissuo.NewSuo(rdb, "cluster-lock", time.Minute*10)
```

### Custom Logging Configuration

```go
import "go.uber.org/zap"

// Create custom zap logger
logger, _ := zap.NewProduction()
customLogger := logging.NewZapLogger(logger)

// Use with lock operations
lock := redissuo.NewSuo(rdb, "logged-lock", time.Minute).
    WithLogger(customLogger)
```

## Testing

The project includes comprehensive tests using in-RAM Redis (miniredis):

```bash
# Run tests
go test ./...

# Run with verbose output
go test -v ./...

# Run with race detection
go test -race ./...

# Generate coverage report
go test -coverage ./...
```

## Architecture

```
redis-go-suo/
├── redissuo/           # Core lock implementation
│   ├── redis_suo.go    # Main lock operations
│   └── redis_suo_test.go
├── redissuorun/        # High-grade enclosure
│   ├── redis_suo_run.go # Lifecycle management
│   └── redis_suo_run_test.go
└── internal/           # Private utilities
    ├── logging/        # Pluggable logging interface
    └── utils/          # UUID generation utilities
```

## Examples

### Lock Extension

**Extend existing lock duration:**
```go
extendedSession, err := lock.AcquireAgainExtendLock(ctx, session)
if err != nil {
    log.Printf("Failed to extend lock: %v", err)
}
```

**Custom session UUID:**
```go
sessionUUID := "my-custom-session-123"
session, err := lock.AcquireLockWithSession(ctx, sessionUUID)
```

### Logging Configuration

**Silent logging in tests:**
```go
nopLogger := logging.NewNopLogger()
lock := redissuo.NewSuo(rdb, "test-lock", time.Minute).
    WithLogger(nopLogger)
```

**Custom Zap logging:**
```go
logger, _ := zap.NewDevelopment()
customLogger := logging.NewZapLogger(logger)
err := redissuorun.SuoLockXqt(ctx, lock, businessFunc, retryInterval, customLogger)
```

### Exception Handling

**Handle lock acquisition timeout:**
```go
ctx, cancel := context.WithTimeout(context.Background(), time.Second*10)
defer cancel()

session, err := lock.Acquire(ctx)
if err != nil {
    if errors.Is(err, context.DeadlineExceeded) {
        fmt.Println("Lock acquisition timed out")
        return
    }
    // Handle more errors
}
```

**Lock release handling:**
```go
defer func() {
    if session != nil {
        if success, err := lock.Release(context.Background(), session); err != nil {
            log.Printf("Warning: Failed to release lock: %v", err)
        } else if !success {
            log.Println("Warning: Lock was released")
        }
    }
}()
```

## Best Practices

1. **Always Release Locks**: Use defer with guaranteed cleanup mechanisms
2. **Handle Lock Failures**: Check session state and handle as needed  
3. **Set Appropriate TTLs**: Balance between protection and performance
4. **Use Repeat Logic**: Implement backoff strategies in high-contention scenarios
5. **Watch Lock Usage**: Implement logging and metrics in production systems

<!-- TEMPLATE (EN) BEGIN: STANDARD PROJECT FOOTER -->
<!-- VERSION 2025-09-06 04:53:24.895249 +0000 UTC -->

## 📄 License

MIT License. See [LICENSE](LICENSE).

---

## 🤝 Contributing

Contributions are welcome! Report bugs, suggest features, and contribute code:

- 🐛 **Found a bug?** Open an issue on GitHub with reproduction steps
- 💡 **Have a feature idea?** Create an issue to discuss the suggestion
- 📖 **Documentation confusing?** Report it so we can improve
- 🚀 **Need new features?** Share the use cases to help us understand requirements
- ⚡ **Performance issue?** Help us optimize through reporting slow operations
- 🔧 **Configuration problem?** Ask questions about complex setups
- 📢 **Follow project progress?** Watch the repo to get new releases and features
- 🌟 **Success stories?** Share how this package improved the workflow
- 💬 **Feedback?** We welcome suggestions and comments

---

## 🔧 Development

New code contributions, follow this process:

1. **Fork**: Fork the repo on GitHub (using the webpage UI).
2. **Clone**: Clone the forked project (`git clone https://github.com/yourname/repo-name.git`).
3. **Navigate**: Navigate to the cloned project (`cd repo-name`)
4. **Branch**: Create a feature branch (`git checkout -b feature/xxx`).
5. **Code**: Implement the changes with comprehensive tests
6. **Testing**: (Golang project) Ensure tests pass (`go test ./...`) and follow Go code style conventions
7. **Documentation**: Update documentation to support client-facing changes and use significant commit messages
8. **Stage**: Stage changes (`git add .`)
9. **Commit**: Commit changes (`git commit -m "Add feature xxx"`) ensuring backward compatible code
10. **Push**: Push to the branch (`git push origin feature/xxx`).
11. **PR**: Open a pull request on GitHub (on the GitHub webpage) with detailed description.

Please ensure tests pass and include relevant documentation updates.

---

## 🌟 Support

Welcome to contribute to this project via submitting merge requests and reporting issues.

**Project Support:**

- ⭐ **Give GitHub stars** if this project helps you
- 🤝 **Share with teammates** and (golang) programming friends
- 📝 **Write tech blogs** about development tools and workflows - we provide content writing support
- 🌟 **Join the ecosystem** - committed to supporting open source and the (golang) development scene

**Have Fun Coding with this package!** 🎉

<!-- TEMPLATE (EN) END: STANDARD PROJECT FOOTER -->

---

## GitHub Stars

[![Stargazers](https://starchart.cc/go-xlan/redis-go-suo.svg?variant=adaptive)](https://starchart.cc/go-xlan/redis-go-suo)