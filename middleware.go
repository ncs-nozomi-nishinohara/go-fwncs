package fwncs

import (
	"fmt"
	"net/http"
	"strconv"
)

type DefaultResponseBody struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

func NewDefaultResponseBody(status int, message string) *DefaultResponseBody {
	return &DefaultResponseBody{
		Status:  strconv.Itoa(status),
		Message: message,
	}
}

func NotFound() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		c := newContext(rw, req)
		c.JSON(http.StatusNotFound, NewDefaultResponseBody(http.StatusNotFound, http.StatusText(http.StatusNotFound)))
	})
}

func MethodNotAllowed() http.Handler {
	return http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		c := newContext(rw, req)
		c.JSON(http.StatusMethodNotAllowed, NewDefaultResponseBody(http.StatusMethodNotAllowed, http.StatusText(http.StatusMethodNotAllowed)))
	})
}

func Logger() HandlerFunc {
	return func(c Context) {
		c.Start()
		c.Next()
		c.End()
	}
}

func Recovery() HandlerFunc {
	return func(c Context) {
		defer func() {
			if rcv := recover(); rcv != nil {
				c.Logger().ChangeFormatType(FormatStandard).Skip(2).Error(rcv)
				c.AbortWithStatusAndMessage(http.StatusInternalServerError, NewDefaultResponseBody(http.StatusInternalServerError, fmt.Sprintf("%v", rcv)))
			}
		}()
		c.Next()
	}
}
