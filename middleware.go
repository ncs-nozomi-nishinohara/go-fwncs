package fwncs

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"runtime"
	"time"

	"github.com/n-creativesystem/go-fwncs/constant"
)

type DefaultResponseBody struct {
	Code     int    `json:"code"`
	Status   string `json:"status"`
	Message  string `json:"message"`
	Internal error  `json:"-"`
}

func (d *DefaultResponseBody) Error() string {
	if d.Internal == nil {
		return fmt.Sprintf("code=%d, message=%v", d.Code, d.Message)
	}
	return fmt.Sprintf("code=%d, message=%v, internal=%v", d.Code, d.Message, d.Internal)
}

func NewDefaultResponseBody(code int, message string) *DefaultResponseBody {
	var status string
	// 200 ~ 226
	switch {
	case http.StatusOK <= code && code <= http.StatusIMUsed:
		status = "success"
	case http.StatusMultipleChoices <= code && code <= http.StatusPermanentRedirect:
		status = "redirect"
	default:
		status = "error"
	}
	if http.StatusOK <= code && code <= http.StatusIMUsed {
		status = "success"
	} else {
		status = "error"
	}
	return &DefaultResponseBody{
		Status:  status,
		Code:    code,
		Message: message,
	}
}

func Recovery() HandlerFunc {
	return func(c Context) {
		defer func() {
			if rcv := recover(); rcv != nil {
				for depth := 0; ; depth++ {
					_, file, line, ok := runtime.Caller(depth)
					if !ok {
						break
					}
					c.Logger().Error(fmt.Sprintf("%d: %v:%d", depth, file, line))
				}
				c.AbortWithStatusAndMessage(http.StatusInternalServerError, NewDefaultResponseBody(http.StatusInternalServerError, fmt.Sprintf("%v", rcv)))
			}
		}()
		c.Next()
	}
}

var requestIDKey fwNscContext

func FromRequestID(ctx context.Context) string {
	rid, ok := ctx.Value(requestIDKey).(string)
	if !ok {
		rid = requestIDGenerator()
	}
	return rid
}

type RequestIDConfig struct {
	Generator func() string
}

func RequestID() HandlerFunc {
	return RequestIDWithConfig(RequestIDConfig{
		Generator: requestIDGenerator,
	})
}

func RequestIDWithConfig(config RequestIDConfig) HandlerFunc {
	if config.Generator == nil {
		config.Generator = requestIDGenerator
	}
	return func(c Context) {
		rid := c.GetRequestID()
		if rid == "" {
			rid = config.Generator()
		}
		ctx := c.GetContext()
		ctx = context.WithValue(ctx, requestIDKey, rid)
		c.SetContext(ctx)
		c.SetHeader(constant.HeaderXRequestID, rid)
		idCookie := new(http.Cookie)
		idCookie.Path = "/"
		idCookie.Name = constant.HeaderXRequestID
		idCookie.Value = rid
		idCookie.MaxAge = 0
		c.SetCookie(idCookie)
		c.Next()
	}
}

type Random struct{}

var random = newRandom()

func newRandom() *Random {
	rand.Seed(time.Now().UnixNano())
	return new(Random)
}

func (*Random) String(length uint8) string {
	b := make([]byte, length)
	for i := range b {
		b[i] = Alphanumeric[rand.Int63()%int64(len(Alphanumeric))]
	}
	return string(b)
}

func requestIDGenerator() string {
	return random.String(32)
}
