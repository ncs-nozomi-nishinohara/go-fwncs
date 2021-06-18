package fwncs_test

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/n-creativesystem/go-fwncs"
	"github.com/n-creativesystem/go-fwncs/tests"
	"github.com/stretchr/testify/assert"
)

func TestProxy(t *testing.T) {
	// Setup
	t1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "target 1")
	}))
	defer t1.Close()
	url1, _ := url.Parse(t1.URL)
	t2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "target 2")
	}))
	defer t2.Close()
	url2, _ := url.Parse(t2.URL)

	targets := []*fwncs.ProxyTarget{
		{
			Name: "target 1",
			URL:  url1,
		},
		{
			Name: "target 2",
			URL:  url2,
		},
	}
	wTargets := []*fwncs.WeightProxyTarget{
		{
			Weight: 1,
			ProxyTarget: &fwncs.ProxyTarget{
				Name: "target 1",
				URL:  url1,
			},
		},
		{
			Weight: 1,
			ProxyTarget: &fwncs.ProxyTarget{
				Name: "target 2",
				URL:  url2,
			},
		},
	}

	tt := tests.TestFrames{
		{
			Name: "Rondom Balancer",
			Fn: func(t *testing.T) {
				rb := fwncs.NewRandomBalancer(nil)
				for _, target := range targets {
					assert.True(t, rb.Add(target))
				}
				for _, target := range targets {
					assert.False(t, rb.Add(target))
				}
				for _, target := range wTargets {
					assert.False(t, rb.Add(target))
				}
				router := fwncs.New()
				router.Use(fwncs.Proxy(rb))
				req := httptest.NewRequest(http.MethodGet, "/", nil)
				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, req)
				body := rec.Body.String()
				expected := map[string]bool{
					"target 1": true,
					"target 2": true,
				}
				assert.Condition(t, func() bool {
					return expected[body]
				})

				for _, target := range targets {
					assert.True(t, rb.Remove(target.Name))
				}
				assert.False(t, rb.Remove("unknown target"))
			},
		},
		{
			Name: "Roundrobin Balancer",
			Fn: func(t *testing.T) {
				rrb := fwncs.NewRoundRobinBalancer(targets)
				router := fwncs.New()
				router.Use(fwncs.Proxy(rrb))

				req := httptest.NewRequest(http.MethodGet, "/", nil)
				rec := httptest.NewRecorder()
				router.ServeHTTP(rec, req)
				body := rec.Body.String()
				assert.Equal(t, "target 1", body)

				rec = httptest.NewRecorder()
				router.ServeHTTP(rec, req)
				body = rec.Body.String()
				assert.Equal(t, "target 2", body)

				// ModifyResponse
				router = fwncs.New()
				router.Use(fwncs.ProxyWithConfig(fwncs.ProxyConfig{
					LoadBalancer: rrb,
					ModifyResponse: func(res *http.Response) error {
						res.Body = ioutil.NopCloser(bytes.NewBuffer([]byte("modified")))
						res.Header.Set("X-Modified", "1")
						return nil
					},
				}))

				rec = httptest.NewRecorder()
				router.ServeHTTP(rec, req)
				assert.Equal(t, "modified", rec.Body.String())
				assert.Equal(t, "1", rec.Header().Get("X-Modified"))

			},
		},
	}

	tt.Run(t)
}

func TestStaticWeightingLoadBalancer(t *testing.T) {
	targets := []*fwncs.ProxyTarget{
		{
			Name: "target 1",
			URL:  nil,
		},
		{
			Name: "target 2",
			URL:  nil,
		},
	}

	t1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "target 1")
	}))
	defer t1.Close()
	url1, _ := url.Parse(t1.URL)

	t2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "target 2")
	}))
	defer t2.Close()
	url2, _ := url.Parse(t2.URL)
	wTargets := []*fwncs.WeightProxyTarget{
		{
			Weight: 1,
			ProxyTarget: &fwncs.ProxyTarget{
				Name: "target 1",
				URL:  url1,
			},
		},
		{
			Weight: 1,
			ProxyTarget: &fwncs.ProxyTarget{
				Name: "target 2",
				URL:  url2,
			},
		},
	}
	swrb := fwncs.NewStaticWeightedRoundRobinBalancer(nil)
	for _, target := range wTargets {
		assert.True(t, swrb.Add(target))
	}
	// type error
	for _, target := range targets {
		assert.False(t, swrb.Add(target))
	}
	router := fwncs.New()
	router.Use(fwncs.Proxy(swrb))
	expected := map[string]bool{
		"target 1": true,
		"target 2": true,
	}
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		body := rec.Body.String()
		assert.Condition(t, func() bool {
			return expected[body]
		})
	}
}

func TestStaticWeightingLoadBalancer2(t *testing.T) {
	t1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "target 1")
	}))
	defer t1.Close()
	url1, _ := url.Parse(t1.URL)

	t2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "target 2")
	}))
	defer t2.Close()
	url2, _ := url.Parse(t2.URL)
	wTargets := []*fwncs.WeightProxyTarget{
		{
			Weight: 1,
			ProxyTarget: &fwncs.ProxyTarget{
				Name: "target 1",
				URL:  url1,
			},
		},
		{
			Weight: 0,
			ProxyTarget: &fwncs.ProxyTarget{
				Name: "target 2",
				URL:  url2,
			},
		},
	}

	swrb := fwncs.NewStaticWeightedRoundRobinBalancer(nil)
	for _, target := range wTargets {
		assert.True(t, swrb.Add(target))
	}
	router := fwncs.New()
	router.Use(fwncs.Proxy(swrb))
	expected := map[string]bool{
		"target 1": true,
		"target 2": true,
	}
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		body := rec.Body.String()
		assert.Condition(t, func() bool {
			return expected[body]
		})
	}
}

func TestStaticWeightingLoadBalancer3(t *testing.T) {
	t1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "target 1")
	}))
	defer t1.Close()
	url1, _ := url.Parse(t1.URL)

	t2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "target 2")
	}))
	defer t2.Close()
	url2, _ := url.Parse(t2.URL)

	t3 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "target 3")
	}))
	defer t3.Close()
	url3, _ := url.Parse(t3.URL)

	wTargets := []*fwncs.WeightProxyTarget{
		{
			Weight: 0.7,
			ProxyTarget: &fwncs.ProxyTarget{
				Name: "target 1",
				URL:  url1,
			},
		},
		{
			Weight: 0.2,
			ProxyTarget: &fwncs.ProxyTarget{
				Name: "target 2",
				URL:  url2,
			},
		},
		{
			Weight: 0.1,
			ProxyTarget: &fwncs.ProxyTarget{
				Name: "target 3",
				URL:  url3,
			},
		},
	}

	swrb := fwncs.NewStaticWeightedRoundRobinBalancer(nil)
	for _, target := range wTargets {
		assert.True(t, swrb.Add(target))
	}
	router := fwncs.New()
	router.Use(fwncs.Proxy(swrb))
	expected := map[string]bool{
		"target 1": true,
		"target 2": true,
		"target 3": true,
	}
	for i := 0; i < 10; i++ {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rec := httptest.NewRecorder()
		router.ServeHTTP(rec, req)
		body := rec.Body.String()
		assert.Condition(t, func() bool {
			return expected[body]
		})
	}
}
