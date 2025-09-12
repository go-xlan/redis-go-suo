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
		fmt.Println("Lock free failed - might be freed via timeout in different session")
	}
}
