package fwncs

import (
	"net/http"
)

type HandlerWrap struct {
	next interface{}
}

func NewHandlerWrap(h interface{}) http.Handler {
	return &HandlerWrap{
		next: h,
	}
}

func (h *HandlerWrap) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch handler := h.next.(type) {
	case http.HandlerFunc:
		handler.ServeHTTP(w, r)
	case http.Handler:
		handler.ServeHTTP(w, r)
	case func(http.ResponseWriter, *http.Request):
		handler(w, r)
	}
}

func WrapHandler(h http.Handler) HandlerFunc {
	return func(c Context) {
		h.ServeHTTP(c.Writer(), c.Request())
	}
}
