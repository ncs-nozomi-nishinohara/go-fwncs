package redis_test

import (
	"testing"

	"github.com/n-creativesystem/go-fwncs/sessions"
	"github.com/n-creativesystem/go-fwncs/sessions/redis"
	"github.com/n-creativesystem/go-fwncs/sessions/tester"
)

var redisTestServer = []string{"localhost:6379"}

var newRedisStore = func(_ *testing.T) sessions.Store {
	opts := redis.RedisOptions{
		Endpoints: redisTestServer,
		KeyPairs:  []byte("secure"),
	}
	store := redis.NewStore(&opts)
	return store
}

func TestRedis_SessionGetSet(t *testing.T) {
	tester.GetSet(t, newRedisStore)
}

func TestRedis_SessionDeleteKey(t *testing.T) {
	tester.DeleteKey(t, newRedisStore)
}

func TestRedis_SessionFlashes(t *testing.T) {
	tester.Flashes(t, newRedisStore)
}

func TestRedis_SessionClear(t *testing.T) {
	tester.Clear(t, newRedisStore)
}

func TestRedis_SessionOptions(t *testing.T) {
	tester.Options(t, newRedisStore)
}

func TestRedis_SessionMany(t *testing.T) {
	tester.Many(t, newRedisStore)
}

func TestGetRedisStore(t *testing.T) {
	t.Run("unmatched type", func(t *testing.T) {
		type store struct{ redis.Store }
		err, rediStore := redis.GetRedisStore(store{})
		if err == nil || rediStore != nil {
			t.Fail()
		}
	})
}
