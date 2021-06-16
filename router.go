package fwncs

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path"
	"sync"
	"syscall"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type HandlerFunc func(Context)

type HandlerFuncChain []HandlerFunc

func (hc HandlerFuncChain) Last() HandlerFunc {
	if length := len(hc); length > 0 {
		return hc[length-1]
	}
	return nil
}

func (hc HandlerFuncChain) LastIndex() int {
	if length := len(hc); length > 0 {
		return length - 1
	}
	return 0
}

type HandlerMiddleware func(next http.Handler) http.Handler

type TraceHandler func(method, path string, h httprouter.Handle) (string, string, httprouter.Handle)

var DefaultTracing TraceHandler = func(method, path string, h httprouter.Handle) (string, string, httprouter.Handle) {
	return method, path, func(w http.ResponseWriter, r *http.Request, p httprouter.Params) {
		h(w, r, p)
	}
}

type routerInfo struct {
	method      string
	path        string
	handlerName string
	handler     HandlerFunc
}

type Router struct {
	Router  *httprouter.Router
	group   string
	logger  ILogger
	use     []HandlerFunc
	routes  map[string][]routerInfo
	pool    *sync.Pool
	tracing TraceHandler
}

// var _ IRouter = &Router{}

func Default(opts ...Options) *Router {
	opts = append([]Options{LoggerOptions(DefaultLogger), UsePrometheus()}, opts...)
	router := New(opts...)
	router.Use(Logger(), Recovery(), RequestID())
	return router
}

func New(opts ...Options) *Router {
	builder := &Builder{}
	for _, opt := range opts {
		opt.Apply(builder)
	}
	if builder.logger == nil {
		builder.logger = DefaultLogger
	}
	r := httprouter.New()
	r.NotFound = NotFound()
	r.MethodNotAllowed = MethodNotAllowed()
	router := &Router{
		Router: r,
		logger: builder.logger,
		group:  "/",
		routes: map[string][]routerInfo{},
	}
	router.pool = &sync.Pool{
		New: func() interface{} {
			return &_context{
				router: router,
				abort:  false,
				index:  -1,
				mu:     sync.Mutex{},
			}
		},
	}
	if len(builder.elastic) > 0 {
		router.tracing = Elastic(router, builder.elastic...)
	}
	if builder.newrelic != nil {
		router.tracing = Newrelic(router, builder.newrelic)
	}
	if builder.opentracingTracer != nil {
		router.tracing = JaegerMiddleware(router, builder.opentracingTracer, builder.opentracingOptions...)
	}
	if router.tracing == nil {
		router.tracing = DefaultTracing
	}
	if builder.tracePrometheus {
		router.Use(InstrumentHandlerInFlight, InstrumentHandlerDuration, InstrumentHandlerCounter, InstrumentHandlerResponseSize)
		router.GET("/metrics", WrapHandler(promhttp.Handler()))
	}

	return router
}

func (r *Router) path(relativePath string) string {
	if relativePath == "" {
		return r.group
	}
	return path.Join(r.group, relativePath)
}

func (r *Router) handle(h ...HandlerFunc) httprouter.Handle {
	originalHandler := func(w http.ResponseWriter, req *http.Request, p httprouter.Params) {
		c := r.pool.Get().(*_context)
		c.reset(w, req)
		c.logger = r.logger
		c.params = p
		middlewareLength := len(r.use)
		handlerLength := len(h)
		length := middlewareLength + handlerLength
		c.handler = make(HandlerFuncChain, length)
		for i := 0; i < length; i++ {
			if i < middlewareLength {
				c.handler[i] = r.use[i]
			} else {
				c.handler[i] = h[i-middlewareLength]
			}
		}
		c.Next()
		r.pool.Put(c)
	}
	return originalHandler
}

func (r *Router) Handler(method, path string, h ...HandlerFunc) {
	info := r.routes[method]
	if info == nil {
		info = []routerInfo{}
	}
	lastHandler := HandlerFuncChain(h).Last()
	info = append(info, routerInfo{
		method:      method,
		path:        path,
		handlerName: NameOfFunction(lastHandler),
		handler:     lastHandler,
	})
	r.routes[method] = info
	handle := r.handle(h...)
	r.Router.Handle(r.tracing(method, r.path(path), handle))
}

