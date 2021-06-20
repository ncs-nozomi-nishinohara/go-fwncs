package fwncs

import (
	"context"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/n-creativesystem/go-fwncs/constant"
)

var target = "target"

type ProxyTarget struct {
	Name string
	URL  *url.URL
	Meta map[string]interface{}
}

type ProxyBalancer interface {
	Add(target interface{}) bool
	Remove(name string) bool
	Next(c Context) *ProxyTarget
}
type commonBalancer struct {
	target []*ProxyTarget
	mu     sync.RWMutex
}

func (b *commonBalancer) Add(p interface{}) bool {
	var proxy *ProxyTarget
	if v, ok := p.(*ProxyTarget); !ok {
		return false
	} else {
		proxy = v
	}
	for _, t := range b.target {
		if t.Name == proxy.Name {
			return false
		}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.target = append(b.target, proxy)
	return true
}

func (b *commonBalancer) Remove(name string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	target := b.target
	for i, t := range target {
		if t.Name == name {
			b.target = append(b.target[:i], b.target[i+1:]...)
			return true
		}
	}
	return false
}

type randomBalancer struct {
	*commonBalancer
	random *rand.Rand
}

func (b *randomBalancer) Next(c Context) *ProxyTarget {
	if b.random == nil {
		b.random = rand.New(rand.NewSource(time.Now().UnixNano()))
	}
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.target[b.random.Intn(len(b.target))]
}

func NewRandomBalancer(proxies []*ProxyTarget) ProxyBalancer {
	b := &randomBalancer{
		commonBalancer: new(commonBalancer),
		random:         rand.New(rand.NewSource(time.Now().UnixNano())),
	}
	b.target = proxies
	return b
}

type roundRobinBalancer struct {
	*commonBalancer
	i uint32
}

func (b *roundRobinBalancer) Next(c Context) *ProxyTarget {
	b.mu.RLock()
	defer b.mu.RUnlock()
	b.i = b.i % uint32(len(b.target))
	t := b.target[b.i]
	atomic.AddUint32(&b.i, 1)
	return t
}

func NewRoundRobinBalancer(proxies []*ProxyTarget) ProxyBalancer {
	b := &roundRobinBalancer{
		commonBalancer: new(commonBalancer),
	}
	b.target = proxies
	return b
}

type WeightProxyTarget struct {
	Weight float64
	*ProxyTarget
}
type staticRoundRobinBalancer struct {
	choices Choices
	mu      sync.RWMutex
}

func (b *staticRoundRobinBalancer) Next(c Context) *ProxyTarget {
	b.mu.RLock()
	defer b.mu.RUnlock()
	choice := b.choices.GetOne()
	return choice.Item.(*WeightProxyTarget).ProxyTarget
}

func (b *staticRoundRobinBalancer) Add(p interface{}) bool {
	var proxy *WeightProxyTarget
	if v, ok := p.(*WeightProxyTarget); !ok {
		return false
	} else {
		proxy = v
	}
	for _, choice := range b.choices {
		t := choice.Item.(*WeightProxyTarget)
		if t.Name == proxy.Name {
			return false
		}
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.choices = append(b.choices, Choice{
		Weight: proxy.Weight,
		Item:   proxy,
	})
	return true

}

func (b *staticRoundRobinBalancer) Remove(name string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	for i, choice := range b.choices {
		t := choice.Item.(*WeightProxyTarget)
		if t.Name == name {
			b.choices = append(b.choices[:i], b.choices[i+1:]...)
			return true
		}
	}
	return false
}

func NewStaticWeightedRoundRobinBalancer(proxies []*WeightProxyTarget) ProxyBalancer {
	b := &staticRoundRobinBalancer{}
	b.choices = make([]Choice, len(proxies))
	for idx, proxy := range proxies {
		b.choices[idx] = Choice{
			Weight: proxy.Weight,
			Item:   proxy,
		}
	}
	return b
}

type ProxyConfig struct {
	LoadBalancer   ProxyBalancer
	Rewrite        map[string]string
	RegexRewrite   map[*regexp.Regexp]string
	Transport      http.RoundTripper
	ModifyResponse func(*http.Response) error
	ContextKey     string
}

func Proxy(balancer ProxyBalancer) HandlerFunc {
	config := ProxyConfig{
		LoadBalancer: balancer,
	}
	return ProxyWithConfig(config)
}

func ProxyWithConfig(config ProxyConfig) HandlerFunc {
	if config.LoadBalancer == nil {
		panic("required load balancer")
	}
	if config.ContextKey == "" {
		config.ContextKey = target
	}
	if config.Rewrite != nil {
		if config.RegexRewrite == nil {
			config.RegexRewrite = make(map[*regexp.Regexp]string)
		}
		for k, v := range rewriteRulesRegex(config.Rewrite) {
			config.RegexRewrite[k] = v
		}
	}
	return func(c Context) {
		req := c.Request()
		t := config.LoadBalancer.Next(c)
		c.Set(config.ContextKey, t)
		if err := rewriteURL(config.RegexRewrite, req); err != nil {
			c.Error(err)
			c.AbortWithStatusAndErrorMessage(http.StatusInternalServerError, err)
			return
		}
		if req.Header.Get(constant.HeaderXRealIP) == "" {
			req.Header.Set(constant.HeaderXRealIP, c.ClientIP())
		}
		if req.Header.Get(constant.HeaderXForwardedProto) == "" {
			req.Header.Set(constant.HeaderXForwardedProto, c.Scheme())
		}
		if c.IsWebSocket() && req.Header.Get(constant.HeaderXForwardedFor) == "" { // For HTTP, it is automatically set by Go HTTP reverse proxy.
			req.Header.Set(constant.HeaderXForwardedFor, c.ClientIP())
		}
		switch {
		case c.IsWebSocket():
			proxyRaw(t, c).ServeHTTP(c.Writer(), req)
		case req.Header.Get(constant.HeaderAccept) == "text/event-stream":
		default:
			proxyHTTP(t, c, config).ServeHTTP(c.Writer(), req)
		}
		if err, ok := c.Get("_error").(error); ok {
			if v, ok := err.(*DefaultResponseBody); ok {
				c.AbortWithStatusAndErrorMessage(v.Code, v)
			}
		}
		c.Skip()
	}
}

func proxyRaw(t *ProxyTarget, c Context) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		in, _, err := c.Writer().Hijack()
		if err != nil {
			c.Set("_error", fmt.Sprintf("proxy raw, hijack error=%v, url=%s", t.URL, err))
			return
		}
		defer in.Close()

		out, err := net.Dial("tcp", t.URL.Host)
		if err != nil {
			c.Set("_error", NewDefaultResponseBody(http.StatusBadGateway, fmt.Sprintf("proxy raw, dial error=%v, url=%s", t.URL, err)))
			return
		}
		defer out.Close()

		// Write header
		err = r.Write(out)
		if err != nil {
			c.Set("_error", NewDefaultResponseBody(http.StatusBadGateway, fmt.Sprintf("proxy raw, request header copy error=%v, url=%s", t.URL, err)))
			return
		}

		errCh := make(chan error, 2)
		cp := func(dst io.Writer, src io.Reader) {
			_, err = io.Copy(dst, src)
			errCh <- err
		}

		go cp(out, in)
		go cp(in, out)
		err = <-errCh
		if err != nil && err != io.EOF {
			c.Set("_error", fmt.Errorf("proxy raw, copy body error=%v, url=%s", t.URL, err))
		}
	})
}

const StatusCodeContextCanceled = 499

func proxyHTTP(t *ProxyTarget, c Context, config ProxyConfig) http.Handler {
	proxy := httputil.NewSingleHostReverseProxy(t.URL)
	proxy.ErrorHandler = func(resp http.ResponseWriter, req *http.Request, err error) {
		desc := t.URL.String()
		if t.Name != "" {
			desc = fmt.Sprintf("%s(%s)", t.Name, t.URL.String())
		}
		if err == context.Canceled || strings.Contains(err.Error(), "operation was canceled") {
			httpError := NewDefaultResponseBody(StatusCodeContextCanceled, fmt.Sprintf("client closed connection: %v", err))
			httpError.Internal = err
			c.Set("_error", httpError)
		} else {
			httpError := NewDefaultResponseBody(http.StatusBadGateway, fmt.Sprintf("remote %s unreachable, could not forward: %v", desc, err))
			httpError.Internal = err
			c.Set("_error", httpError)
		}
	}
	proxy.Transport = config.Transport
	proxy.ModifyResponse = config.ModifyResponse
	return proxy
}
