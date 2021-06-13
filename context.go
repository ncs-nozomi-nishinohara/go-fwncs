package fwncs

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/julienschmidt/httprouter"
)

type Context interface {
	Writer() http.ResponseWriter
	Request() *http.Request
	Header() http.Header
	Logger() ILogger
	JSON(status int, v interface{}) error
	Params() httprouter.Params
	ClientIP() string
	Start()
	End()
	Abort()
	AbortWithStatus(status int)
	AbortWithStatusAndMessage(status int, v interface{}) error
	Next()
	SetStatus(status int)
	GetStatus() int
	ResponseSize() int64
	Set(key string, value interface{})
	Get(key string) (value interface{}, exists bool)
	JSONBody(v interface{}) error
}

type _context struct {
	mu          sync.Mutex
	w           *responseWriter
	req         *http.Request
	res         *Response
	params      httprouter.Params
	logger      ILogger
	requestInfo *requestInfo
	abort       bool
	handler     []HandlerFunc
	index       int
	mp          map[string]interface{}
}

var _ Context = &_context{}

func newContext(w http.ResponseWriter, r *http.Request) *_context {
	rw, res := wrapResponseWriter(w)
	return &_context{
		w:     rw,
		req:   r,
		res:   res,
		abort: false,
		index: -1,
		mu:    sync.Mutex{},
	}
}

func (c *_context) Writer() http.ResponseWriter {
	return c.w
}

func (c *_context) Request() *http.Request {
	return c.req
}

func (c *_context) Header() http.Header {
	return c.req.Header
}

func (c *_context) Logger() ILogger {
	return c.logger
}

func (c *_context) JSON(status int, v interface{}) error {
	c.w.Header().Set("X-Content-Type-Options", "nosniff")
	c.w.Header().Set(MIMEJSON.Get())
	c.SetStatus(status)
	return json.NewEncoder(c.w).Encode(v)
}

func (c *_context) Params() httprouter.Params {
	return c.params
}

func (c *_context) ClientIP() string {
	clientIP := c.Header().Get("X-Forwarded-For")
	clientIP = strings.TrimSpace(strings.Split(clientIP, ",")[0])
	if clientIP == "" {
		clientIP = strings.TrimSpace(c.Header().Get("X-Real-IP"))
	}
	if clientIP != "" {
		return clientIP
	}

	if ip, _, err := net.SplitHostPort(strings.TrimSpace(c.req.RemoteAddr)); err == nil {
		return ip
	}
	return ""
}

type requestInfo struct {
	StartTime     time.Time
	Path          string
	RawQuery      string
	Method        string
	RemoteAddress string
	EndTime       time.Time
}

func (i *requestInfo) GetLatency() time.Duration {
	return i.EndTime.Sub(i.StartTime)
}

func (i *requestInfo) GetPath() string {
	if i.RawQuery != "" {
		return i.Path + "?" + i.RawQuery
	}
	return i.Path
}

func (c *_context) Start() {
	c.requestInfo = &requestInfo{
		StartTime:     time.Now(),
		Path:          c.req.URL.Path,
		RawQuery:      c.req.URL.RawQuery,
		Method:        c.req.Method,
		RemoteAddress: c.ClientIP(),
	}
}

func (c *_context) End() {
	info := c.requestInfo
	info.EndTime = time.Now()
	// 時間 ステータスコード レイテンシー IPアドレス HTTPメソッド パス
	timeFormat := "2006/01/02 15:04:05"
	s := fmt.Sprintf(
		"code:%d\tstart:%v\tend:%v\tlatency:%v\tip:%s\tmethod:%s\tpath:%s\n",
		c.w.resp.StatusCode,
		info.StartTime.Format(timeFormat),
		info.EndTime.Format(timeFormat),
		info.GetLatency(),
		info.RemoteAddress,
		info.Method,
		info.GetPath(),
	)
	c.logger.Info(s)
}

func (c *_context) Abort() {
	c.abort = true
}

func (c *_context) AbortWithStatus(status int) {
	c.abort = true
	c.w.WriteHeader(status)
}

func (c *_context) Next() {
	c.index++
	for c.index < len(c.handler) {
		if !c.abort {
			c.handler[c.index](c)
			c.index++
		} else {
			break
		}
	}
}

func (c *_context) SetStatus(status int) {
	c.w.WriteHeader(status)
}

func (c *_context) GetStatus() int {
	return c.w.resp.StatusCode
}

func (c *_context) ResponseSize() int64 {
	return c.w.size
}

func (c *_context) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.mp[key] = value
}
func (c *_context) Get(key string) (value interface{}, exists bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	value, exists = c.mp[key]
	return
}

func (c *_context) JSONBody(v interface{}) error {
	return json.NewDecoder(c.req.Body).Decode(v)
}

func (c *_context) AbortWithStatusAndMessage(status int, v interface{}) error {
	c.Abort()
	return c.JSON(status, v)
}
