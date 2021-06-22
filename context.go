package fwncs

import (
	"context"
	"encoding/json"
	"html/template"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/n-creativesystem/go-fwncs/constant"
	"github.com/n-creativesystem/go-fwncs/render"
)

type Context interface {
	/*
		ResponceWriter
	*/

	Writer() ResponseWriter
	SetWriter(w ResponseWriter)
	GetStatus() int
	SetStatus(status int)
	ResponseSize() int
	SetHeader(key, value string)

	/*
		Request
	*/
	Request() *http.Request
	SetRequest(r *http.Request)
	Header() http.Header
	GetContext() context.Context
	SetContext(ctx context.Context)
	IsWebSocket() bool
	Scheme() string

	/*
		Abort or error
	*/
	AbortWithStatus(status int)
	AbortWithStatusAndErrorMessage(status int, err error)
	AbortWithStatusAndMessage(status int, v interface{})
	Error(err error)
	GetError() []error
	// Skip is 後続の処理を止める
	IsSkip() bool
	Skip()

	/*
		Query or URL parameter
	*/
	Param(name string) string
	Params() Params
	QueryParam(name string) string
	DefaultQuery(name string, defaultValue string) string

	/*
		Request body
	*/
	ReadJsonBody(v interface{}) error
	FormValue(name string) string
	FormFile(name string) (*multipart.FileHeader, error)
	MultiPartForm() (*multipart.Form, error)

	/*
		Cookie
	*/
	Cookie(name string) (*http.Cookie, error)
	Cookies() []*http.Cookie

	/*
		Response body
	*/
	Render(status int, r render.Render)
	String(status int, format string, v ...interface{})
	JSON(status int, v interface{})
	JSONP(status int, v interface{})
	IndentJSON(status int, v interface{}, indent string)
	AsciiJSON(status int, v interface{})
	YAML(status int, v interface{})
	Template(status int, v interface{}, filenames ...string)

	/*
		Middlewere or handler
	*/
	Next()
	HandlerName() string
	Logger() ILogger
	ClientIP() string
	Set(key string, value interface{})
	Get(key string) interface{}
	Redirect(status int, url string)

	/*
		Utils
	*/
	// HttpClient when the tr is nil, the default transport is http.DefaultTransport
	HttpClient(tr http.RoundTripper) *http.Client
	Path() string
	RealPath() string
	Method() string
	RealMethod() string
}

type _context struct {
	router   *Router
	w        ResponseWriter
	req      *http.Request
	params   *Params
	logger   ILogger
	skip     bool
	handler  HandlerFuncChain
	index    int
	mp       map[string]interface{}
	errs     []error
	mu       sync.Mutex
	query    url.Values
	path     string
	method   string
	_Params  Params
	fullPath string
}

var _ Context = &_context{}

func (c *_context) reset(w http.ResponseWriter, r *http.Request) {
	c.w = wrapResponseWriter(w, c.logger)
	c.req = r
	c._Params = c._Params[:0]
	*c.params = (*c.params)[:0]
	c.skip = false
	c.handler = nil
	c.index = -1
	c.mp = map[string]interface{}{}
	c.errs = c.errs[:0]
	c.mu = sync.Mutex{}
	c.query = r.URL.Query()
	c.path = ""
	c.method = ""
}

func (c *_context) Writer() ResponseWriter {
	return c.w
}

func (c *_context) SetWriter(w ResponseWriter) {
	c.w = w
}

func (c *_context) GetStatus() int {
	return c.w.Status()
}

func (c *_context) SetStatus(status int) {
	c.w.WriteHeader(status)
}

func (c *_context) ResponseSize() int {
	return c.w.Size()
}

func (c *_context) SetHeader(key, value string) {
	c.Writer().Header().Set(key, value)
}

func (c *_context) Request() *http.Request {
	return c.req
}

func (c *_context) SetRequest(r *http.Request) {
	*c.req = *r
}

func (c *_context) Header() http.Header {
	return c.req.Header
}

func (c *_context) GetContext() context.Context {
	return c.Request().Context()
}

func (c *_context) SetContext(ctx context.Context) {
	*c.req = *c.req.WithContext(ctx)
}

func (c *_context) IsWebSocket() bool {
	upgrade := c.Header().Get(constant.HeaderUpgrade)
	return strings.EqualFold(upgrade, "websocket")
}

func (c *_context) Scheme() string {
	if c.req.TLS != nil {
		return "https"
	}
	if scheme := c.req.Header.Get(constant.HeaderXForwardedProto); scheme != "" {
		return scheme
	}
	if scheme := c.req.Header.Get(constant.HeaderXForwardedProtocol); scheme != "" {
		return scheme
	}
	if ssl := c.req.Header.Get(constant.HeaderXForwardedSsl); ssl == "on" {
		return "https"
	}
	if scheme := c.req.Header.Get(constant.HeaderXUrlScheme); scheme != "" {
		return scheme
	}
	return "http"
}

func (c *_context) AbortWithStatusAndMessage(status int, v interface{}) {
	if !c.IsSkip() {
		c.Skip()
		c.JSON(status, v)
	}
}

