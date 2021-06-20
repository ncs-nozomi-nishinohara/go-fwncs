package redis

import (
	"context"
	"crypto/tls"
	"encoding/base32"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/go-redis/redis/v8"
	"github.com/gorilla/securecookie"
	gsessions "github.com/gorilla/sessions"
	"github.com/n-creativesystem/go-fwncs"
	"github.com/n-creativesystem/go-fwncs/sessions"
)

var sessionExpire = 86400 * 30

const (
	keyPrefix = "session_"
)

type redisInterface interface {
	redis.Cmdable
	Close() error
}

// type redisClient struct {
// 	client redis.Cmdable
// }

// func (r *redisClient) HGetAll(ctx context.Context, key string) (map[string]string, error) {
// 	switch client := r.client.(type) {
// 	case *redis.Client:
// 		return client.HGetAll(ctx, key).Result()
// 	case *redis.ClusterClient:
// 		return client.HGetAll(ctx, key).Result()
// 	default:
// 		panic(fmt.Sprintf("no support client type: %v", client))
// 	}
// }

// func (r *redisClient) Del(ctx context.Context, key string) error {
// 	switch client := r.client.(type) {
// 	case *redis.Client:
// 		return client.Del(ctx, key).Err()
// 	case *redis.ClusterClient:
// 		return client.Del(ctx, key).Err()
// 	default:
// 		panic(fmt.Sprintf("no support client type: %v", client))
// 	}
// }

// func (r *redisClient) Close() error {
// 	switch client := r.client.(type) {
// 	case *redis.Client:
// 		return client.Close()
// 	case *redis.ClusterClient:
// 		return client.Close()
// 	default:
// 		panic(fmt.Sprintf("no support client type: %v", client))
// 	}
// }

// func (r *redisClient) SetEX(ctx context.Context, key string, value interface{}, age time.Duration) error {
// 	switch client := r.client.(type) {
// 	case *redis.Client:
// 		return client.SetEX(ctx, key, value, age).Err()
// 	case *redis.ClusterClient:
// 		return client.SetEX(ctx, key, value, age).Err()
// 	default:
// 		panic(fmt.Sprintf("no support client type: %v", client))
// 	}

// }

type Store interface {
	sessions.Store
}

type RedisStore struct {
	username          string
	password          string
	endpoints         []string
	cluster           bool
	options           *gsessions.Options
	codecs            []securecookie.Codec
	keyPrefix         string
	dialer            func(ctx context.Context, network, addr string) (net.Conn, error)
	onConnect         func(ctx context.Context, cn *redis.Conn) error
	tlsConfig         *tls.Config
	sessionSerializer SessionSerializer
	log               fwncs.ILogger
}

type RedisOptions struct {
	Username          string
	Password          string
	Endpoints         []string
	GorillaOptions    *gsessions.Options
	KeyPairs          []byte
	KeyPrefix         string
	SessionSerializer SessionSerializer
	Dialer            func(ctx context.Context, network, addr string) (net.Conn, error)
	OnConnect         func(ctx context.Context, cn *redis.Conn) error
	TlsConfig         *tls.Config
}

type store struct {
	*RedisStore
}

func (c *store) Options(options sessions.Options) {
	c.RedisStore.options = options.ToGorillaOptions()
}

func (c *store) Logger(log fwncs.ILogger) {
	c.RedisStore.log = log
}

var _ Store = &store{}

func NewStore(opts *RedisOptions) Store {
	gorillaOptions := gsessions.Options{}
	if opts.GorillaOptions != nil {
		gorillaOptions = *opts.GorillaOptions
	}
	if gorillaOptions.MaxAge <= 0 {
		gorillaOptions.MaxAge = sessionExpire
	}
	if gorillaOptions.Path == "" {
		gorillaOptions.Path = "/"
	}
	if opts.SessionSerializer == nil {
		opts.SessionSerializer = JSONSerializer{}
	}
	if opts.KeyPrefix == "" {
		opts.KeyPrefix = keyPrefix
	}
	return &store{RedisStore: &RedisStore{
		username:          opts.Username,
		password:          opts.Password,
		endpoints:         opts.Endpoints,
		cluster:           len(opts.Endpoints) > 1,
		options:           &gorillaOptions,
		codecs:            securecookie.CodecsFromPairs(opts.KeyPairs),
		keyPrefix:         opts.KeyPrefix,
		dialer:            opts.Dialer,
		onConnect:         opts.OnConnect,
		tlsConfig:         opts.TlsConfig,
		sessionSerializer: opts.SessionSerializer,
	}}
}

func (s *RedisStore) Get(req *http.Request, name string) (*gsessions.Session, error) {
	return gsessions.GetRegistry(req).Get(s, name)
}

func (s *RedisStore) New(r *http.Request, name string) (*gsessions.Session, error) {
	session := gsessions.NewSession(s, name)
	opts := *s.options
	session.Options = &opts
	session.IsNew = true
	if c, errCookie := r.Cookie(name); errCookie == nil {
		err := securecookie.DecodeMulti(name, c.Value, &session.ID, s.codecs...)
		if err == nil {
			ok, err := s.load(session)
			session.IsNew = !(err == nil && ok)
		}
	}
	return session, nil
}

