package redissuorun

import (
	"context"
	"time"

	"github.com/go-xlan/redissuo"
	"github.com/go-xlan/redissuo/internal/utils"
	"github.com/yyle88/erero"
	"github.com/yyle88/zaplog"
	"go.uber.org/zap"
)

func SuoLockRun(ctx context.Context, suo *redissuo.Suo, run func(ctx context.Context) error, sleep time.Duration) error {
	var uus = utils.NewUUID()
	var xin *redissuo.Xin
	acquireFunc := func(ctx context.Context) (bool, error) {
		res, err := suo.AcquireLockWithSession(ctx, uus)
		if err != nil {
			return false, erero.Wro(err)
		}
		xin = res
		return xin != nil, nil //当最终锁定成功时就会返回
	}
	if err := unremittingDoAcquire(ctx, acquireFunc, sleep); err != nil {
		return erero.Wro(err) // 说明是 context 出现错误
	}
	defer func() { //只要锁成功，在执行完以后无论是否执行逻辑出错，都要释放锁
		unremittingDoRelease(func() (bool, error) { return releaseFunc(ctx, suo, xin, sleep) }, sleep)
	}()

	duration := time.Until(xin.Exp()) //既然设置了的锁的存活时间，就得在这段时间内把事情干完，因此也希望把存活时间设置长些，要不然无法覆盖逻辑执行的全流程
	newCtx, can := context.WithTimeout(ctx, duration)
	defer can()
	if err := safeRun(newCtx, run); err != nil {
		return erero.Wro(err)
	}
	return nil
}

func unremittingDoAcquire(ctx context.Context, run func(ctx context.Context) (bool, error), duration time.Duration) error {
	for {
		if err := ctx.Err(); err != nil {
			return erero.Wro(err) //就是你 context 出错的时候，里面的 redis 或者 database 操作都会永不成功，因此就是需要判定 context 的情况
		}
		if success, e := run(ctx); e != nil {
			zaplog.LOGGER.LOG.Debug("wrong", zap.Error(e))
			time.Sleep(duration)
			continue
		} else if !success {
			time.Sleep(duration)
			continue
		}
		return nil
	}
}

func releaseFunc(ctx context.Context, suo *redissuo.Suo, xin *redissuo.Xin, sleep time.Duration) (bool, error) {
	ctx, can := safeCtx(ctx, max(sleep, time.Second*10))
	defer can()

	success, err := suo.Release(ctx, xin)
	if err != nil {
		return false, erero.Wro(err)
	}
	return success, nil //当最终解锁成功时就会返回
}

func unremittingDoRelease(run func() (bool, error), duration time.Duration) {
	for {
		if success, err := run(); err != nil {
			zaplog.LOGGER.LOG.Debug("wrong", zap.Error(err))
			time.Sleep(duration)
			continue
		} else if !success {
			time.Sleep(duration)
			continue
		}
		return
	}
}

func safeCtx(ctx context.Context, duration time.Duration) (context.Context, context.CancelFunc) {
	if ctx.Err() != nil {
		return context.WithTimeout(context.Background(), duration)
	}
	return context.WithCancel(ctx)
}

func safeRun(ctx context.Context, run func(ctx context.Context) error) (err error) {
	defer func() {
		if rec := recover(); rec != nil {
			switch erx := rec.(type) {
			case error:
				err = erx
			default:
				err = erero.Errorf("错误(已从崩溃中恢复):%v", rec)
			}
		}
	}()
	return run(ctx)
}
