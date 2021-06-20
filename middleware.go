package fwncs

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"runtime"
	"time"

	"github.com/n-creativesystem/go-fwncs/constant"
	"github.com/n-creativesystem/go-fwncs/render"
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

func notFound() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		render.WriteJson(w, NewDefaultResponseBody(http.StatusNotFound, http.StatusText(http.StatusNotFound)))
	})
}

var methodNotAllowed = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
	w.WriteHeader(http.StatusMethodNotAllowed)
	render.WriteJson(w, NewDefaultResponseBody(http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed)))
})

var defaultGlobalOPTIONS = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("Access-Control-Request-Method") != "" {
		// Set CORS headers
		header := w.Header()
		header.Set("Access-Control-Allow-Methods", header.Get("Allow"))
		header.Set("Access-Control-Allow-Origin", "*")
	}
	// Adjust status code to 204
	w.WriteHeader(http.StatusNoContent)
})

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
		rid := c.Header().Get(constant.HeaderXRequestID)
		if rid == "" {
			rid = config.Generator()
		}
		ctx := c.GetContext()
		ctx = context.WithValue(ctx, requestIDKey, rid)
		c.SetContext(ctx)
		c.SetHeader(constant.HeaderXRequestID, rid)
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
