package redissuo

import (
	"context"
	"reflect"
	"strconv"
	"time"

	"github.com/go-xlan/redis-go-suo/internal/utils"
	"github.com/pkg/errors"
	"github.com/redis/go-redis/v9"
	"github.com/yyle88/erero"
	"github.com/yyle88/must"
	"github.com/yyle88/zaplog"
	"go.uber.org/zap"
)

type Suo struct {
	redisClient redis.UniversalClient
	key         string
	ttl         time.Duration
}

func NewSuo(rds redis.UniversalClient, key string, ttl time.Duration) *Suo {
	return &Suo{
		redisClient: must.Nice(rds),
		key:         must.Nice(key),
		ttl:         must.Nice(ttl),
	}
}

const (
	commandAcquire = `if redis.call("GET", KEYS[1]) == ARGV[1] then
    redis.call("SET", KEYS[1], ARGV[1], "PX", ARGV[2])
    return "OK"
else
    return redis.call("SET", KEYS[1], ARGV[1], "NX", "PX", ARGV[2])
end`
)

func (o *Suo) acquire(ctx context.Context, value string) (bool, error) {
	must.OK(value)

	LOG := zaplog.ZAP.NewLog("action", "申请锁", zap.String("k", o.key), zap.String("v", value))

	mst := o.ttl.Milliseconds() // 设置过期时间，在 redis 里 px 含义是 milliseconds to expire，时间取毫秒数，因为 PX 接受的是毫秒数

	resp, err := o.redisClient.Eval(ctx, commandAcquire, []string{o.key}, []string{value, strconv.FormatInt(mst, 10)}).Result()
	if errors.Is(err, redis.Nil) {
		LOG.Debug("锁已经被占用-申请不到-请等待释放")
		return false, nil
	} else if err != nil {
		LOG.Error("请求报错", zap.Error(err))
		return false, erero.Wro(err)
	} else if resp == nil {
		LOG.Error("其它错误")
		return false, nil
	}

	msg, ok := resp.(string)
	if !ok {
		LOG.Error("回复非预期类型", zap.Any("resp", resp), zap.String("resp_type", reflect.TypeOf(resp).String()))
		return false, nil
	}
	if msg != "OK" {
		LOG.Error("消息内容不匹配", zap.String("msg", msg))
		return false, nil
	}
	LOG.Debug("锁已成功申请")
	return true, nil
}

const (
	// 通过官方文档，在 Lua 脚本里判定 redis.call("GET", KEYS[1]) 返回是否为空值，该直接判断结果 true/false，直接不是使用 nil/null 判定不存在
	// redis.call("DEL", KEYS[1]) 只会返回 0 或 1，不会有其他返回值
	commandRelease = `local ch = redis.call("GET", KEYS[1])
if (ch == false) then
	return 2
elseif ch == ARGV[1] then
    return redis.call("DEL", KEYS[1])
else
    return 3
end`
)

func (o *Suo) release(ctx context.Context, value string) (bool, error) {
	must.OK(value)

	LOG := zaplog.ZAP.NewLog("action", "释放锁", zap.String("k", o.key), zap.String("v", value))

	resp, err := o.redisClient.Eval(ctx, commandRelease, []string{o.key}, []string{value}).Result()
	if err != nil {
		LOG.Error("请求报错", zap.Error(err))
		return false, erero.Wro(err)
	} else if resp == nil {
		LOG.Error("其它错误")
		return false, nil
	}

	num, ok := resp.(int64)
	if !ok {
		LOG.Debug("回复非预期类型", zap.Any("resp", resp), zap.String("resp_type", reflect.TypeOf(resp).String()))
		return false, nil
	}
	switch num {
	case 0: //说明GET时得到而DEL时删不掉，说明在这个极小的间隔里锁被自动释放，这种场景很不常见
		LOG.Debug("锁已自动释放")
		return true, nil
	case 1: //正常删除
		LOG.Debug("锁已成功释放")
		return true, nil
	case 2: //表示这个键已经自动过期，说明锁定以后使用了过长的时间，以致于调用解锁以前锁已经被自动释放
		LOG.Debug("锁不存在-或者锁已自动释放")
		return true, nil
	case 3:
		LOG.Debug("释放出错-锁被其它线程占用")
		return false, nil
	default:
		LOG.Debug("其它错误", zap.Int64("num", num))
		return false, nil
	}
}

type Xin struct {
	key         string
	sessionUUID string    //当前锁的会话信息
	expire      time.Time //最保守的过期时间
}

func (s *Xin) SessionUUID() string {
	return s.sessionUUID
}

func (s *Xin) Expire() time.Time {
	return s.expire
}

func (o *Suo) AcquireLockWithSession(ctx context.Context, sessionUUID string) (*Xin, error) {
	//记住申请锁的起始时间
	var startTime = time.Now()
	//使用此会话信息获取锁
	if ok, err := o.acquire(ctx, sessionUUID); err != nil {
		return nil, erero.Wro(err)
	} else if !ok {
		return nil, nil
	} else {
		now := time.Now() //需要让这个值在前面，以确保时间的保守
		timeSpent := time.Since(startTime)
		remain := o.ttl - timeSpent //配置的生存期减去去申请锁消耗的时间
		expire := now.Add(remain)   //得到保守估计的锁的过期时间
		return &Xin{key: o.key, sessionUUID: sessionUUID, expire: expire}, nil
	}
}

func (o *Suo) Acquire(ctx context.Context) (*Xin, error) {
	//使用完全随机的信号量
	var sessionUUID = utils.NewUUID()
	//使用此信号量申请新锁
	return o.AcquireLockWithSession(ctx, sessionUUID)
}

func (o *Suo) Release(ctx context.Context, xin *Xin) (bool, error) {
	//需要用相同的信息去释放锁，否则就不能释放
	must.Equals(xin.key, o.key)
	//使用此会话信息释放锁
	return o.release(ctx, xin.sessionUUID)
}

func (o *Suo) AcquireAgainExtendLock(ctx context.Context, xin *Xin) (*Xin, error) {
	//需要用相同的信息再次申请，否则就不能延期
	must.Equals(xin.key, o.key)
	//使用相同的信息再去申请锁，就能把原来的锁延期，得到新的会话信息
	return o.AcquireLockWithSession(ctx, xin.sessionUUID)
}
