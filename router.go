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
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"
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

type pathHandler struct {
	paths   []string
	handler []HandlerFuncChain
}

type Router struct {
	UseRawPath             bool
	UnescapePathValues     bool
	RemoveExtraSlash       bool
	RedirectFixedPath      bool
	HandleMethodNotAllowed bool
	group                  string
	logger                 ILogger
	use                    []HandlerFunc
	routes                 MapRouterInformations
	pool                   *sync.Pool
	usePool                *sync.Pool
	trees                  map[string]nodelocation
	pathHandlers           map[string]pathHandler
	allNotFound            HandlerFuncChain
	allNoMethod            HandlerFuncChain
	notFound               HandlerFuncChain
	noMethod               HandlerFuncChain
	maxParams              uint16
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
	router := newRouter(builder.logger)
	return router
}

func newRouter(logger ILogger) *Router {
	router := &Router{
		logger:                 logger,
		group:                  "/",
		routes:                 MapRouterInformations{},
		UseRawPath:             false,
		RemoveExtraSlash:       false,
		UnescapePathValues:     true,
		RedirectFixedPath:      false,
		HandleMethodNotAllowed: true,
		trees:                  map[string]nodelocation{},
		pathHandlers:           map[string]pathHandler{},
	}
	router.pool = &sync.Pool{
		New: func() interface{} {
			return &_context{
				router: router,
				skip:   false,
				index:  -1,
				mu:     sync.Mutex{},
				logger: router.logger,
				params: &Params{},
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
	return router
}

var match, _ = regexp.Compile("(?P<match>= ?)")
var prefixMatch, _ = regexp.Compile("(?P<match>~ ?)")

func (r *Router) path(relativePath string) string {
	matchFlg := match.MatchString(relativePath)
	prefixMatchFlg := prefixMatch.MatchString(relativePath)
	relativePath = match.ReplaceAllString(relativePath, "")
	relativePath = prefixMatch.ReplaceAllString(relativePath, "")
	if relativePath == "" {
		return r.group
	}
	p := path.Join(r.group, relativePath)
	if lastChar(relativePath) == '/' && lastChar(p) != '/' {
		return p + "/"
	}
	if matchFlg {
		p = "= " + p
	}
	if prefixMatchFlg {
		p = "~ " + p
	}
	return p
}

func (r *Router) Handler(method, path string, h ...HandlerFunc) {
	path = r.path(path)
	h = r.mergeHandlers(h)
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
	ph, ok := r.pathHandlers[method]
	if !ok {
		ph = pathHandler{
			paths:   []string{},
			handler: []HandlerFuncChain{},
		}
	}
	ph.paths = append(ph.paths, path)
	ph.handler = append(ph.handler, h)
	r.pathHandlers[method] = ph
	locations := locationRegex(ph.paths)
	r.trees[method] = locations
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

func (r *Router) Use(middleware ...HandlerFunc) {
	r.use = append(r.use, middleware...)
	r.rebuild404Handlers()
	r.rebuild405Handlers()
}

func (r *Router) Group(path string, middleware ...HandlerFunc) *Router {
	u := r.mergeHandlers(middleware)
	router := newRouter(r.logger)
	router.group = r.path(path)
	router.use = u
	router.routes = r.routes
	router.pool = r.pool
	router.trees = r.trees
	router.pathHandlers = r.pathHandlers
	router.maxParams = r.maxParams
	return router
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

func (r *Router) rebuild404Handlers() {
	r.allNotFound = r.mergeHandlers(r.notFound)
}

func (r *Router) rebuild405Handlers() {
	r.allNoMethod = r.mergeHandlers(r.noMethod)
}

func (r *Router) NotFound(h ...HandlerFunc) *Router {
	r.notFound = r.mergeHandlers(h)
	r.rebuild404Handlers()
	return r
}

func (r *Router) NoMethod(h ...HandlerFunc) *Router {
	r.noMethod = h
	r.rebuild405Handlers()
	return r
}

func (r *Router) mergeHandlers(handlers HandlerFuncChain) HandlerFuncChain {
	length := len(r.use) + len(handlers)
	h := make(HandlerFuncChain, length)
	copy(h, r.use)
	copy(h[len(r.use):], handlers)
	return h
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

func (r *Router) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	c := r.pool.Get().(*_context)
	cpHandler := r.usePool.Get().(HandlerFuncChain)
	c.reset(w, req)
	c.logger = r.logger
	c.handler = cpHandler

	r.handleHTTPRequest(c)

	r.pool.Put(c)
	r.usePool.Put(cpHandler)
}

func (r *Router) ServeFiles(paths string, fs http.FileSystem) {
	if strings.Contains(paths, ":") || strings.Contains(paths, "*") {
		panic("URL parameters can not be used when serving a static folder")
	}
	handler := r.createStaticHandler(paths, fs)
	urlPattern := path.Join(paths, "/*filepath")

	// Register GET and HEAD handlers
	r.GET(urlPattern, handler)
	r.HEAD(urlPattern, handler)
}

func (r *Router) handleHTTPRequest(c *_context) {
	httpMethod := c.req.Method
	rPath := c.req.URL.Path
	if r.UseRawPath && len(c.req.URL.RawPath) > 0 {
		rPath = c.req.URL.RawPath
	}

	if r.RemoveExtraSlash {
		rPath = cleanPath(rPath)
	}

	// Find root of the tree for the given HTTP method
	var node *paramNode
	t, ok := r.trees[httpMethod]
	if ok {
		node = matchURL(t, rPath)
	}
	if node != nil {
		ph := r.pathHandlers[httpMethod]
		c.fullPath = ph.paths[node.index]
		*c.params = *node.params
		c.handler = ph.handler[node.index]
		c.Next()
		c.w.WriteHeaderNow()
		return
	}
	if r.HandleMethodNotAllowed {
		for method, node := range r.trees {
			if method == httpMethod {
				continue
			}
			if value := matchURL(node, rPath); value != nil {
				c.handler = r.allNoMethod
				serveError(c, http.StatusMethodNotAllowed, "method not allowed")
				return
			}
		}
	}
	c.handler = r.allNotFound
	serveError(c, http.StatusNotFound, "page not found")
}

func (r *Router) createStaticHandler(relativePath string, fs http.FileSystem) HandlerFunc {
	absolutePath := r.path(relativePath)
	fileServer := http.StripPrefix(absolutePath, http.FileServer(fs))

	return func(c Context) {
		file := c.Param("filepath")
		f, err := fs.Open(file)
		if err != nil {
			c.Writer().WriteHeader(http.StatusNotFound)
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		f.Close()
		fileServer.ServeHTTP(c.Writer(), c.Request())
	}
}

func serveError(c Context, code int, message string) {
	// c.SetStatus(code)
	c.Next()
	if c.Writer().Status() == http.StatusOK {
		c.SetStatus(code)
	}
	if c.Writer().Written() {
		return
	}
	if c.Writer().Status() == code {
		htmlText := `
<!DOCTYPE html>
<html lang="ja">
<head>
  <meta charset="UTF-8">
  <meta http-equiv="X-UA-Compatible" content="IE=edge">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{{.Message}}</title>
</head>
<body>
  <p>{{.Message}}</p>
</body>
</html>
		`

		c.TemplateText(code, htmlText, map[string]string{
			"Message": http.StatusText(code),
		})
		return
	}
	c.Writer().WriteHeaderNow()
}
