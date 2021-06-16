package fwncs

import (
	"context"
	"net/http"

	"github.com/n-creativesystem/go-fwncs/constant"
)

type requestIDTransport struct {
	tr http.RoundTripper
	c  context.Context
}

func (r *requestIDTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	req.Header.Set(constant.HeaderXRequestID, FromRequestID(r.c))
	return r.tr.RoundTrip(req)
}

func RequestIDTransport(c context.Context, tr http.RoundTripper) http.RoundTripper {
	if tr == nil {
		tr = http.DefaultTransport
	}
	return &requestIDTransport{
		tr: tr,
		c:  c,
	}
}
