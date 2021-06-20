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
)

type RouterInfo struct {
	Method      string
	Path        string
	HandlerName string
}

type MapRouterInformations map[string][]RouterInfo

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

type Router struct {
	Router  *httprouter.Router
	group   string
	logger  ILogger
	use     []HandlerFunc
	routes  MapRouterInformations
	pool    *sync.Pool
	usePool *sync.Pool
}

func Default(opts ...Options) *Router {
	opts = append([]Options{LoggerOptions(DefaultLogger)}, opts...)
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
	if builder.methodNotAllowedHandler == nil {
		builder.methodNotAllowedHandler = methodNotAllowed
	}
	r := httprouter.New()
	r.MethodNotAllowed = builder.methodNotAllowedHandler
	r.GlobalOPTIONS = builder.globalOPTIONS
	router := &Router{
		Router: r,
		logger: builder.logger,
		group:  "/",
		routes: MapRouterInformations{},
	}
	router.pool = &sync.Pool{
		New: func() interface{} {
			return &_context{
				router: router,
				skip:   false,
				index:  -1,
				mu:     sync.Mutex{},
				logger: router.logger,
			}
		},
	}
	router.usePool = &sync.Pool{
		New: func() interface{} {
			cpHandler := make(HandlerFuncChain, len(router.use)+1)
			copy(cpHandler, router.use)
			cpHandler[len(cpHandler)-1] = func(c Context) {
				c.JSON(http.StatusNotFound, NewDefaultResponseBody(http.StatusNotFound, http.StatusText(http.StatusNotFound)))
			}
			return cpHandler
		},
	}
	router.Router.NotFound = router.httpHandle()
	return router
}

func httpRouterHandle(r *Router, method, path string, h ...HandlerFunc) httprouter.Handle {
	length := len(r.use) + len(h)
	cpHandler := make(HandlerFuncChain, length)
	copy(cpHandler, append(r.use, h...))
	return func(w http.ResponseWriter, req *http.Request, p httprouter.Params) {
		c := r.pool.Get().(*_context)
		c.reset(w, req)
		c.logger = r.logger
		c.params = p
		c.handler = cpHandler
		c.path = path
		c.method = method
		c.Next()
		r.pool.Put(c)
	}
}

func (r *Router) path(relativePath string) string {
	if relativePath == "" {
		return r.group
	}
	return path.Join(r.group, relativePath)
}

func (r *Router) httpHandle() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		cpHandler := r.usePool.Get().(HandlerFuncChain)
		c := r.pool.Get().(*_context)
		c.reset(w, req)
		c.logger = r.logger
		c.handler = cpHandler
		c.Next()
		r.pool.Put(c)
		r.usePool.Put(cpHandler)
	})
}

func (r *Router) httpRouterHandle(method, path string, h ...HandlerFunc) httprouter.Handle {
	return httpRouterHandle(r, method, path, h...)
}

func (r *Router) Handler(method, path string, h ...HandlerFunc) {
	info := r.routes[method]
	if info == nil {
		info = []RouterInfo{}
	}
	lastHandler := HandlerFuncChain(h).Last()
	info = append(info, RouterInfo{
		Method:      method,
		Path:        path,
		HandlerName: NameOfFunction(lastHandler),
	})
	r.routes[method] = info
	handle := r.httpRouterHandle(method, path, h...)
	r.Router.Handle(method, r.path(path), handle)
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

func (r *Router) Group(path string, middleware ...HandlerFunc) *Router {
	u := make([]HandlerFunc, len(r.use))
	copy(u, append(r.use, middleware...))
	return &Router{
		Router: r.Router,
		group:  r.path(path),
		logger: r.logger,
		use:    u,
		routes: r.routes,
		pool:   r.pool,
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