func (s *RedisStore) Save(r *http.Request, w http.ResponseWriter, session *gsessions.Session) error {
	if session.Options.MaxAge <= 0 {
		if err := s.delete(session); err != nil {
			return err
		}
		http.SetCookie(w, gsessions.NewCookie(session.Name(), "", session.Options))
	} else {
		if session.ID == "" {
			session.ID = strings.TrimRight(base32.StdEncoding.EncodeToString(securecookie.GenerateRandomKey(32)), "=")
		}
		if err := s.save(session); err != nil {
			return err
		}
		encoded, err := securecookie.EncodeMulti(session.Name(), session.ID, s.codecs...)
		if err != nil {
			return err
		}
		http.SetCookie(w, gsessions.NewCookie(session.Name(), encoded, session.Options))
	}
	return nil
}

func (s *RedisStore) loadClient() (*redis.Client, error) {
	client := redis.NewClient(&redis.Options{
		Addr:      s.endpoints[0],
		Username:  s.username,
		Password:  s.password,
		Dialer:    s.dialer,
		OnConnect: s.onConnect,
		TLSConfig: s.tlsConfig,
	})
	err := client.Ping(context.Background()).Err()
	return client, err
}

func (s *RedisStore) loadCluster() (*redis.ClusterClient, error) {
	client := redis.NewClusterClient(&redis.ClusterOptions{
		Addrs:     s.endpoints,
		Username:  s.username,
		Password:  s.password,
		Dialer:    s.dialer,
		OnConnect: s.onConnect,
		TLSConfig: s.tlsConfig,
	})
	err := client.Ping(context.Background()).Err()
	return client, err
}

func (s *RedisStore) getClient() (redisInterface, error) {
	if s.cluster {
		return s.loadCluster()
	} else {
		return s.loadClient()
	}
}

func (s *RedisStore) load(session *gsessions.Session) (bool, error) {
	client, err := s.getClient()
	if err != nil {
		return false, err
	}
	defer client.Close()
	result, err := client.Get(context.Background(), s.GetSessionName(session)).Result()
	if err != nil {
		return false, err
	}
	return true, s.sessionSerializer.Deserialize([]byte(result), session)
}

func (s *RedisStore) delete(session *gsessions.Session) error {
	client, err := s.getClient()
	if err != nil {
		return err
	}
	defer client.Close()
	return client.Del(context.Background(), s.GetSessionName(session)).Err()
}

func (s *RedisStore) save(session *gsessions.Session) error {
	client, err := s.getClient()
	if err != nil {
		return err
	}
	defer client.Close()
	buf, err := s.sessionSerializer.Serialize(session)
	if err != nil {
		return err
	}
	age := session.Options.MaxAge
	if age == 0 {
		age = sessionExpire
	}
	age = int(time.Second * time.Duration(age))
	return client.SetEX(context.Background(), s.GetSessionName(session), string(buf), time.Duration(age)).Err()
}

func (s *RedisStore) GetSessionName(session *gsessions.Session) string {
	return s.keyPrefix + session.ID
}

func GetRedisStore(s Store) (err error, rediStore *RedisStore) {
	realStore, ok := s.(*store)
	if !ok {
		err = errors.New("unable to get the redis store: Store isn't *store")
		return
	}

	rediStore = realStore.RedisStore
	return
}

type SessionSerializer interface {
	Serialize(ss *gsessions.Session) ([]byte, error)
	Deserialize(d []byte, ss *gsessions.Session) error
	Logger() fwncs.ILogger
	SetLogger(fwncs.ILogger)
}

type JSONSerializer struct {
	log fwncs.ILogger
}

var _ SessionSerializer = JSONSerializer{}

func (s JSONSerializer) Serialize(ss *gsessions.Session) ([]byte, error) {
	m := make(map[string]interface{}, len(ss.Values))
	for k, v := range ss.Values {
		ks, ok := k.(string)
		if !ok {
			err := fmt.Errorf("Non-string key value, cannot serialize session to JSON: %v", k)
			s.Logger().Error(fmt.Sprintf("redistore.JSONSerializer.serialize() Error: %v", err))
			return nil, err
		}
		m[ks] = v
	}
	return json.Marshal(m)
}

func (s JSONSerializer) Deserialize(d []byte, ss *gsessions.Session) error {
	m := make(map[string]interface{})
	err := json.Unmarshal(d, &m)
	if err != nil {
		s.Logger().Error(fmt.Sprintf("redistore.JSONSerializer.deserialize() Error: %v", err))
		return err
	}
	for k, v := range m {
		ss.Values[k] = v
	}
	return nil
}

func (s JSONSerializer) Logger() fwncs.ILogger {
	return s.log
}

func (s JSONSerializer) SetLogger(log fwncs.ILogger) {
	s.log = log
}