func (c *_context) AbortWithStatusAndErrorMessage(status int, err error) {
	if !c.IsSkip() {
		c.Skip()
		type errorBody struct {
			Status   int    `json:"status"`
			Message  string `json:"message"`
			Describe string `json:"message_describe"`
		}
		e := &errorBody{
			Status:   status,
			Message:  "error",
			Describe: err.Error(),
		}
		c.JSON(status, e)
	}
}

func (c *_context) Error(err error) {
	c.errs = append(c.errs, err)
}

func (c *_context) GetError() []error {
	return c.errs
}

func (c *_context) IsSkip() bool {
	return c.skip
}

func (c *_context) Skip() {
	c.skip = true
}

func (c *_context) Param(name string) string {
	return c.params.ByName(name)
}

func (c *_context) Params() Params {
	return *c.params
}

func (c *_context) Logger() ILogger {
	return c.logger
}

func (c *_context) ClientIP() string {
	clientIP := c.Header().Get(constant.HeaderXForwardedFor)
	clientIP = strings.TrimSpace(strings.Split(clientIP, ",")[0])
	if clientIP == "" {
		clientIP = strings.TrimSpace(c.Header().Get(constant.HeaderXRealIP))
	}
	if clientIP != "" {
		return clientIP
	}

	if ip, _, err := net.SplitHostPort(strings.TrimSpace(c.req.RemoteAddr)); err == nil {
		return ip
	}
	return ""
}

func (c *_context) AbortWithStatus(status int) {
	if !c.IsSkip() {
		c.Skip()
		c.w.WriteHeader(status)
	}
}

func (c *_context) Next() {
	c.index++
	for c.index < len(c.handler) {
		if !c.skip {
			handler := c.handler[c.index]
			handler(c)
			c.index++
		} else {
			break
		}
	}
}

func (c *_context) Set(key string, value interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.mp == nil {
		c.mp = make(map[string]interface{})
	}
	c.mp[key] = value
}

func (c *_context) Get(key string) interface{} {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.mp[key]
}

func (c *_context) ReadJsonBody(v interface{}) error {
	return json.NewDecoder(c.req.Body).Decode(v)
}

func (c *_context) Redirect(status int, url string) {
	c.Render(-1, render.Redirect{Status: status, Location: url, Request: c.req})
}

func (c *_context) HandlerName() string {
	return NameOfFunction(c.handler.Last())
}

func (c *_context) QueryParam(name string) string {
	if c.query == nil {
		c.query = c.req.URL.Query()
	}
	return c.query.Get(name)
}

func (c *_context) DefaultQuery(name string, defaultValue string) string {
	if v := c.QueryParam(name); v != "" {
		return v
	}
	return defaultValue
}

func (c *_context) FormValue(name string) string {
	return c.req.FormValue(name)
}

func (c *_context) Cookie(name string) (*http.Cookie, error) {
	return c.req.Cookie(name)
}

func (c *_context) Cookies() []*http.Cookie {
	return c.req.Cookies()
}

func (c *_context) FormFile(name string) (*multipart.FileHeader, error) {
	f, fh, err := c.req.FormFile(name)
	if err != nil {
		return nil, err
	}
	f.Close()
	return fh, nil
}

func (c *_context) MultiPartForm() (*multipart.Form, error) {
	err := c.req.ParseMultipartForm(defaultMemory)
	return c.req.MultipartForm, err
}

// HttpClient when the tr is nil, the default transport is http.DefaultTransport
func (c *_context) HttpClient(tr http.RoundTripper) *http.Client {
	client := http.DefaultClient
	client.Transport = RequestIDTransport(c.GetContext(), tr)
	return client
}

func (c *_context) Render(status int, r render.Render) {
	w := c.Writer()
	c.SetStatus(status)
	if !bodyAllowedForStatus(status) {
		r.WriteContentType(w)
		w.WriteHeaderNow()
		return
	}
	if err := r.Render(w); err != nil {
		panic(err)
	}
}

func (c *_context) JSON(status int, v interface{}) {
	c.Render(status, render.JSON{Data: v})
}

func (c *_context) JSONP(status int, v interface{}) {
	callback := c.DefaultQuery("callback", "")
	if callback == "" {
		c.JSON(status, v)
	} else {
		c.Render(status, render.JSONP{Data: v, Callback: callback})
	}
}

func (c *_context) IndentJSON(status int, v interface{}, indent string) {
	c.Render(status, render.IndentJSON{Data: v, Indent: indent})
}

func (c *_context) AsciiJSON(status int, v interface{}) {
	c.Render(status, render.AsciiJSON{Data: v})
}

func (c *_context) String(status int, format string, v ...interface{}) {
	c.Render(status, render.Text{Format: format, Data: v})
}

func (c *_context) YAML(status int, v interface{}) {
	c.Render(status, render.YAML{Data: v})
}

func (c *_context) Template(status int, v interface{}, filenames ...string) {
	c.Render(status, render.TemplateRender{
		Template: template.Must(template.New("html").ParseFiles(filenames...)),
		Data:     v,
	})
}

func (c *_context) Path() string {
	return c.path
}

func (c *_context) RealPath() string {
	return c.req.URL.RawPath
}

func (c *_context) Method() string {
	return c.method
}

func (c *_context) RealMethod() string {
	return c.req.Method
}
