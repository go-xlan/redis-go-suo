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
