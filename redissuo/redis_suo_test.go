package redissuo_test

import (
	"context"
	"testing"
	"time"

	"github.com/go-xlan/redis-go-suo/internal/utils"
	"github.com/go-xlan/redis-go-suo/redissuo"
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

func TestSuoAcquire(t *testing.T) {
	ctx := context.Background()

	suo := redissuo.NewSuo(caseRds, utils.NewUUID(), 200*time.Millisecond)
	xin, err := suo.Acquire(ctx)
	require.NoError(t, err)
	require.NotNil(t, xin)

	t.Log(time.Until(xin.Exp()))

	success, err := suo.Release(ctx, xin)
	require.NoError(t, err)
	require.True(t, success)
}

func TestSuoAcquireTwice(t *testing.T) {
	ctx := context.Background()
	suo := redissuo.NewSuo(caseRds, utils.NewUUID(), 5*time.Second) //当使用相同的 key 时就是有冲突的

	for i := 0; i < 2; i++ {
		xin, err := suo.Acquire(ctx)
		require.NoError(t, err)
		require.NotNil(t, xin)

		{
			non, err := suo.Acquire(ctx)
			require.NoError(t, err)
			require.Nil(t, non)
		}

		success, err := suo.Release(ctx, xin)
		require.Nil(t, err)
		require.True(t, success)
	}
}

func TestSuoAcquireTwo(t *testing.T) {
	ctx := context.Background()

	suo1 := redissuo.NewSuo(caseRds, utils.NewUUID(), 5*time.Second)
	xin1, err := suo1.Acquire(ctx)
	require.NoError(t, err)
	require.NotNil(t, xin1)

	suo2 := redissuo.NewSuo(caseRds, utils.NewUUID(), 5*time.Second) //当使用不同的 key 时是没有冲突的
	xin2, err := suo2.Acquire(ctx)
	require.NoError(t, err)
	require.NotNil(t, xin2)

	{
		success, err := suo1.Release(ctx, xin1)
		require.Nil(t, err)
		require.True(t, success)
	}

	{
		success, err := suo2.Release(ctx, xin2)
		require.Nil(t, err)
		require.True(t, success)
	}
}

func TestSuoReleaseTimeout(t *testing.T) {
	ctx := context.Background()

	duration := 100 * time.Millisecond

	suo := redissuo.NewSuo(caseRds, utils.NewUUID(), duration)
	xin, err := suo.Acquire(ctx)
	require.NoError(t, err)
	require.NotNil(t, xin)

	time.Sleep(duration)

	success, err := suo.Release(ctx, xin)
	require.Nil(t, err)
	require.True(t, success)
}
