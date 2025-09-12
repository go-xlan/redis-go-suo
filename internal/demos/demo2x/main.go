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
