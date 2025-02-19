package redissuorun

import (
	"context"
	"time"

	"github.com/go-xlan/redis-go-suo/internal/utils"
	"github.com/go-xlan/redis-go-suo/redissuo"
	"github.com/yyle88/erero"
	"github.com/yyle88/must"
	"github.com/yyle88/zaplog"
	"go.uber.org/zap"
)

func SuoLockRun(ctx context.Context, suo *redissuo.Suo, run func(ctx context.Context) error, sleep time.Duration) error {
	var sessionUUID = utils.NewUUID()

	var message = &outputMessage{}
	if err := retryingAcquire(ctx, func(ctx context.Context) (bool, error) {
		return acquireOnce(ctx, suo, sessionUUID, message)
	}, sleep); err != nil {
		return erero.Wro(err) // 说明是 context 出现错误
	}

	must.Nice(message.xin) //前面要么出错返回，要么必然成功，因此走到这里时，就表示锁已经申请成功

	defer func() { //只要锁成功，在执行完以后无论是否执行逻辑出错，都要释放锁
		retryingRelease(func() (bool, error) {
			return releaseOnce(ctx, suo, message.xin, sleep)
		}, sleep)
	}()

	//在锁内执行业务逻辑。既然设置了的锁的存活时间，就得在这段时间内把事情干完，因此也希望把存活时间设置长些，要不然无法覆盖逻辑执行的全流程
	if err := execRun(ctx, run, time.Until(message.xin.Expire())); err != nil {
		return erero.Wro(err)
	}
	return nil
}

type outputMessage struct {
	xin *redissuo.Xin
}

func acquireOnce(ctx context.Context, suo *redissuo.Suo, sessionUUID string, output *outputMessage) (bool, error) {
	xin, err := suo.AcquireLockWithSession(ctx, sessionUUID)
	if err != nil {
		return false, erero.Wro(err)
	}
	if xin != nil {
		output.xin = xin
		return true, nil //当最终锁定成功时就会返回
	}
	return false, nil
}

func retryingAcquire(ctx context.Context, run func(ctx context.Context) (bool, error), duration time.Duration) error {
	for {
		if err := ctx.Err(); err != nil {
			return erero.Wro(err) //就是你 context 出错的时候，里面的 redis 或者 database 操作都会永不成功，因此就是需要判定 context 的情况
		}
		success, err := run(ctx)
		if err != nil {
			zaplog.LOGGER.LOG.Debug("wrong", zap.Error(err))
			time.Sleep(duration)
			continue
		}
		if success {
			return nil
		}
		time.Sleep(duration)
		continue
	}
}

func releaseOnce(ctx context.Context, suo *redissuo.Suo, xin *redissuo.Xin, sleep time.Duration) (bool, error) {
	ctx, can := safeCtx(ctx, max(sleep, time.Second*10))
	defer can()

	success, err := suo.Release(ctx, xin)
	if err != nil {
		return false, erero.Wro(err)
	}
	return success, nil //当最终解锁成功时就会返回
}

func retryingRelease(run func() (bool, error), duration time.Duration) {
	for {
		success, err := run()
		if err != nil {
			zaplog.LOGGER.LOG.Debug("wrong", zap.Error(err))
			time.Sleep(duration)
			continue
		}
		if success {
			return
		}
		time.Sleep(duration)
		continue
	}
}

func safeCtx(ctx context.Context, duration time.Duration) (context.Context, context.CancelFunc) {
	if ctx.Err() != nil {
		return context.WithTimeout(context.Background(), duration)
	}
	return context.WithCancel(ctx)
}

func execRun(ctx context.Context, run func(ctx context.Context) error, duration time.Duration) (err error) {
	ctx, can := context.WithTimeout(ctx, duration)
	defer can()

	return safeRun(ctx, run)
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
