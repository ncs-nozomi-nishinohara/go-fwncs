package fwncs

import (
	"net/http"
	"os"
	"path"

	"github.com/julienschmidt/httprouter"
	"github.com/newrelic/go-agent/v3/integrations/nrhttprouter"
	"github.com/newrelic/go-agent/v3/newrelic"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.elastic.co/apm/module/apmhttprouter"
)

type HandlerFunc func(Context)

type HandlerMiddleware func(next http.Handler) http.Handler

type IRouter interface {
	GET(path string, h ...HandlerFunc)
	POST(path string, h ...HandlerFunc)
	PUT(path string, h ...HandlerFunc)
	DELETE(path string, h ...HandlerFunc)
	PATCH(path string, h ...HandlerFunc)
	HEAD(path string, h ...HandlerFunc)
	OPTIONS(path string, h ...HandlerFunc)
	ServeHTTP(w http.ResponseWriter, req *http.Request)
	Use(middleware ...HandlerFunc)
	// UseHandler(middleware ...MiddlewareFunc)
	Group(path string) IRouter
	ServeFiles(path string, root http.FileSystem)
	Any(path string, h ...HandlerFunc) IRouter
}

type router struct {
	Router IHandler
	group  string
	logger ILogger
	use    []HandlerFunc
	// useHandler middlewareStack
}

var _ IRouter = &router{}

func Default() IRouter {
	logger := NewLogger(os.Stderr, Info, FormatShort, FormatDatetime)
	router := New(logger, PrometheusOptions())
	router.Use(Logger(), Recovery())
	return router
}

func New(logger ILogger, opts ...Options) IRouter {
	builder := Builder{}
	for _, opt := range opts {
		opt.Apply(builder)
	}
	var route IHandler
	switch GetApmName() {
	case Unknown:
		r := httprouter.New()
		r.NotFound = NotFound()
		r.MethodNotAllowed = MethodNotAllowed()
		route = r
	case Elastic:
		o, _ := builder["elastic"].([]apmhttprouter.Option)
		r := apmhttprouter.New(o...)
		r.NotFound = NotFound()
		r.MethodNotAllowed = MethodNotAllowed()
		route = r
	case Newrelic:
		app, ok := builder["newrelic"].(*newrelic.Application)
		if !ok {
			app = defaultNewrelicApp()
		}
		r := nrhttprouter.New(app)
		r.NotFound = NotFound()
		r.MethodNotAllowed = MethodNotAllowed()
		route = r
	}
	router := &router{
		Router: route,
		logger: logger,
		group:  "/",
	}
	if _, ok := builder[_prometheus]; ok {
		router.Use(InstrumentHandlerInFlight, InstrumentHandlerDuration, InstrumentHandlerCounter, InstrumentHandlerResponseSize)
		router.GET("/metrics", WrapHandler(promhttp.Handler()))
	}
	return router
}

func (r *router) path(relativePath string) string {
	if relativePath == "" {
		return r.group
	}
	return path.Join(r.group, relativePath)
}

func (r *router) handle(h ...HandlerFunc) httprouter.Handle {
	originalHandler := func(rw http.ResponseWriter, req *http.Request, p httprouter.Params) {
		c := newContext(rw, req)
		c.logger = r.logger
		c.params = p
		c.handler = append(c.handler, r.use...)
		c.handler = append(c.handler, h...)
		c.Next()
	}
	return originalHandler
}

func (r *router) GET(path string, h ...HandlerFunc) {
	r.Router.Handle(http.MethodGet, r.path(path), r.handle(h...))
}

func (r *router) POST(path string, h ...HandlerFunc) {
	r.Router.Handle(http.MethodPost, r.path(path), r.handle(h...))
}

func (r *router) PUT(path string, h ...HandlerFunc) {
	r.Router.Handle(http.MethodPut, r.path(path), r.handle(h...))
}

func (r *router) DELETE(path string, h ...HandlerFunc) {
	r.Router.Handle(http.MethodDelete, r.path(path), r.handle(h...))
}

func (r *router) PATCH(path string, h ...HandlerFunc) {
	r.Router.Handle(http.MethodPatch, r.path(path), r.handle(h...))
}

func (r *router) HEAD(path string, h ...HandlerFunc) {
	r.Router.Handle(http.MethodHead, r.path(path), r.handle(h...))
}

func (r *router) OPTIONS(path string, h ...HandlerFunc) {
	r.Router.Handle(http.MethodOptions, r.path(path), r.handle(h...))
}

func (r *router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.Router.ServeHTTP(w, req)
}

func (r *router) ServeFiles(path string, root http.FileSystem) {
	r.Router.ServeFiles(path, root)
}

func (r *router) Use(middleware ...HandlerFunc) {
	r.use = append(r.use, middleware...)
}

func (r *router) Group(path string) IRouter {
	u := make([]HandlerFunc, len(r.use))
	for idx, use := range r.use {
		u[idx] = use
	}
	return &router{
		Router: r.Router,
		group:  r.path(path),
		logger: r.logger,
		use:    u,
	}
}

func (r *router) Any(path string, h ...HandlerFunc) IRouter {
	r.OPTIONS(path, h...)
	r.HEAD(path, h...)
	r.GET(path, h...)
	r.POST(path, h...)
	r.PUT(path, h...)
	r.PATCH(path, h...)
	r.DELETE(path, h...)
	return r
}