func (r *Router) GET(path string, h ...HandlerFunc) {
	r.Handler(http.MethodGet, path, h...)
}

func (r *Router) POST(path string, h ...HandlerFunc) {
	r.Handler(http.MethodPost, path, h...)
}

func (r *Router) PUT(path string, h ...HandlerFunc) {
	r.Handler(http.MethodPut, path, h...)
}

func (r *Router) DELETE(path string, h ...HandlerFunc) {
	r.Handler(http.MethodDelete, path, h...)
}

func (r *Router) PATCH(path string, h ...HandlerFunc) {
	r.Handler(http.MethodPatch, path, h...)
}

func (r *Router) HEAD(path string, h ...HandlerFunc) {
	r.Handler(http.MethodHead, path, h...)
}

func (r *Router) OPTIONS(path string, h ...HandlerFunc) {
	r.Handler(http.MethodOptions, path, h...)
}

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	r.Router.ServeHTTP(w, req)
}

func (r *Router) ServeFiles(path string, root http.FileSystem) {
	r.Router.ServeFiles(path, root)
}

func (r *Router) Use(middleware ...HandlerFunc) {
	r.use = append(r.use, middleware...)
}

func (r *Router) Group(path string) *Router {
	u := make([]HandlerFunc, len(r.use))
	for idx, use := range r.use {
		u[idx] = use
	}
	return &Router{
		Router:  r.Router,
		group:   r.path(path),
		logger:  r.logger,
		use:     u,
		routes:  r.routes,
		pool:    r.pool,
		tracing: r.tracing,
	}
}

func (r *Router) Any(path string, h ...HandlerFunc) *Router {
	r.OPTIONS(path, h...)
	r.HEAD(path, h...)
	r.GET(path, h...)
	r.POST(path, h...)
	r.PUT(path, h...)
	r.PATCH(path, h...)
	r.DELETE(path, h...)
	return r
}

func (r *Router) Run(port int) error {
	l, err := getListen(port)
	if err != nil {
		return err
	}
	srv := &http.Server{
		Handler: r,
	}
	go func() {
		if err := srv.Serve(l); err != nil && err != http.ErrServerClosed {
			r.logger.Error(err)
		}
	}()
	return r.run(srv)
}

// RunTLS is https
func (r *Router) RunTLS(port int, certFile, keyFile string) error {
	l, err := getListen(port)
	if err != nil {
		return err
	}
	if certFile == "" {
		return errors.New("certFile is empty")
	}
	srv := &http.Server{
		Handler: r,
	}
	go func() {
		if err := srv.ServeTLS(l, certFile, keyFile); err != nil && err != http.ErrServerClosed {
			r.logger.Error(err)
		}
	}()
	return r.run(srv)
}

// RunUnix is unix domain socket
// 	When the file is empty, the default name is www.sock
func (r *Router) RunUnix(file string) error {
	if file == "" {
		file = "www.sock"
	}
	l, err := net.Listen("unix", file)
	if err != nil {
		return err
	}
	defer l.Close()
	defer os.Remove(file)
	srv := &http.Server{
		Handler: r,
	}
	go func() {
		if err := srv.Serve(l); err != nil && err != http.ErrServerClosed {
			r.logger.Error(err)
		}
	}()
	return r.run(srv)
}

func (r *Router) run(s *http.Server) error {
	signals := []os.Signal{
		syscall.SIGINT,
		syscall.SIGQUIT,
		syscall.SIGABRT,
		syscall.SIGKILL,
		syscall.SIGTERM,
		syscall.SIGSTOP,
	}
	osNotify := make(chan os.Signal, 1)
	signal.Notify(osNotify, signals...)
	sig := <-osNotify
	r.logger.Info(fmt.Sprintf("signal: %v", sig))
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
	defer cancel()
	return s.Shutdown(ctx)
}
