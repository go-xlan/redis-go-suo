package redissuorun_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/go-xlan/redis-go-suo/internal/utils"
	"github.com/go-xlan/redis-go-suo/redissuo"
	"github.com/go-xlan/redis-go-suo/redissuorun"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	"github.com/yyle88/must"
)

var caseRds redis.UniversalClient

func TestMain(m *testing.M) {
	redisUc := redis.NewUniversalClient(&redis.UniversalOptions{
		Addrs:        []string{"127.0.0.1:6379"},
		PoolSize:     10,
		MinIdleConns: 10,
	})
	must.Done(redisUc.Ping(context.Background()).Err())

	caseRds = redisUc

	m.Run()
}

func TestSuoLockRun(t *testing.T) {
	suo := redissuo.NewSuo(caseRds, utils.NewUUID(), 50*time.Millisecond)
	var since = time.Now()
	var wg sync.WaitGroup
	for idx := 0; idx < 10; idx++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			run := func(ctx context.Context) error {
				require.NoError(t, ctx.Err())
				t.Log("run->", time.Since(since))
				time.Sleep(time.Millisecond * 20)
				require.NoError(t, ctx.Err())
				t.Log("run<-", time.Since(since))
				return nil
			}

			require.NoError(t, redissuorun.SuoLockRun(context.Background(), suo, run, time.Millisecond*20))
		}()
	}
	wg.Wait()
}
